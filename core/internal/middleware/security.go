// QuotaGate global security middleware: rate limiting, request size limits,
// path traversal protection, and other request-level safeguards.
// These are applied independently of authentication state.

package middleware

import (
	"net/http"

	kexerrors "github.com/roidmc/quotagate/internal/errors"
)

const (
	// DefaultMaxRequestURI is the default maximum URI length (8192 bytes).
	DefaultMaxRequestURI = 8192

	// DefaultMaxRequestBody is the default maximum request body size (1MB).
	DefaultMaxRequestBody = 1 << 20 // 1MB
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
