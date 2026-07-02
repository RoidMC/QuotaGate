package middleware

import (
	"net/http"
	"strings"

	"github.com/roidmc/quotagate/internal/authz"
	kexerrors "github.com/roidmc/quotagate/internal/errors"
)

func getRoleFromCtx(r *http.Request) string {
	return GetUserRole(r.Context())
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

func Authz(authzManager *authz.AuthzManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path, ok := sanitizePath(r.URL.Path)
			if !ok {
				kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
				return
			}

			role := getRoleFromCtx(r)
			allowed, err := authzManager.Enforce(role, path, r.Method)
			if err != nil {
				kexerrors.AbortInternalError(w, kexerrors.ErrInternalError)
				return
			}

			if !allowed {
				kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func AuthzWithResource(action string, authzManager *authz.AuthzManager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path, ok := sanitizePath(r.URL.Path)
			if !ok {
				kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
				return
			}

			role := getRoleFromCtx(r)
			allowed, err := authzManager.Enforce(role, path, action)
			if err != nil {
				kexerrors.AbortInternalError(w, kexerrors.ErrInternalError)
				return
			}

			if !allowed {
				kexerrors.AbortForbidden(w, kexerrors.ErrForbidden)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
