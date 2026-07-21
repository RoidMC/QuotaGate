package middleware

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"strings"
)

type contextKey string

const (
	userIDKey    contextKey = "user_id"
	userRolesKey contextKey = "user_roles"
)

func WithUserID(ctx context.Context, userID string) context.Context {
	return context.WithValue(ctx, userIDKey, userID)
}

func GetUserID(ctx context.Context) string {
	v, _ := ctx.Value(userIDKey).(string)
	return v
}

func WithUserRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, userRolesKey, roles)
}

func GetUserRoles(ctx context.Context) []string {
	v, _ := ctx.Value(userRolesKey).([]string)
	return v
}

// HasRole reports whether the current context carries the given role.
// Convenience wrapper around GetUserRoles so handlers don't have to
// re-implement the linear scan every time.
func HasRole(ctx context.Context, role string) bool {
	for _, r := range GetUserRoles(ctx) {
		if r == role {
			return true
		}
	}
	return false
}

func ClientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		parts := strings.Split(xff, ",")
		return strings.TrimSpace(parts[0])
	}
	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return xri
	}
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	return host
}

// RequestScheme returns the request scheme ("https" or "http"), honoring the
// X-Forwarded-Proto / X-Forwarded-Scheme headers that a reverse proxy sets
// when terminating TLS. Used to derive the WebAuthn origin from the request
// Host.
func RequestScheme(r *http.Request) string {
	if p := strings.ToLower(r.Header.Get("X-Forwarded-Proto")); p == "http" || p == "https" {
		return p
	}
	if p := strings.ToLower(r.Header.Get("X-Forwarded-Scheme")); p == "http" || p == "https" {
		return p
	}
	if r.TLS != nil {
		return "https"
	}
	return "http"
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
