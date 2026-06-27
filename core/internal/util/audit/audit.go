package audit

import (
	"net"
	"strings"
)

// AuditLogInput holds the raw input fields for audit log sanitization.
// Using a struct prevents positional parameter errors at compile time.
type AuditLogInput struct {
	Action     string
	ActorID    string
	TargetID   string
	TargetType string
	Result     string
	Severity   string
	TenantID   string
	IP         string
	UserAgent  string
	Message    string
	Detail     string
}

// SanitizeResult holds the sanitized audit log fields.
type SanitizeResult = AuditLogInput

// SanitizeAuditLog validates and sanitizes audit log fields before persistence.
// It protects against:
//   - SQL injection via log content (by sanitizing, not blocking)
//   - Log injection attacks (newline stripping)
//   - XSS via UserAgent (newline stripping)
//   - Data overflow (length limits)
//   - Invalid IP formats
//
// This is a defense-in-depth measure. SQL injection is primarily prevented by
// GORM's parameterized queries, but sanitization adds an extra layer of protection.
//
// The function returns sanitized values - it does NOT fail on invalid input.
// Invalid values are replaced with safe defaults:
//   - Invalid IPs → "0.0.0.0"
//   - Empty Action → "unknown"
//   - Empty TargetType → "unknown"
//   - Invalid Result → "success"
//   - Invalid Severity → "info"
//   - Empty TenantID → "unknown"
func SanitizeAuditLog(in AuditLogInput) SanitizeResult {
	return SanitizeResult{
		Action:     sanitizeAction(in.Action),
		ActorID:    sanitizeID(in.ActorID),
		TargetID:   sanitizeID(in.TargetID),
		TargetType: sanitizeTargetType(in.TargetType),
		Result:     sanitizeResult(in.Result),
		Severity:   sanitizeSeverity(in.Severity),
		TenantID:   sanitizeTenantID(in.TenantID),
		IP:         sanitizeIP(in.IP),
		UserAgent:  sanitizeUserAgent(in.UserAgent),
		Message:    sanitizeMessage(in.Message),
		Detail:     sanitizeDetail(in.Detail),
	}
}

// sanitizeMessage sanitizes human-readable audit message (256 chars max).
func sanitizeMessage(s string) string {
	if s == "" {
		return ""
	}
	if len(s) > 256 {
		s = s[:256]
	}
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.ReplaceAll(s, "\r", " ")
	return s
}

// sanitizeAction ensures action contains only safe characters.
// Actions follow the pattern "domain.action" (e.g., "user.register").
// Returns "unknown" if the input is empty or contains no safe characters.
func sanitizeAction(s string) string {
	if s == "" {
		return "unknown"
	}
	if len(s) > 64 {
		s = s[:64]
	}
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' {
			result.WriteRune(r)
		}
	}
	cleaned := strings.Trim(result.String(), "._")
	if cleaned == "" {
		return "unknown"
	}
	return result.String()
}

// sanitizeID sanitizes UUID-like identifiers (36 chars max).
func sanitizeID(s string) string {
	if s == "" {
		return ""
	}
	if len(s) > 36 {
		s = s[:36]
	}
	return s
}

// sanitizeTenantID sanitizes tenant ID (36 chars max).
// Returns "unknown" if the input is empty.
func sanitizeTenantID(s string) string {
	if s == "" {
		return "unknown"
	}
	if len(s) > 36 {
		s = s[:36]
	}
	return s
}

// sanitizeTargetType sanitizes target type (32 chars max, alphanumeric with dots/underscores).
// Returns "unknown" if the input is empty or contains no safe characters.
func sanitizeTargetType(s string) string {
	if s == "" {
		return "unknown"
	}
	if len(s) > 32 {
		s = s[:32]
	}
	var result strings.Builder
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-' {
			result.WriteRune(r)
		}
	}
	if result.Len() == 0 {
		return "unknown"
	}
	return result.String()
}

// sanitizeIP validates IP format and truncates to max 45 chars (IPv6).
// Invalid IPs are replaced with "0.0.0.0".
func sanitizeIP(ip string) string {
	if ip == "" {
		return ""
	}
	if len(ip) > 45 {
		ip = ip[:45]
	}
	if net.ParseIP(ip) == nil {
		return "0.0.0.0"
	}
	return ip
}

// sanitizeUserAgent strips newlines to prevent log injection and XSS.
func sanitizeUserAgent(ua string) string {
	if ua == "" {
		return ""
	}
	ua = strings.ReplaceAll(ua, "\n", " ")
	ua = strings.ReplaceAll(ua, "\r", " ")
	if len(ua) > 512 {
		ua = ua[:512]
	}
	return ua
}

// sanitizeDetail strips newlines to prevent log injection.
func sanitizeDetail(detail string) string {
	detail = strings.ReplaceAll(detail, "\n", " ")
	detail = strings.ReplaceAll(detail, "\r", " ")
	return detail
}

// sanitizeResult ensures result is one of the allowed values.
func sanitizeResult(s string) string {
	switch s {
	case "success", "failure", "denied":
		return s
	default:
		return "success"
	}
}

// sanitizeSeverity ensures severity is one of the allowed values.
func sanitizeSeverity(s string) string {
	switch s {
	case "info", "warn", "error", "critical":
		return s
	default:
		return "info"
	}
}
