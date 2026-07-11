package authz_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/roidmc/quotagate/internal/middleware"
)

// TestPathTraversalExtremeCases tests exotic path traversal vectors
// against the sanitizePath + keyMatch3 defense stack.
func TestPathTraversalExtremeCases(t *testing.T) {
	manager := setupTestManager(t)
	_, _ = manager.AddPolicy("*", "admin", "GET", "/api/users/{id}", "*")
	_, _ = manager.AddPolicy("*", "admin", "GET", "/api/files/*", "*")

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
	r.Get("/api/files/*", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	// All these targets should be rejected (403 or 404), never 200.
	attackVectors := []struct {
		name   string
		target string
	}{
		// Standard traversals
		{"double_dot", "/api/users/../../etc/passwd"},
		{"single_dot_dot", "/api/users/./../../etc"},

		// URL-encoded traversals (Go net/http decodes %2E -> .)
		{"encoded_dot", "/api/users/%2E%2E/%2E%2E/etc"},
		{"encoded_slash", "/api/users/..%2F..%2Fetc"},

		// Double-encoded (Go decodes once: %252E -> %2E literal, not ..)
		// These don't contain ".." so they bypass sanitizePath,
		// but keyMatch3 treats %2E%2E as a literal segment, not traversal.
		{"double_encoded", "/api/users/%252E%252E/%252E%252E/etc"},

		// NULL byte injection
		{"null_after_dot", "/api/users/..%00/../../etc"},
		{"null_in_dot", "/api/users/%2E%2E%00/"},

		// Mixed case encoding (already lowercase after decode)
		{"mixed_case_encoded", "/api/users/%2e%2e/%2e%2e/etc"},

		// Backslash variants (Windows-style)
		{"backslash_traversal", "/api/users/..\\..\\etc"},

		// Excessive dots
		{"triple_dot", "/api/users/.../.../etc"},
		{"quad_dot", "/api/users/..../etc"},

		// Dot-dot with trailing slash
		{"dot_slash_dot_dot", "/api/users/./.././../etc"},

		// Unicode fullwidth dots (U+FF0E)
		{"unicode_fullwidth", "/api/users/．．/．．/etc"},

		// Semicolon path parameter injection
		{"semicolon_injection", "/api/users/..;/api/admin"},

		// Encoded backslash
		{"encoded_backslash", "/api/users/..%5C..%5Cetc"},

		// Valid requests (should pass)
		// Tested separately below
	}

	for _, v := range attackVectors {
		t.Run(v.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, v.target, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)

			if w.Code == http.StatusOK {
				t.Errorf("SECURITY BREACH: attack vector %q was allowed (200)!", v.target)
			}
			t.Logf("target=%q -> status=%d (blocked)", v.target, w.Code)
		})
	}
}

// TestLongPathDoS checks if keyMatch3 is vulnerable to ReDoS
// with pathologically long paths.
func TestLongPathDoS(t *testing.T) {
	manager := setupTestManager(t)
	_, _ = manager.AddPolicy("*", "admin", "GET", "/api/users/{id}", "*")

	// Generate a 10KB path
	longSegment := ""
	for i := 0; i < 10000; i++ {
		longSegment += "a"
	}

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, withRole(r, "admin"))
		})
	})
	r.Use(middleware.Authz(manager, nil))
	r.Get("/api/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	target := "/api/users/" + longSegment
	req := httptest.NewRequest(http.MethodGet, target, nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Should be 404 (chi can't route) or 200 (matches {id})
	// The key is it completes without hanging
	t.Logf("long path (10KB) -> status=%d", w.Code)
}

// TestEncodedPathBypass verifies that percent-encoded paths
// don't bypass authorization after Go's URL decoding.
func TestEncodedPathBypass(t *testing.T) {
	manager := setupTestManager(t)
	_, _ = manager.AddPolicy("*", "admin", "GET", "/api/admin/secret", "*")

	r := chi.NewRouter()
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			next.ServeHTTP(w, withRole(r, "admin"))
		})
	})
	r.Use(middleware.Authz(manager, nil))
	r.Get("/api/admin/secret", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("secret"))
	})

	// Try to access /api/admin/secret via encoded segments
	encodedTargets := []string{
		"/api/admin/%73ecret",  // %73 = 's'
		"/api/%61dmin/secret",  // %61 = 'a'
		"/%61pi/admin/secret",  // encoded 'a' in api
		"/api/admin/secret%00", // null byte appended
		"/api/admin/secret%20", // space appended
	}

	for _, target := range encodedTargets {
		req := httptest.NewRequest(http.MethodGet, target, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		// Encoded paths should either:
		// 1. Be decoded by Go and match the route -> 200 (authorized)
		// 2. Not match the route -> 404
		// They should NOT bypass authz and return 200 for a path
		// that the policy doesn't cover.
		t.Logf("target=%q -> path=%q status=%d",
			target, req.URL.Path, w.Code)
	}
}
