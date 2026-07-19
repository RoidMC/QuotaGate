package middleware

import (
	"context"
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

// AuthzOption configures the Authz middleware.
type AuthzOption func(*authzMiddleware)

// WithRoleResolver injects a RoleResolver. When set, strict token validation
// is performed on every request.
func WithRoleResolver(r RoleResolver) AuthzOption {
	return func(m *authzMiddleware) {
		m.roleResolver = r
	}
}

type authzMiddleware struct {
	manager      *authz.AuthzManager
	resolver     RuleTypeResolver
	roleResolver RoleResolver
}

func getRolesFromCtx(r *http.Request) []string {
	roles := GetUserRoles(r.Context())
	if len(roles) == 0 {
		if role := GetUserRole(r.Context()); role != "" {
			roles = []string{role}
		}
	}
	return roles
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
		manager:  authzManager,
		resolver: resolver,
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
				if !mw.rolesMatchDB(r.Context(), userID, roles) {
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

func (m *authzMiddleware) rolesMatchDB(ctx context.Context, userID string, tokenRoles []string) bool {
	dbRoles, err := m.roleResolver.EffectiveRoles(ctx, userID)
	if err != nil {
		return false
	}
	return roleSetsEqual(tokenRoles, dbRoles)
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
