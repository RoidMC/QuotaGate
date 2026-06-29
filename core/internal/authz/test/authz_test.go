package authz_test

import (
	"os"
	"testing"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	"github.com/glebarez/sqlite"
	"github.com/roidmc/quotagate/internal/authz"
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

func setupInMemoryEnforcer(t *testing.T, modelStr string) *casbin.Enforcer {
	t.Helper()
	m, err := model.NewModelFromString(modelStr)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}
	e, err := casbin.NewEnforcer(m)
	if err != nil {
		t.Fatalf("failed to create enforcer: %v", err)
	}
	return e
}

func TestNewAuthzManager(t *testing.T) {
	db := setupTestDB(t)

	tests := []struct {
		name    string
		mode    authz.Mode
		wantErr bool
	}{
		{"RBAC mode", authz.ModeRBAC, false},
		{"ABAC mode", authz.ModeABAC, false},
		{"unsupported mode", authz.Mode("unknown"), true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, err := authz.NewAuthzManager(db, tt.mode)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if m == nil {
				t.Fatal("expected non-nil manager")
			}
		})
	}
}

func TestAuthzManager_EnforceRBAC(t *testing.T) {
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db, authz.ModeRBAC)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	_, _ = m.AddPolicy("admin", "/api/v1/users", "GET")
	_, _ = m.AddRoleForUser("alice", "admin")

	tests := []struct {
		name     string
		user     string
		path     string
		method   string
		expected bool
	}{
		{"admin user allowed", "alice", "/api/v1/users", "GET", true},
		{"direct role allowed", "admin", "/api/v1/users", "GET", true},
		{"wrong method denied", "alice", "/api/v1/users", "POST", false},
		{"wrong path denied", "alice", "/api/v1/other", "GET", false},
		{"unknown user denied", "bob", "/api/v1/users", "GET", false},
		{"anonymous denied", "anonymous", "/api/v1/users", "GET", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := m.EnforceRBAC(tt.user, tt.path, tt.method)
			if err != nil {
				t.Fatalf("EnforceRBAC error: %v", err)
			}
			if got != tt.expected {
				t.Errorf("EnforceRBAC(%q, %q, %q) = %v, want %v", tt.user, tt.path, tt.method, got, tt.expected)
			}
		})
	}
}

func TestAuthzManager_RoleManagement(t *testing.T) {
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db, authz.ModeRBAC)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	t.Run("add and get roles", func(t *testing.T) {
		_, err := m.AddRoleForUser("alice", "admin")
		if err != nil {
			t.Fatalf("AddRoleForUser error: %v", err)
		}

		roles, err := m.GetRolesForUser("alice")
		if err != nil {
			t.Fatalf("GetRolesForUser error: %v", err)
		}
		if len(roles) != 1 || roles[0] != "admin" {
			t.Errorf("expected roles [admin], got %v", roles)
		}
	})

	t.Run("get users for role", func(t *testing.T) {
		users, err := m.GetUsersForRole("admin")
		if err != nil {
			t.Fatalf("GetUsersForRole error: %v", err)
		}
		found := false
		for _, u := range users {
			if u == "alice" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected alice in admin users, got %v", users)
		}
	})

	t.Run("delete role", func(t *testing.T) {
		_, err := m.DeleteRoleForUser("alice", "admin")
		if err != nil {
			t.Fatalf("DeleteRoleForUser error: %v", err)
		}

		roles, err := m.GetRolesForUser("alice")
		if err != nil {
			t.Fatalf("GetRolesForUser error: %v", err)
		}
		if len(roles) != 0 {
			t.Errorf("expected no roles, got %v", roles)
		}
	})
}

func TestAuthzManager_PolicyManagement(t *testing.T) {
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db, authz.ModeRBAC)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	t.Run("add and check policy", func(t *testing.T) {
		added, err := m.AddPolicy("editor", "/api/v1/posts", "GET")
		if err != nil {
			t.Fatalf("AddPolicy error: %v", err)
		}
		if !added {
			t.Error("expected policy to be added")
		}

		has, err := m.HasPolicy("editor", "/api/v1/posts", "GET")
		if err != nil {
			t.Fatalf("HasPolicy error: %v", err)
		}
		if !has {
			t.Error("expected policy to exist")
		}
	})

	t.Run("remove policy", func(t *testing.T) {
		removed, err := m.RemovePolicy("editor", "/api/v1/posts", "GET")
		if err != nil {
			t.Fatalf("RemovePolicy error: %v", err)
		}
		if !removed {
			t.Error("expected policy to be removed")
		}

		has, err := m.HasPolicy("editor", "/api/v1/posts", "GET")
		if err != nil {
			t.Fatalf("HasPolicy error: %v", err)
		}
		if has {
			t.Error("expected policy to not exist")
		}
	})

	t.Run("get policy", func(t *testing.T) {
		_, _ = m.AddPolicy("viewer", "/api/v1/items", "GET")
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
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db, authz.ModeRBAC)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	err = m.InitDefaultPolicies()
	if err != nil {
		t.Fatalf("InitDefaultPolicies error: %v", err)
	}

	policies, err := m.GetPolicy()
	if err != nil {
		t.Fatalf("GetPolicy error: %v", err)
	}
	if len(policies) == 0 {
		t.Error("expected default policies to be loaded")
	}

	roles, err := m.GetAllRoles()
	if err != nil {
		t.Fatalf("GetAllRoles error: %v", err)
	}
	if len(roles) == 0 {
		t.Error("expected some roles")
	}

	t.Run("idempotent init", func(t *testing.T) {
		err := m.InitDefaultPolicies()
		if err != nil {
			t.Fatalf("second InitDefaultPolicies error: %v", err)
		}
	})
}

func TestAuthzManager_ReloadPolicy(t *testing.T) {
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db, authz.ModeRBAC)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	err = m.ReloadPolicy()
	if err != nil {
		t.Fatalf("ReloadPolicy error: %v", err)
	}
}

func TestAuthzManager_Enforce(t *testing.T) {
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db, authz.ModeRBAC)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	_, _ = m.AddPolicy("admin", "/api/v1/users", "GET")

	got, err := m.Enforce("admin", "/api/v1/users", "GET")
	if err != nil {
		t.Fatalf("Enforce error: %v", err)
	}
	if !got {
		t.Error("expected allowed")
	}
}

func TestAuthzManager_ConcurrentAccess(t *testing.T) {
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db, authz.ModeRBAC)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	_, _ = m.AddPolicy("user", "/api/v1/resource", "GET")

	done := make(chan bool, 100)
	for i := 0; i < 100; i++ {
		go func() {
			_, err := m.EnforceRBAC("user", "/api/v1/resource", "GET")
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

func TestMain(m *testing.M) {
	code := m.Run()
	os.Exit(code)
}

func TestAuthzManager_Close(t *testing.T) {
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db, authz.ModeRBAC)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}

	err = m.Close()
	if err != nil {
		t.Fatalf("Close error: %v", err)
	}
}

func TestAuthzManager_ABAC(t *testing.T) {
	db := setupTestDB(t)
	m, err := authz.NewAuthzManager(db, authz.ModeABAC)
	if err != nil {
		t.Fatalf("failed to create ABAC manager: %v", err)
	}

	// ABAC policies use sub_rule (boolean expression) instead of direct sub match
	// Policy format: p = sub_rule, obj, act
	// Matcher: m = eval(p.sub_rule) && r.obj == p.obj && r.act == p.act

	t.Run("basic role-based ABAC rule", func(t *testing.T) {
		// Policy: if sub == "admin", allow access to /api/admin
		added, err := m.AddPolicy(`r.sub == "admin"`, "/api/admin", "GET")
		if err != nil {
			t.Fatalf("AddPolicy error: %v", err)
		}
		if !added {
			t.Error("expected policy to be added")
		}

		tests := []struct {
			name     string
			sub      string
			obj      string
			act      string
			expected bool
		}{
			{"admin can access admin path", "admin", "/api/admin", "GET", true},
			{"user cannot access admin path", "user", "/api/admin", "GET", false},
			{"admin cannot access other path", "admin", "/api/other", "GET", false},
			{"admin cannot POST to admin path", "admin", "/api/admin", "POST", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := m.Enforce(tt.sub, tt.obj, tt.act)
				if err != nil {
					t.Fatalf("Enforce error: %v", err)
				}
				if got != tt.expected {
					t.Errorf("Enforce(%q, %q, %q) = %v, want %v", tt.sub, tt.obj, tt.act, got, tt.expected)
				}
			})
		}
	})

	t.Run("ABAC with multiple rules", func(t *testing.T) {
		// Policy: if sub == "editor", allow POST to /api/posts
		added, err := m.AddPolicy(`r.sub == "editor"`, "/api/posts", "POST")
		if err != nil {
			t.Fatalf("AddPolicy error: %v", err)
		}
		if !added {
			t.Error("expected policy to be added")
		}

		// Policy: if sub == "viewer", allow GET to /api/posts
		added, err = m.AddPolicy(`r.sub == "viewer"`, "/api/posts", "GET")
		if err != nil {
			t.Fatalf("AddPolicy error: %v", err)
		}

		tests := []struct {
			name     string
			sub      string
			obj      string
			act      string
			expected bool
		}{
			{"editor can POST", "editor", "/api/posts", "POST", true},
			{"editor cannot GET", "editor", "/api/posts", "GET", false},
			{"viewer can GET", "viewer", "/api/posts", "GET", true},
			{"viewer cannot POST", "viewer", "/api/posts", "POST", false},
			{"guest cannot access", "guest", "/api/posts", "GET", false},
		}

		for _, tt := range tests {
			t.Run(tt.name, func(t *testing.T) {
				got, err := m.Enforce(tt.sub, tt.obj, tt.act)
				if err != nil {
					t.Fatalf("Enforce error: %v", err)
				}
				if got != tt.expected {
					t.Errorf("Enforce(%q, %q, %q) = %v, want %v", tt.sub, tt.obj, tt.act, got, tt.expected)
				}
			})
		}
	})

	t.Run("ABAC policy management", func(t *testing.T) {
		added, err := m.AddPolicy(`r.sub == "test_editor"`, "/api/test_posts", "POST")
		if err != nil {
			t.Fatalf("AddPolicy error: %v", err)
		}
		if !added {
			t.Error("expected policy to be added")
		}

		has, err := m.HasPolicy(`r.sub == "test_editor"`, "/api/test_posts", "POST")
		if err != nil {
			t.Fatalf("HasPolicy error: %v", err)
		}
		if !has {
			t.Error("expected policy to exist")
		}

		removed, err := m.RemovePolicy(`r.sub == "test_editor"`, "/api/test_posts", "POST")
		if err != nil {
			t.Fatalf("RemovePolicy error: %v", err)
		}
		if !removed {
			t.Error("expected policy to be removed")
		}

		has, err = m.HasPolicy(`r.sub == "test_editor"`, "/api/test_posts", "POST")
		if err != nil {
			t.Fatalf("HasPolicy error: %v", err)
		}
		if has {
			t.Error("expected policy to not exist after removal")
		}
	})
}

func TestAuthzManager_ABAC_Vs_RBAC(t *testing.T) {
	t.Run("RBAC supports role inheritance", func(t *testing.T) {
		db := setupTestDB(t)
		m, err := authz.NewAuthzManager(db, authz.ModeRBAC)
		if err != nil {
			t.Fatalf("failed to create RBAC manager: %v", err)
		}

		_, _ = m.AddPolicy("member", "/api/content", "READ")
		_, _ = m.AddRoleForUser("alice", "member")

		allowed, err := m.Enforce("alice", "/api/content", "READ")
		if err != nil {
			t.Fatalf("Enforce error: %v", err)
		}
		if !allowed {
			t.Error("RBAC: alice (member) should be able to READ via role inheritance")
		}
	})

	t.Run("ABAC uses expression evaluation without inheritance", func(t *testing.T) {
		db := setupTestDB(t)
		m, err := authz.NewAuthzManager(db, authz.ModeABAC)
		if err != nil {
			t.Fatalf("failed to create ABAC manager: %v", err)
		}

		_, _ = m.AddPolicy(`r.sub == "vip"`, "/api/premium", "READ")

		allowed, err := m.Enforce("vip", "/api/premium", "READ")
		if err != nil {
			t.Fatalf("Enforce error: %v", err)
		}
		if !allowed {
			t.Error("ABAC: vip user should be able to READ premium content")
		}

		allowed, err = m.Enforce("regular", "/api/premium", "READ")
		if err != nil {
			t.Fatalf("Enforce error: %v", err)
		}
		if allowed {
			t.Error("ABAC: regular user should NOT be able to READ premium content")
		}

		_, _ = m.AddRoleForUser("alice", "vip")
		allowed, err = m.Enforce("alice", "/api/premium", "READ")
		if err != nil {
			t.Fatalf("Enforce error: %v", err)
		}
		if allowed {
			t.Error("ABAC: AddRoleForUser should NOT affect ABAC (no inheritance)")
		}
	})
}
