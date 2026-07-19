package authz_test

import (
	"context"
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/roidmc/quotagate/internal/authz"
	"github.com/roidmc/quotagate/internal/model"
	"gorm.io/gorm"
)

func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

func setupTestManager(t *testing.T) *authz.AuthzManager {
	t.Helper()
	m := setupTestManagerWithABAC(t)
	if err := m.InitRoleRegistry(context.Background(), authz.DefaultSystemRoles()); err != nil {
		t.Fatalf("failed to init role registry: %v", err)
	}
	// Prime the enforcer with sample user-role assignments so that request
	// subName = userID can resolve to a role via g rules.
	if _, err := m.AssignUserRole("alice", "admin", ""); err != nil {
		t.Fatalf("failed to assign alice admin: %v", err)
	}
	if _, err := m.AssignUserRole("bob", "user", ""); err != nil {
		t.Fatalf("failed to assign bob user: %v", err)
	}
	return m
}

func setupTestManagerWithABAC(t *testing.T) *authz.AuthzManager {
	t.Helper()
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	if err := m.InitDefaultPolicies(); err != nil {
		t.Fatalf("failed to init default policies: %v", err)
	}
	return m
}

func TestNewAuthzManager(t *testing.T) {
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if m == nil {
		t.Fatal("expected non-nil manager")
	}
}

func TestAuthzManager_EnforceRBAC(t *testing.T) {
	m := setupTestManager(t)

	// Default system policy: admin can GET /api/users.
	tests := []struct {
		name     string
		user     string
		roles    []string
		path     string
		method   string
		expected bool
	}{
		{"admin user allowed", "alice", []string{"admin"}, "/api/users", "GET", true},
		{"direct admin role allowed", "admin", []string{"admin"}, "/api/users", "GET", true},
		{"wrong method denied", "alice", []string{"admin"}, "/api/users/{id}", "POST", false},
		{"wrong path denied", "alice", []string{"admin"}, "/api/v1/other", "GET", false},
		{"unknown user denied", "bob", []string{"unknown"}, "/api/users", "GET", false},
		{"anonymous denied on admin route", "anonymous", []string{"anonymous"}, "/api/users", "GET", false},
		{"user self-service allowed", "bob", []string{"user"}, "/api/my-account", "GET", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.EnforceRBAC(context.Background(), "", tt.user, tt.roles, tt.method, tt.path, "*")
			if err != nil {
				t.Fatalf("EnforceRBAC error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("EnforceRBAC(%q, %q, %q) = %v, want %v", tt.user, tt.path, tt.method, got, tt.expected)
			}
		})
	}
}

func TestAuthzManager_RoleInheritance(t *testing.T) {
	m := setupTestManager(t)

	// admin inherits user via DefaultSystemRoles, so admin can access user routes.
	allowed, err := m.EnforceRBAC(context.Background(), "", "alice", []string{"admin"}, "GET", "/api/my-account", "*")
	if err != nil {
		t.Fatalf("EnforceRBAC error: %v", err)
	}
	if !allowed {
		t.Error("admin should inherit user and access /api/my-account")
	}
}

func TestRoleRegistry_GetInheritanceRules_MergesSystemAndTenantDomains(t *testing.T) {
	registry := authz.NewRoleRegistry()
	defs := []model.RoleDefinition{
		{Name: "user", IsSystem: true, InheritedRoles: nil},
		{Name: "admin", IsSystem: true, InheritedRoles: []string{"user"}},
		{Name: "tenant-admin", TenantID: "tenant-a", InheritedRoles: []string{"user"}},
		{Name: "tenant-manager", TenantID: "tenant-a", InheritedRoles: []string{"tenant-admin"}},
	}
	if err := registry.Load(context.Background(), defs); err != nil {
		t.Fatalf("Load error: %v", err)
	}

	rules := registry.GetInheritanceRules("tenant-a")

	contains := func(child, parent, domain string) bool {
		for _, r := range rules {
			if len(r) == 3 && r[0] == child && r[1] == parent && r[2] == domain {
				return true
			}
		}
		return false
	}

	// System rules are always injected under the unified wildcard domain.
	if !contains("admin", "user", "*") {
		t.Error("expected system rule admin -> user in wildcard domain")
	}

	// Tenant rules are merged with system rules.
	if !contains("tenant-admin", "user", "tenant-a") {
		t.Error("expected tenant rule tenant-admin -> user in tenant-a domain")
	}
	if !contains("tenant-manager", "tenant-admin", "tenant-a") {
		t.Error("expected tenant rule tenant-manager -> tenant-admin in tenant-a domain")
	}

	// Requesting the wildcard system domain should not include tenant rules.
	systemOnly := registry.GetInheritanceRules("*")
	for _, r := range systemOnly {
		if len(r) == 3 && r[2] != "*" {
			t.Errorf("system domain should not contain tenant rules, got %v", r)
		}
	}
}

func TestAuthzManager_PolicyManagement(t *testing.T) {
	m := setupTestManager(t)

	t.Run("add and check policy", func(t *testing.T) {
		added, err := m.AddPolicy("*", "editor", "GET", "/api/v1/posts", "*")
		if err != nil {
			t.Fatalf("AddPolicy error: %v", err)
		}
		if !added {
			t.Error("expected policy to be added")
		}

		has, err := m.HasPolicy("*", "editor", "GET", "/api/v1/posts", "*")
		if err != nil {
			t.Fatalf("HasPolicy error: %v", err)
		}
		if !has {
			t.Error("expected policy to exist")
		}
	})

	t.Run("remove policy", func(t *testing.T) {
		removed, err := m.RemovePolicy("*", "editor", "GET", "/api/v1/posts", "*")
		if err != nil {
			t.Fatalf("RemovePolicy error: %v", err)
		}
		if !removed {
			t.Error("expected policy to be removed")
		}

		has, err := m.HasPolicy("*", "editor", "GET", "/api/v1/posts", "*")
		if err != nil {
			t.Fatalf("HasPolicy error: %v", err)
		}
		if has {
			t.Error("expected policy to not exist")
		}
	})

	t.Run("get policy", func(t *testing.T) {
		policies, err := m.GetPolicy()
		if err != nil {
			t.Fatalf("GetPolicy error: %v", err)
		}
		if len(policies) == 0 {
			t.Error("expected some policies")
		}
	})
}

func TestAuthzManager_InitDefaultPolicies(t *testing.T) {
	m := setupTestManager(t)

	policies, err := m.GetPolicy()
	if err != nil {
		t.Fatalf("GetPolicy error: %v", err)
	}
	if len(policies) == 0 {
		t.Error("expected default policies to be loaded")
	}

	err = m.InitDefaultPolicies()
	if err != nil {
		t.Fatalf("second InitDefaultPolicies error: %v", err)
	}
}

func TestAuthzManager_ReloadPolicy(t *testing.T) {
	m := setupTestManager(t)

	if err := m.ReloadPolicy(); err != nil {
		t.Fatalf("ReloadPolicy error: %v", err)
	}
}

func TestAuthzManager_ConcurrentAccess(t *testing.T) {
	m := setupTestManager(t)

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			_, err := m.EnforceRBAC(context.Background(), "", "alice", []string{"admin"}, "GET", "/api/users", "*")
			if err != nil {
				t.Errorf("concurrent EnforceRBAC error: %v", err)
			}
			done <- true
		}()
	}

	for i := 0; i < 100; i++ {
		<-done
	}
}

func TestAuthzManager_Close(t *testing.T) {
	m := setupTestManager(t)

	if err := m.Close(); err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestAuthzManager_ABAC(t *testing.T) {
	m := setupTestManagerWithABAC(t)

	t.Run("basic role-based ABAC rule", func(t *testing.T) {
		added, err := m.AddABACPolicy(`r.sub.ID == "admin"`, "", "GET", "/api/admin", `r.obj.Owner == ""`)
		if err != nil {
			t.Fatalf("AddABACPolicy error: %v", err)
		}
		if !added {
			t.Error("expected policy to be added")
		}

		sub := authz.Subject{ID: "admin", Owner: "", Roles: []interface{}{"admin"}, Attrs: map[string]any{}}
		obj := authz.Object{Owner: "", Name: "/api/admin", Attrs: map[string]any{}}

		got, err := m.EnforceABAC(context.Background(), sub, "", "GET", "/api/admin", obj)
		if err != nil {
			t.Fatalf("EnforceABAC error: %v", err)
		}
		if !got {
			t.Error("expected admin allowed via ABAC")
		}

		sub.ID = "user"
		sub.Roles = []interface{}{"user"}
		got, err = m.EnforceABAC(context.Background(), sub, "", "GET", "/api/admin", obj)
		if err != nil {
			t.Fatalf("EnforceABAC error: %v", err)
		}
		if got {
			t.Error("expected user denied via ABAC")
		}
	})

	t.Run("ABAC domain isolation", func(t *testing.T) {
		_, _ = m.AddABACPolicy(`r.sub.Owner == "tenant-a"`, "tenant-a", "GET", "/api/resource", `r.obj.Owner == "tenant-a"`)

		sub := authz.Subject{ID: "admin", Owner: "tenant-a", Roles: []interface{}{"admin"}, Attrs: map[string]any{}}
		obj := authz.Object{Owner: "tenant-a", Name: "/api/resource", Attrs: map[string]any{}}

		got, err := m.EnforceABAC(context.Background(), sub, "tenant-a", "GET", "/api/resource", obj)
		if err != nil {
			t.Fatalf("EnforceABAC error: %v", err)
		}
		if !got {
			t.Error("expected tenant-a admin allowed")
		}

		got, err = m.EnforceABAC(context.Background(), sub, "tenant-b", "GET", "/api/resource", obj)
		if err != nil {
			t.Fatalf("EnforceABAC error: %v", err)
		}
		if got {
			t.Error("expected tenant-b admin denied by domain isolation")
		}
	})

	t.Run("ABAC rejects constant truth rule", func(t *testing.T) {
		_, err := m.AddABACPolicy("true", "", "GET", "/api/x", "true")
		if err == nil {
			t.Error("expected error for constant truth sub_rule")
		}
	})

	t.Run("ABAC allows Attrs field", func(t *testing.T) {
		added, err := m.AddABACPolicy(`r.sub.Attrs.level == "senior"`, "", "GET", "/api/attrs", `r.obj.Owner == ""`)
		if err != nil {
			t.Fatalf("AddABACPolicy with Attrs failed: %v", err)
		}
		if !added {
			t.Error("expected policy to be added")
		}

		sub := authz.Subject{ID: "u1", Owner: "", Roles: []interface{}{"user"}, Attrs: map[string]any{"level": "senior"}}
		obj := authz.Object{Owner: "", Name: "/api/attrs", Attrs: map[string]any{}}
		got, err := m.EnforceABAC(context.Background(), sub, "", "GET", "/api/attrs", obj)
		if err != nil {
			t.Fatalf("EnforceABAC error: %v", err)
		}
		if !got {
			t.Error("expected senior user allowed via Attrs rule")
		}
	})
}

func TestAuthzManager_TempEnforcerNoGRulePollution(t *testing.T) {
	m := setupTestManager(t)

	// alice has admin role; bob has no roles.
	aliceAllowed, err := m.EnforceRBAC(context.Background(), "", "alice", []string{"admin"}, "GET", "/api/users", "*")
	if err != nil {
		t.Fatalf("EnforceRBAC error: %v", err)
	}
	if !aliceAllowed {
		t.Error("alice should be allowed")
	}

	bobAllowed, err := m.EnforceRBAC(context.Background(), "", "bob", []string{}, "GET", "/api/users", "*")
	if err != nil {
		t.Fatalf("EnforceRBAC error: %v", err)
	}
	if bobAllowed {
		t.Error("bob should be denied; temporary enforcer must not leak alice's g rules")
	}
}

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}
