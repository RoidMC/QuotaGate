package authz_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/util"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/glebarez/sqlite"
	"github.com/roidmc/quotagate/internal/authz"
	"github.com/roidmc/quotagate/internal/model"
	"gorm.io/gorm"
)

func setupBenchDB(b *testing.B) *gorm.DB {
	b.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		b.Fatalf("failed to open bench db: %v", err)
	}
	return db
}

func BenchmarkAuthz_GlobalEnforcer(b *testing.B) {
	db := setupBenchDB(b)
	mgr, err := authz.NewAuthzManager(db, false)
	if err != nil {
		b.Fatalf("NewAuthzManager failed: %v", err)
	}
	if err := mgr.InitDefaultPolicies(); err != nil {
		b.Fatalf("InitDefaultPolicies failed: %v", err)
	}
	if err := mgr.InitGroupingRelations(
		context.Background(),
		authz.DefaultSystemRoles(),
		[]model.UserRoleAssignment{
			{UserID: "alice", Role: "admin", TenantID: "tenant-a"},
			{UserID: "bob", Role: "user", TenantID: "tenant-a"},
		},
	); err != nil {
		b.Fatalf("InitGroupingRelations failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, err := mgr.EnforceRBAC(context.Background(), "tenant-a", "alice", []string{"admin"}, "GET", "/api/users", "*")
		if err != nil {
			b.Fatalf("EnforceRBAC error: %v", err)
		}
	}
}

// BenchmarkAuthz_TempEnforcerPerRequest simulates the old per-request temporary
// Enforcer approach: each iteration creates a fresh Enforcer, loads p policies,
// injects inheritance and user-role g rules, and then enforces once.
func BenchmarkAuthz_TempEnforcerPerRequest(b *testing.B) {
	db := setupBenchDB(b)

	adapter, err := gormadapter.NewAdapterByDBUseTableName(db, "", "casbin_rule")
	if err != nil {
		b.Fatalf("failed to create adapter: %v", err)
	}
	m, err := casbinmodel.NewModelFromString(authz.RBACWithDomainsModel)
	if err != nil {
		b.Fatalf("failed to create model: %v", err)
	}

	// Seed the adapter with default p policies once.
	seedMgr, err := authz.NewAuthzManager(db, false)
	if err != nil {
		b.Fatalf("failed to create seed manager: %v", err)
	}
	if err := seedMgr.InitDefaultPolicies(); err != nil {
		b.Fatalf("failed to seed default policies: %v", err)
	}
	// Intentionally not closing seedMgr; closing would shut down the shared
	// in-memory SQLite connection used by the per-request benchmark.

	registry := authz.NewRoleRegistry()
	if err := registry.Load(context.Background(), authz.DefaultSystemRoles()); err != nil {
		b.Fatalf("failed to load registry: %v", err)
	}
	inheritance := registry.GetInheritanceRules("*")
	assignments := [][]string{
		{"alice", "admin", "tenant-a"},
		{"bob", "user", "tenant-a"},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		temp, err := casbin.NewSyncedEnforcer(m, adapter)
		if err != nil {
			b.Fatalf("NewSyncedEnforcer error: %v", err)
		}
		if err := temp.LoadPolicy(); err != nil {
			b.Fatalf("LoadPolicy error: %v", err)
		}
		rm := temp.GetRoleManager()
		rm.AddDomainMatchingFunc("keyMatch", util.KeyMatch)

		for _, rule := range inheritance {
			if _, err := temp.AddGroupingPolicy(rule); err != nil {
				b.Fatalf("AddGroupingPolicy error: %v", err)
			}
		}
		for _, rule := range assignments {
			if _, err := temp.AddGroupingPolicy(rule); err != nil {
				b.Fatalf("AddGroupingPolicy error: %v", err)
			}
		}

		_, err = temp.Enforce("tenant-a", "alice", "GET", "/api/users", "*")
		if err != nil {
			b.Fatalf("Enforce error: %v", err)
		}
	}
}

// BenchmarkAuthz_InitGroupingRelations_10k measures the cold-start cost of
// loading 30,000 user-role g relations into the global Enforcer. This is the
// dominant startup cost when scaling to a large multi-tenant deployment.
func BenchmarkAuthz_InitGroupingRelations_10k(b *testing.B) {
	const (
		tenantCount  = 100
		userCount    = 10000
		rolesPerUser = 3
	)

	roleNames := []string{"admin", "editor", "viewer"}

	defs := authz.DefaultSystemRoles()
	for t := 0; t < tenantCount; t++ {
		for _, name := range roleNames {
			defs = append(defs, model.RoleDefinition{
				Name:     name,
				TenantID: fmt.Sprintf("tenant-%d", t),
			})
		}
	}

	assignments := make([]model.UserRoleAssignment, 0, userCount*rolesPerUser)
	for u := 0; u < userCount; u++ {
		tenantID := fmt.Sprintf("tenant-%d", u%tenantCount)
		userID := fmt.Sprintf("user-%d", u)
		for i := 0; i < rolesPerUser; i++ {
			assignments = append(assignments, model.UserRoleAssignment{
				UserID:   userID,
				Role:     roleNames[i],
				TenantID: tenantID,
			})
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		db := setupBenchDB(b)
		mgr, err := authz.NewAuthzManager(db, false)
		if err != nil {
			b.Fatalf("NewAuthzManager failed: %v", err)
		}
		if err := mgr.InitDefaultPolicies(); err != nil {
			b.Fatalf("InitDefaultPolicies failed: %v", err)
		}
		if err := mgr.InitGroupingRelations(context.Background(), defs, assignments); err != nil {
			b.Fatalf("InitGroupingRelations failed: %v", err)
		}
	}
}

// BenchmarkAuthz_10kUsers stresses the global Enforcer with a large multi-tenant
// workload: 100 tenants, 10,000 users, 3 roles per user (30,000 g relations).
// It verifies that Enforce latency stays bounded as the grouping-rule count
// grows into the tens of thousands.
func BenchmarkAuthz_10kUsers(b *testing.B) {
	db := setupBenchDB(b)
	mgr, err := authz.NewAuthzManager(db, false)
	if err != nil {
		b.Fatalf("NewAuthzManager failed: %v", err)
	}
	if err := mgr.InitDefaultPolicies(); err != nil {
		b.Fatalf("InitDefaultPolicies failed: %v", err)
	}

	const (
		tenantCount  = 100
		userCount    = 10000
		rolesPerUser = 3
	)

	roleNames := []string{"admin", "editor", "viewer"}

	// System roles plus one set of tenant-scoped roles per tenant.
	defs := authz.DefaultSystemRoles()
	for t := 0; t < tenantCount; t++ {
		for _, name := range roleNames {
			defs = append(defs, model.RoleDefinition{
				Name:     name,
				TenantID: fmt.Sprintf("tenant-%d", t),
			})
		}
	}

	// Distribute users evenly across tenants; each user gets all three roles.
	assignments := make([]model.UserRoleAssignment, 0, userCount*rolesPerUser)
	for u := 0; u < userCount; u++ {
		tenantID := fmt.Sprintf("tenant-%d", u%tenantCount)
		userID := fmt.Sprintf("user-%d", u)
		for i := 0; i < rolesPerUser; i++ {
			assignments = append(assignments, model.UserRoleAssignment{
				UserID:   userID,
				Role:     roleNames[i],
				TenantID: tenantID,
			})
		}
	}

	if err := mgr.InitGroupingRelations(context.Background(), defs, assignments); err != nil {
		b.Fatalf("InitGroupingRelations failed: %v", err)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		tenantID := fmt.Sprintf("tenant-%d", i%tenantCount)
		userID := fmt.Sprintf("user-%d", i%userCount)
		_, err := mgr.EnforceRBAC(context.Background(), tenantID, userID, nil, "GET", "/api/users", "*")
		if err != nil {
			b.Fatalf("EnforceRBAC error: %v", err)
		}
	}
}
