package authz_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/roidmc/quotagate/internal/middleware"
)

// fakeRoleResolver is a test-only RoleResolver whose behaviour is fully
// controlled by the test. It returns the configured (roles, err) tuple,
// letting us simulate DB success / DB failure / role-mismatch without
// touching a real database.
type fakeRoleResolver struct {
	roles []string
	err   error
	calls int
}

func (f *fakeRoleResolver) EffectiveRoles(ctx context.Context, userID string) ([]string, error) {
	f.calls++
	return f.roles, f.err
}

// withRoleAndUser injects both userID and roles into the request context,
// matching what the BearerAuth middleware would set after parsing a token.
func withRoleAndUser(r *http.Request, userID string, roles []string) *http.Request {
	ctx := middleware.WithUserID(r.Context(), userID)
	ctx = middleware.WithUserRoles(ctx, roles)
	return r.WithContext(ctx)
}

// runAuthzWithResolver wires the Authz middleware with the given resolver
// and failure mode, then dispatches a single GET /api/users request with
// the provided identity. It returns the resulting HTTP status code.
//
// We target /api/users, which the default policy grants to the "admin"
// role, so a request carrying role=admin AND a resolver that returns
// [admin] is the baseline "allowed" case. Deviations from that baseline
// are what the tests below exercise.
func runAuthzWithResolver(t *testing.T, resolver middleware.RoleResolver, mode middleware.DBFailureMode, userID string, tokenRoles []string) int {
	t.Helper()
	manager := setupMiddlewareTestManager(t)

	r := chi.NewRouter()
	r.Use(middleware.Authz(manager, nil,
		middleware.WithRoleResolver(resolver),
		middleware.WithDBFailureMode(mode),
	))
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req = withRoleAndUser(req, userID, tokenRoles)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

// TestAuthz_DBFailure_DefaultModeReturns503 verifies the default mode
// (ModeServiceUnavailable): when the resolver errors, the middleware
// returns 503, NOT 403. This is the headline fix — DB outages no longer
// masquerade as permission denials.
func TestAuthz_DBFailure_DefaultModeReturns503(t *testing.T) {
	resolver := &fakeRoleResolver{
		err: errors.New("simulated DB connection lost"),
	}
	// Token carries admin, which the default policy would allow — but
	// the resolver fails, so the request should NOT reach the handler.
	code := runAuthzWithResolver(t, resolver, middleware.ModeServiceUnavailable, "user-1", []string{"admin"})
	if code != http.StatusServiceUnavailable {
		t.Errorf("expected 503 on DB failure (default mode), got %d", code)
	}
	if resolver.calls != 1 {
		t.Errorf("expected resolver to be called once, got %d", resolver.calls)
	}
}

// TestAuthz_DBFailure_ModeDenyReturns403 covers the legacy mode: operators
// who explicitly want the old semantics can opt back in via WithDBFailureMode(ModeDeny).
// Useful for compliance regimes that require any DB-contact failure to be
// reported as a denial.
func TestAuthz_DBFailure_ModeDenyReturns403(t *testing.T) {
	resolver := &fakeRoleResolver{
		err: errors.New("simulated DB timeout"),
	}
	code := runAuthzWithResolver(t, resolver, middleware.ModeDeny, "user-1", []string{"admin"})
	if code != http.StatusForbidden {
		t.Errorf("expected 403 on DB failure (ModeDeny), got %d", code)
	}
}

// TestAuthz_DBFailure_ModeTrustTokenLetsRequestThrough covers the
// availability-over-security mode: on resolver failure the middleware
// falls through to RBAC using the token's roles. The strict-revocation
// guarantee is suspended for the duration of the outage.
//
// Note on the EnforceRBAC dispatch: EnforceRBAC resolves the user's roles
// via the Casbin enforcer's grouping policy (g(userID, role, domain)), NOT
// via the roles []string argument — the argument is kept only for API
// compatibility. So to make ModeTrustToken meaningful in this test we
// pre-register "user-1" → "admin" in the enforcer; the resolver then fails,
// but RBAC still finds the grouping policy and lets the request through.
//
// In production this grouping policy is populated by AuthzManager.SyncAll
// (or AssignUserRole) from UserRoleAssignment rows; a DB outage that
// prevents SyncAll is precisely the scenario ModeTrustToken is designed
// to tolerate — the in-memory enforcer still carries the last-known
// grouping state.
func TestAuthz_DBFailure_ModeTrustTokenLetsRequestThrough(t *testing.T) {
	manager := setupMiddlewareTestManager(t)
	if _, err := manager.AssignUserRole("user-1", "admin", ""); err != nil {
		t.Fatalf("AssignUserRole: %v", err)
	}

	resolver := &fakeRoleResolver{
		err: errors.New("simulated DB connection lost"),
	}

	r := chi.NewRouter()
	r.Use(middleware.Authz(manager, nil,
		middleware.WithRoleResolver(resolver),
		middleware.WithDBFailureMode(middleware.ModeTrustToken),
	))
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req = withRoleAndUser(req, "user-1", []string{"admin"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on DB failure (ModeTrustToken, admin in token), got %d", w.Code)
	}
}

// TestAuthz_DBFailure_ModeTrustTokenStillAppliesRBAC ensures that
// ModeTrustToken does NOT bypass RBAC — it only bypasses the strict
// token-vs-DB comparison. If the token's roles are insufficient for the
// route, RBAC still denies.
//
// Token carries "user", policy on /api/users requires "admin" → 403,
// even under ModeTrustToken with a failing resolver.
func TestAuthz_DBFailure_ModeTrustTokenStillAppliesRBAC(t *testing.T) {
	resolver := &fakeRoleResolver{
		err: errors.New("simulated DB connection lost"),
	}
	code := runAuthzWithResolver(t, resolver, middleware.ModeTrustToken, "user-1", []string{"user"})
	if code != http.StatusForbidden {
		t.Errorf("expected 403 (RBAC still applies in ModeTrustToken), got %d", code)
	}
}

// TestAuthz_RoleMismatch_Returns403 verifies that a *successful* resolver
// query whose result does not match the token is still treated as a
// definitive 403. This is the case the resolver-failure modes must NOT
// shadow: when the DB is reachable and the roles genuinely differ, there
// is no ambiguity to defer to a mode — the user's token is stale.
func TestAuthz_RoleMismatch_Returns403(t *testing.T) {
	// Token says admin, DB says user → mismatch → 403, regardless of mode.
	// We test under the default mode to confirm mismatch is not treated as
	// a DB failure.
	resolver := &fakeRoleResolver{
		roles: []string{"user"}, // DB claims the user lost admin since the token was issued
	}
	code := runAuthzWithResolver(t, resolver, middleware.ModeServiceUnavailable, "user-1", []string{"admin"})
	if code != http.StatusForbidden {
		t.Errorf("expected 403 on role mismatch (DB reachable), got %d", code)
	}
}

// TestAuthz_RolesMatch_AllowsThrough is the happy path: resolver succeeds
// and the DB roles match the token roles. The request should proceed to RBAC,
// which (with role=admin and the default policy) lets it through.
//
// As with ModeTrustToken, EnforceRBAC resolves roles via the Casbin enforcer's
// grouping policy rather than the roles argument, so we pre-register user-1 →
// admin. The resolver's [admin] return matches the token's [admin], so the
// strict check passes; Casbin then finds the grouping policy and allows.
func TestAuthz_RolesMatch_AllowsThrough(t *testing.T) {
	manager := setupMiddlewareTestManager(t)
	if _, err := manager.AssignUserRole("user-1", "admin", ""); err != nil {
		t.Fatalf("AssignUserRole: %v", err)
	}

	resolver := &fakeRoleResolver{
		roles: []string{"admin"},
	}

	r := chi.NewRouter()
	r.Use(middleware.Authz(manager, nil,
		middleware.WithRoleResolver(resolver),
		middleware.WithDBFailureMode(middleware.ModeServiceUnavailable),
	))
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req = withRoleAndUser(req, "user-1", []string{"admin"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 on role match, got %d", w.Code)
	}
}

// TestAuthz_NoResolver_SkipsCheck confirms that when no RoleResolver is
// wired, the strict-token-validation block is skipped entirely and the
// request proceeds to RBAC using the token's roles. This is the behaviour
// for deployments that do not want strict revocation (e.g. offline-first
// or read-heavy public APIs).
//
// Like the happy-path test above, we pre-register user-1 → admin in the
// enforcer so EnforceRBAC finds the grouping policy. The roles claim in
// the token is [admin], but EnforceRBAC ignores it — what matters is the
// in-enforcer grouping state.
func TestAuthz_NoResolver_SkipsCheck(t *testing.T) {
	manager := setupMiddlewareTestManager(t)
	if _, err := manager.AssignUserRole("user-1", "admin", ""); err != nil {
		t.Fatalf("AssignUserRole: %v", err)
	}

	r := chi.NewRouter()
	r.Use(middleware.Authz(manager, nil)) // no WithRoleResolver
	r.Get("/api/users", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	req = withRoleAndUser(req, "user-1", []string{"admin"})
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected 200 with no resolver wired, got %d", w.Code)
	}
}

// TestAuthz_DBFailureMode_String confirms the mode names show up in logs
// as readable strings rather than integers. This is a small quality-of-life
// check — when operators see "mode=trust_token" in a slog.Warn line they
// should be able to map it back to the option without consulting the source.
func TestAuthz_DBFailureMode_String(t *testing.T) {
	cases := []struct {
		mode middleware.DBFailureMode
		want string
	}{
		{middleware.ModeServiceUnavailable, "service_unavailable"},
		{middleware.ModeDeny, "deny"},
		{middleware.ModeTrustToken, "trust_token"},
	}
	for _, tc := range cases {
		if got := tc.mode.String(); got != tc.want {
			t.Errorf("mode.String() = %q, want %q", got, tc.want)
		}
	}
}
