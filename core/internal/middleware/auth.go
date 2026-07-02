// QuotaGate universal middleware, customized and modified based on KexCore IAM project

package middleware

import (
	"net/http"
	"strings"

	"github.com/roidmc/quotagate/internal/config"
	kexerrors "github.com/roidmc/quotagate/internal/errors"
	"github.com/roidmc/quotagate/internal/service"
	kexjwt "github.com/roidmc/quotagate/pkg/jwt"
)

func HTTPHeaders(cfg *config.Config) func(http.Handler) http.Handler {
	serverHeader := cfg.Server.Name + "/" + cfg.Server.Version
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", serverHeader)
			w.Header().Set("X-Content-Type-Options", cfg.HTTP.Headers.XContentTypeOptions)
			w.Header().Set("X-Frame-Options", cfg.HTTP.Headers.XFrameOptions)
			w.Header().Set("X-XSS-Protection", cfg.HTTP.Headers.XXSSProtection)
			next.ServeHTTP(w, r)
		})
	}
}

func BearerAuth(issuer *service.TokenIssuer) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				kexerrors.AbortUnauthorized(w, kexerrors.MissingAuthHeader())
				return
			}

			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				kexerrors.AbortUnauthorized(w, kexerrors.InvalidAuthHeader())
				return
			}

			token := parts[1]
			if token == "" {
				kexerrors.AbortUnauthorized(w, kexerrors.EmptyToken())
				return
			}

			claims, err := issuer.ParseAccessToken(r.Context(), token)
			if err != nil {
				if err == kexjwt.ErrExpiredToken {
					kexerrors.AbortUnauthorized(w, kexerrors.TokenExpired())
					return
				}
				kexerrors.AbortUnauthorized(w, kexerrors.TokenInvalid())
				return
			}

			ctx := WithUserID(r.Context(), claims.UserID)
			ctx = WithUserRole(ctx, claims.Role)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
