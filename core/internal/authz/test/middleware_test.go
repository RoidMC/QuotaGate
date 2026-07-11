package authz_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/go-chi/chi/v5"
	"github.com/roidmc/quotagate/internal/authz"
	"github.com/roidmc/quotagate/internal/middleware"
	"gorm.io/gorm"
)

func setupMiddlewareTestManager(t *testing.T) *authz.AuthzManager {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	m, err := authz.NewAuthzManager(db, false)
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	if err := m.InitDefaultPolicies(); err != nil {
		t.Fatalf("failed to init default policies: %v", err)
	}
	if err := m.InitRoleRegistry(t.Context(), authz.DefaultSystemRoles()); err != nil {
		t.Fatalf("failed to init role registry: %v", err)
	}
	return m
}

func withRole(r *http.Request, role string) *http.Request {
	ctx := middleware.WithUserRole(r.Context(), role)
	// The middleware resolves roles via g(userID, role, domain); set userID
	// equal to the role so that identity-based grouping lets the request pass.
	ctx = middleware.WithUserID(ctx, role)
	return r.WithContext(ctx)
}

func TestMiddleware_Authz_Allowed(t *testing.T) {
	manager := setupMiddlewareTestManager(t)

	allowed, err := manager.EnforceRBAC(t.Context(), "", "anonymous", []string{"admin"}, "GET", "/api/users", "*")
	if err != nil {
		t.Fatalf("direct EnforceRBAC error: %v", err)
	}
	t.Logf("direct EnforceRBAC: %v", allowed)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, withRole(r, "admin"))
		})
	})
	r.Use(middleware.Authz(manager, nil))
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMiddleware_Authz_Forbidden(t *testing.T) {
	manager := setupMiddlewareTestManager(t)

	r := chi.NewRouter()
	r.Use(middleware.Authz(manager, nil))
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// No role set → anonymous → Casbin denies admin route.
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, w.Code, w.Body.String())
	}
}

func TestMiddleware_AuthzWithResource_Allowed(t *testing.T) {
	manager := setupMiddlewareTestManager(t)

	r := chi.NewRouter()
	r.Use(middleware.Authz(manager, nil))
	r.Get("/api/my-account", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/my-account", nil)
	req = withRole(req, "user")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d: %s", http.StatusOK, w.Code, w.Body.String())
	}
}

func TestMiddleware_AuthzWithResource_Forbidden(t *testing.T) {
	manager := setupMiddlewareTestManager(t)

	r := chi.NewRouter()
	r.Use(middleware.Authz(manager, nil))
	r.Get("/api/my-account", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// Has role "anonymous" but no policy covering it for /api/my-account.
	req := httptest.NewRequest(http.MethodGet, "/api/my-account", nil)
	req = withRole(req, "anonymous")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d, got %d: %s", http.StatusForbidden, w.Code, w.Body.String())
	}
}

func TestMiddleware_Authz_NilManager(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic with nil manager")
		}
	}()

	r := chi.NewRouter()
	r.Use(middleware.Authz(nil, nil))
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
}

func TestMiddleware_Authz_PathTraversal(t *testing.T) {
	manager := setupMiddlewareTestManager(t)

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, withRole(r, "admin"))
		})
	})
	r.Use(middleware.Authz(manager, nil))
	r.Get("/api/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	cases := []struct {
		name       string
		target     string
		wantStatus int
	}{
		{"traversal_literal", "/api/users/../../etc/passwd", http.StatusForbidden},
		{"traversal_encoded", "/api/users/..%2F..%2Fetc", http.StatusForbidden},
		{"double_slash", "/api//users//123", http.StatusForbidden},
		{"valid_id", "/api/users/123", http.StatusOK},
		{"trailing_slash", "/api/users/123/", http.StatusForbidden},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tc.target, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code != tc.wantStatus {
				t.Errorf("target=%q: expected %d, got %d", tc.target, tc.wantStatus, w.Code)
			}
		})
	}
}

func TestMiddleware_AuthzWithResource_PathTraversal(t *testing.T) {
	manager := setupMiddlewareTestManager(t)

	r := chi.NewRouter()
	r.Use(middleware.Authz(manager, nil))
	r.Get("/api/my-account", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/my-account/../../secret", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status %d for traversal, got %d", http.StatusForbidden, w.Code)
	}
}
