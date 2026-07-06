// QuotaGate global security middleware: rate limiting, request size limits,
// path traversal protection, response header enforcement, and other
// request-level safeguards.
// These are applied independently of authentication state.

package middleware

import (
	"fmt"
	"net/http"

	"github.com/roidmc/quotagate/internal/config"
	kexerrors "github.com/roidmc/quotagate/internal/errors"
)

const (
	// DefaultMaxRequestURI is the default maximum URI length (8192 bytes).
	DefaultMaxRequestURI = 8192

	// DefaultMaxRequestBody is the default maximum request body size (1MB).
	DefaultMaxRequestBody = 1 << 20 // 1MB

	// PoweredByName is the product name part of the X-Powered-By header.
	// This is hard-coded for MPL-2.0 compliance; removing it triggers
	// source disclosure obligations.
	PoweredByName = "QuotaGate"
)

// LimitRequestURI rejects requests whose URI exceeds the configured limit.
// Prevents ReDoS in path matching and OOM from excessively long request lines.
func LimitRequestURI(maxBytes int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if len(r.RequestURI) > maxBytes {
				kexerrors.Abort(w, http.StatusRequestURITooLong, kexerrors.URITooLong())
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// LimitRequestBody rejects requests whose body exceeds the given size.
// This prevents ReDoS-style attacks and OOM from long headers.
func LimitRequestBody(maxBytes int) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Body = http.MaxBytesReader(w, r.Body, int64(maxBytes))
			next.ServeHTTP(w, r)
		})
	}
}

// HTTPHeaders sets branding and standard security response headers on all
// responses. The X-Powered-By header is a brand attribution requirement for
// using QuotaGate; removing it requires source modification, which triggers
// MPL-2.0 obligations to redistribute the modified code.
func HTTPHeaders(cfg *config.Config) func(http.Handler) http.Handler {
	serverHeader := cfg.Server.Name + "/" + cfg.Server.Version
	poweredByHeader := fmt.Sprintf("%s/%s", PoweredByName, cfg.Server.Version)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Server", serverHeader)
			w.Header().Set("X-Powered-By", poweredByHeader)
			w.Header().Set("X-Content-Type-Options", cfg.HTTP.Headers.XContentTypeOptions)
			w.Header().Set("X-Frame-Options", cfg.HTTP.Headers.XFrameOptions)
			w.Header().Set("X-XSS-Protection", cfg.HTTP.Headers.XXSSProtection)
			next.ServeHTTP(w, r)
		})
	}
}
