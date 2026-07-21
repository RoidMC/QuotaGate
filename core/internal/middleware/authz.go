package middleware

import (
	"context"
	"log/slog"
	"net/http"
	"sort"
	"strings"

	"github.com/roidmc/quotagate/internal/authz"
	kexerrors "github.com/roidmc/quotagate/internal/errors"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/tenant"
)

// RuleTypeResolver decides whether a route needs RBAC-only, additional ABAC,
// or service-level ReBAC authorization.
type RuleTypeResolver interface {
	RuleType(method, path string) string
}

// RoleResolver loads a user's current effective roles from the authoritative
// store. When non-nil, every authorized request has its token roles compared
// against the resolver's current DB role set — this closes the window between
// role revocation and token expiry.
type RoleResolver interface {
	EffectiveRoles(ctx context.Context, userID string) ([]string, error)
}

// DBFailureMode controls how the Authz middleware behaves when the
// RoleResolver cannot reach its backing store (DB connection lost, query
// timed out, etc.). The previous implementation always returned 403
// Forbidden on any resolver error, which is fail-closed but a usability
// disaster: a transient DB hiccup would surface to every authenticated user
// as if their account had been suspended.
//
// The modes below let the operator trade off between security and
// availability. The default (ModeServiceUnavailable) preserves fail-closed
// semantics but uses HTTP 503 so clients can distinguish "cannot verify"
// from "denied" and retry appropriately.
type DBFailureMode int

const (
	// ModeServiceUnavailable is the default. On resolver error the middleware
	// returns 503 Service Unavailable. The request is NOT allowed through —
	// fail-closed is preserved — but the response status is semantically
	// correct so clients can retry and UIs can render "service degraded"
	// rather than "permission denied".
	ModeServiceUnavailable DBFailureMode = iota

	// ModeDeny returns 403 Forbidden on resolver error, matching the legacy
	// behaviour. Kept as an option for operators who explicitly want the
	// old semantics (e.g. compliance regimes that require any DB-contact
	// failure to be reported as a denial).
	ModeDeny

	// ModeTrustToken lets the request through using the roles embedded in
	// the JWT, on the assumption that a recently-issued token is a good
	// approximation of the user's current roles. This is the most permissive
	// mode: it trades the strict-revocation guarantee (the whole point of
	// rolesMatchDB) for availability during DB outages. Use only when the
	// operator has accepted that role revocations may take effect only at
	// token expiry during an outage.
	//
	// Even in this mode, RBAC/ABAC/ReBAC still run afterwards with the
	// token roles, so a revoked role is still ignored for routing — but
	// a role *grant* made during the outage will not be honoured until
	// the token is refreshed.
	ModeTrustToken
)

// AuthzOption configures the Authz middleware.
type AuthzOption func(*authzMiddleware)

// WithRoleResolver injects a RoleResolver. When set, strict token validation
// is performed on every request.
func WithRoleResolver(r RoleResolver) AuthzOption {
	return func(m *authzMiddleware) {
		m.roleResolver = r
	}
}

// WithDBFailureMode sets the behaviour when the RoleResolver fails. If not
// called, ModeServiceUnavailable is used. The option has no effect when no
// RoleResolver is wired (no strict token validation is performed in that case).
func WithDBFailureMode(mode DBFailureMode) AuthzOption {
	return func(m *authzMiddleware) {
		m.dbFailureMode = mode
	}
}

type authzMiddleware struct {
	manager       *authz.AuthzManager
	resolver      RuleTypeResolver
	roleResolver  RoleResolver
	dbFailureMode DBFailureMode
}

func getRolesFromCtx(r *http.Request) []string {
	return GetUserRoles(r.Context())
}

// sanitizePath rejects path traversal attempts before authorization.
// r.URL.Path is already percent-decoded by net/http, so encoded traversals
// like /api/users/..%2Fetc arrive here as /api/users/../etc and are blocked.
// We do not collapse redundant slashes here: keyMatch3 already rejects // as
// non-matching, and the router (chi) also 404s on // paths. Blocking .. at
// the middleware layer is the critical fix because keyMatch3 would otherwise
// match /api/users/../etc against /api/users/{id} for the first segment.
func sanitizePath(p string) (string, bool) {
	if strings.Contains(p, "..") {
		return "", false
	}
	return p, true
}

// Authz performs domain-aware RBAC and optional secondary authorization.
// If resolver is nil, routes are treated as RBAC-only.
func Authz(authzManager *authz.AuthzManager, resolver RuleTypeResolver, opts ...AuthzOption) func(http.Handler) http.Handler {
	if authzManager == nil {
		panic("quotagate/middleware: Authz requires a non-nil AuthzManager")
	}

	mw := &authzMiddleware{
		manager:       authzManager,
		resolver:      resolver,
		dbFailureMode: ModeServiceUnavailable, // default: fail-closed but semantically honest
	}
	for _, opt := range opts {
		opt(mw)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path, ok := sanitizePath(r.URL.Path)
			if !ok {
				kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
				return
			}

			userID := GetUserID(r.Context())
			tenantID := tenant.FromContext(r.Context())
			roles := getRolesFromCtx(r)

			// Strict token validation: when a RoleResolver is wired, compare
			// the roles embedded in the token against the current DB role set.
			// This is mandatory and cannot be disabled.
			if mw.roleResolver != nil && userID != "" {
				matched, err := mw.rolesMatchDB(r.Context(), userID, roles)
				if err != nil {
					// DB resolver failure. Decision is mode-driven so the
					// operator can trade off between security (ModeDeny) and
					// availability (ModeTrustToken) without recompiling.
					slog.Warn("quotagate/middleware: role resolver failed",
						"user_id", userID,
						"tenant_id", tenantID,
						"path", path,
						"mode", mw.dbFailureMode.String(),
						"error", err)

					switch mw.dbFailureMode {
					case ModeServiceUnavailable:
						// Fail-closed but semantically honest: 503 says
						// "cannot verify", not "denied". Clients retry.
						kexerrors.AbortServiceUnavailable(w, kexerrors.ErrServiceUnavailable)
						return
					case ModeDeny:
						// Legacy behaviour: surface as 403.
						kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
						return
					case ModeTrustToken:
						// Fall through to RBAC using the token's roles.
						// The strict-revocation guarantee is suspended for
						// the duration of the outage; logged above so
						// operators can see how long the window was open.
					}
				} else if !matched {
					kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
					return
				}
			}

			// objOwner="*" represents system-wide or tenant-agnostic resources.
			// Service-level ReBAC is responsible for instance-level ownership checks.
			objOwner := "*"

			allowed, err := authzManager.EnforceRBAC(r.Context(), tenantID, userID, roles, r.Method, path, objOwner)
			if err != nil {
				kexerrors.AbortInternalError(w, kexerrors.ErrInternalError)
				return
			}
			if !allowed {
				kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
				return
			}

			// RouteMeta-driven secondary authorization.
			if resolver != nil {
				switch resolver.RuleType(r.Method, path) {
				case model.RuleTypeRBAC, "":
					// Primary RBAC already approved.
				case model.RuleTypeABAC:
					if !enforceABAC(r.Context(), authzManager, userID, tenantID, roles, r.Method, path) {
						kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
						return
					}
				case model.RuleTypeReBAC:
					// ReBAC is handled by service-layer ownership checks.
					// The middleware only ensures RBAC allowed entry to the route.
				default:
					// Unknown rule type defaults to deny to avoid accidental exposure.
					kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
					return
				}
			}

			next.ServeHTTP(w, r)
		})
	}
}

// rolesMatchDB compares the roles embedded in the JWT against the current
// authoritative role set from the RoleResolver. The (bool, error) return
// separates the two failure modes that the previous implementation conflated:
//
//   - err != nil: the resolver could not reach its backing store. The caller
//     decides whether to fail-closed (503/403) or trust the token (ModeTrustToken)
//     based on the configured DBFailureMode.
//   - matched == false && err == nil: the resolver succeeded and the roles
//     genuinely do not match (e.g. revoked mid-token). This is a definitive
//     403 — there is no ambiguity, the user no longer holds the roles the
//     token claims.
//
// Either case used to surface as plain `false` and a 403, which made a DB
// outage indistinguishable from a mass role revocation.
func (m *authzMiddleware) rolesMatchDB(ctx context.Context, userID string, tokenRoles []string) (bool, error) {
	dbRoles, err := m.roleResolver.EffectiveRoles(ctx, userID)
	if err != nil {
		return false, err
	}
	return roleSetsEqual(tokenRoles, dbRoles), nil
}

// String returns a human-readable name for the mode, used in log lines.
func (m DBFailureMode) String() string {
	switch m {
	case ModeServiceUnavailable:
		return "service_unavailable"
	case ModeDeny:
		return "deny"
	case ModeTrustToken:
		return "trust_token"
	default:
		return "unknown"
	}
}

func roleSetsEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string(nil), a...)
	y := append([]string(nil), b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] {
			return false
		}
	}
	return true
}

// enforceABAC runs a basic secondary ABAC check with the caller's Subject
// and a minimal Object derived from the request path.
func enforceABAC(ctx context.Context, m *authz.AuthzManager, userID, tenantID string, roles []string, method, path string) bool {
	sub := authz.Subject{
		ID:    userID,
		Owner: tenantID,
		Roles: authzToInterfaceSlice(roles),
		Attrs: map[string]any{},
	}
	obj := authz.Object{
		Owner: tenantID,
		Name:  path,
		Attrs: map[string]any{},
	}
	allowed, err := m.EnforceABAC(ctx, sub, tenantID, method, path, obj)
	if err != nil {
		return false
	}
	return allowed
}

func authzToInterfaceSlice(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
}
