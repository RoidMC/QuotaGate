package audit_test

import (
	"strings"
	"testing"

	"github.com/roidmc/quotagate/internal/util/audit"
)

func TestSanitizeAuditLog_NormalInput(t *testing.T) {
	r := audit.SanitizeAuditLog(audit.AuditLogInput{
		Action:     "user.register",
		ActorID:    "550e8400-e29b-41d4-a716-446655440000",
		TargetID:   "660e8400-e29b-41d4-a716-446655440001",
		TargetType: "user",
		Result:     "success",
		Severity:   "info",
		IP:         "192.168.1.1",
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
		Detail:     `{"key":"value"}`,
	})

	if r.Action != "user.register" {
		t.Errorf("Action = %q, want %q", r.Action, "user.register")
	}
	if r.ActorID != "550e8400-e29b-41d4-a716-446655440000" {
		t.Errorf("ActorID = %q, want %q", r.ActorID, "550e8400-e29b-41d4-a716-446655440000")
	}
	if r.TargetID != "660e8400-e29b-41d4-a716-446655440001" {
		t.Errorf("TargetID = %q, want %q", r.TargetID, "660e8400-e29b-41d4-a716-446655440001")
	}
	if r.TargetType != "user" {
		t.Errorf("TargetType = %q, want %q", r.TargetType, "user")
	}
	if r.Result != "success" {
		t.Errorf("Result = %q, want %q", r.Result, "success")
	}
	if r.Severity != "info" {
		t.Errorf("Severity = %q, want %q", r.Severity, "info")
	}
	if r.IP != "192.168.1.1" {
		t.Errorf("IP = %q, want %q", r.IP, "192.168.1.1")
	}
	if r.UserAgent != "Mozilla/5.0 (Windows NT 10.0; Win64; x64)" {
		t.Errorf("UserAgent = %q, want %q", r.UserAgent, "Mozilla/5.0 (Windows NT 10.0; Win64; x64)")
	}
	if r.Detail != `{"key":"value"}` {
		t.Errorf("Detail = %q, want %q", r.Detail, `{"key":"value"}`)
	}
}

func TestSanitizeAuditLog_EmptyInput(t *testing.T) {
	r := audit.SanitizeAuditLog(audit.AuditLogInput{})

	if r.Action != "unknown" {
		t.Errorf("Action should be unknown, got %q", r.Action)
	}
	if r.ActorID != "" {
		t.Errorf("ActorID should be empty, got %q", r.ActorID)
	}
	if r.TargetID != "" {
		t.Errorf("TargetID should be empty, got %q", r.TargetID)
	}
	if r.TargetType != "unknown" {
		t.Errorf("TargetType should be unknown, got %q", r.TargetType)
	}
	if r.Result != "success" {
		t.Errorf("Result should default to success, got %q", r.Result)
	}
	if r.Severity != "info" {
		t.Errorf("Severity should default to info, got %q", r.Severity)
	}
	if r.TenantID != "unknown" {
		t.Errorf("TenantID should default to unknown, got %q", r.TenantID)
	}
	if r.IP != "" {
		t.Errorf("IP should be empty, got %q", r.IP)
	}
	if r.UserAgent != "" {
		t.Errorf("UserAgent should be empty, got %q", r.UserAgent)
	}
	if r.Detail != "" {
		t.Errorf("Detail should be empty, got %q", r.Detail)
	}
}

func TestSanitizeAuditLog_Action(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal action", "user.register", "user.register"},
		{"with underscores", "policy.create", "policy.create"},
		{"with numbers", "auth2.login", "auth2.login"},
		{"multi-level", "admin.user.delete", "admin.user.delete"},
		{"uppercase", "USER.LOGIN", "USER.LOGIN"},
		{"mixed case", "User.Login", "User.Login"},
		{"with special chars", "user@login!", "userlogin"},
		{"with spaces", "user login", "userlogin"},
		{"with angle brackets", "<script>alert(1)</script>", "scriptalert1script"},
		{"with sql injection", "'; DROP TABLE users; --", "DROPTABLEusers"},
		{"with newline", "user.\nlogin", "user.login"},
		{"with carriage return", "user.\rlogin", "user.login"},
		{"with tab", "user.\tlogin", "user.login"},
		{"with slashes", "user/login", "userlogin"},
		{"with backslashes", "user\\login", "userlogin"},
		{"with quotes", `user."login"`, "user.login"},
		{"with dashes", "user-login", "userlogin"},
		{"empty", "", "unknown"},
		{"only special chars", "@#$%", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{Action: tt.input})
			if r.Action != tt.want {
				t.Errorf("SanitizeAuditLog(%q).Action = %q, want %q", tt.input, r.Action, tt.want)
			}
		})
	}
}

func TestSanitizeAuditLog_ActionOverflow(t *testing.T) {
	long := strings.Repeat("a", 100)
	r := audit.SanitizeAuditLog(audit.AuditLogInput{Action: long})
	if len(r.Action) > 64 {
		t.Errorf("Action length %d exceeds max 64", len(r.Action))
	}
	if r.Action != strings.Repeat("a", 64) {
		t.Errorf("Action should be truncated to 64 chars, got %d", len(r.Action))
	}
}

func TestSanitizeAuditLog_ActionStripsUnsafeChars(t *testing.T) {
	unsafe := []string{
		"user\nregister",
		"user\rregister",
		"user\tregister",
		"user<script>register",
		"user'register",
		`user"register`,
		"user;register",
		"user--register",
	}

	for _, input := range unsafe {
		t.Run(input, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{Action: input})
			if strings.ContainsAny(r.Action, "\n\r\t<>'\";-") {
				t.Errorf("Action %q contains unsafe characters", r.Action)
			}
		})
	}
}

func TestSanitizeAuditLog_ID(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal uuid", "550e8400-e29b-41d4-a716-446655440000", "550e8400-e29b-41d4-a716-446655440000"},
		{"empty", "", ""},
		{"short", "abc", "abc"},
		{"with special chars", "abc!@#", "abc!@#"},
		{"with spaces", "abc def", "abc def"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{ActorID: tt.input})
			if r.ActorID != tt.want {
				t.Errorf("SanitizeAuditLog(actorID=%q).ActorID = %q, want %q", tt.input, r.ActorID, tt.want)
			}
		})
	}
}

func TestSanitizeAuditLog_IDOverflow(t *testing.T) {
	long := strings.Repeat("a", 50)
	r := audit.SanitizeAuditLog(audit.AuditLogInput{ActorID: long})
	if len(r.ActorID) > 36 {
		t.Errorf("ActorID length %d exceeds max 36", len(r.ActorID))
	}
	if r.ActorID != strings.Repeat("a", 36) {
		t.Errorf("ActorID should be truncated to 36 chars, got %d", len(r.ActorID))
	}
}

func TestSanitizeAuditLog_TargetID(t *testing.T) {
	r := audit.SanitizeAuditLog(audit.AuditLogInput{TargetID: "target-123"})
	if r.TargetID != "target-123" {
		t.Errorf("TargetID = %q, want %q", r.TargetID, "target-123")
	}
}

func TestSanitizeAuditLog_TargetType(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal", "user", "user"},
		{"with dots", "custom.type", "custom.type"},
		{"with underscores", "my_type", "my_type"},
		{"with dashes", "my-type", "my-type"},
		{"with numbers", "type2", "type2"},
		{"with special chars", "user@type!", "usertype"},
		{"with spaces", "user type", "usertype"},
		{"with angle brackets", "<type>", "type"},
		// TargetType whitelist allows hyphens, so "--" is preserved.
		// This is acceptable because TargetType is used in structured queries, not raw SQL.
		{"with sql injection", "'; DROP TABLE; --", "DROPTABLE--"},
		{"empty", "", "unknown"},
		{"only special chars", "@#$%", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{TargetType: tt.input})
			if r.TargetType != tt.want {
				t.Errorf("SanitizeAuditLog(targetType=%q).TargetType = %q, want %q", tt.input, r.TargetType, tt.want)
			}
		})
	}
}

func TestSanitizeAuditLog_TargetTypeOverflow(t *testing.T) {
	long := strings.Repeat("a", 50)
	r := audit.SanitizeAuditLog(audit.AuditLogInput{TargetType: long})
	if len(r.TargetType) > 32 {
		t.Errorf("TargetType length %d exceeds max 32", len(r.TargetType))
	}
	if r.TargetType != strings.Repeat("a", 32) {
		t.Errorf("TargetType should be truncated to 32 chars, got %d", len(r.TargetType))
	}
}

func TestSanitizeAuditLog_IP(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"ipv4", "192.168.1.1", "192.168.1.1"},
		{"ipv4 localhost", "127.0.0.1", "127.0.0.1"},
		{"ipv6", "::1", "::1"},
		{"ipv6 full", "2001:db8::1", "2001:db8::1"},
		{"ipv6 long", "2001:0db8:85a3:0000:0000:8a2e:0370:7334", "2001:0db8:85a3:0000:0000:8a2e:0370:7334"},
		{"empty", "", ""},
		{"invalid text", "not-an-ip", "0.0.0.0"},
		{"invalid numbers", "999.999.999.999", "0.0.0.0"},
		{"invalid format", "abc.def.ghi.jkl", "0.0.0.0"},
		{"sql injection", "'; DROP TABLE; --", "0.0.0.0"},
		{"xss attempt", "<script>alert(1)</script>", "0.0.0.0"},
		{"partial ip", "192.168", "0.0.0.0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{IP: tt.input})
			if r.IP != tt.want {
				t.Errorf("SanitizeAuditLog(ip=%q).IP = %q, want %q", tt.input, r.IP, tt.want)
			}
		})
	}
}

func TestSanitizeAuditLog_IPOverflow(t *testing.T) {
	long := strings.Repeat("a", 50)
	r := audit.SanitizeAuditLog(audit.AuditLogInput{IP: long})
	if len(r.IP) > 45 {
		t.Errorf("IP length %d exceeds max 45", len(r.IP))
	}
}

func TestSanitizeAuditLog_UserAgent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal browser", "Mozilla/5.0 (Windows NT 10.0)", "Mozilla/5.0 (Windows NT 10.0)"},
		{"empty", "", ""},
		{"with newline", "Mozilla/5.0\nXSS: injected", "Mozilla/5.0 XSS: injected"},
		{"with carriage return", "Mozilla/5.0\rXSS: injected", "Mozilla/5.0 XSS: injected"},
		{"with both newlines", "line1\nline2\rline3", "line1 line2 line3"},
		// UserAgent is free-text; HTML escaping is the responsibility of the rendering layer.
		{"with script tag", "<script>alert(1)</script>", "<script>alert(1)</script>"},
		{"with sql injection", "'; DROP TABLE; --", "'; DROP TABLE; --"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{UserAgent: tt.input})
			if r.UserAgent != tt.want {
				t.Errorf("SanitizeAuditLog(userAgent=%q).UserAgent = %q, want %q", tt.input, r.UserAgent, tt.want)
			}
		})
	}
}

func TestSanitizeAuditLog_UserAgentNoNewlines(t *testing.T) {
	inputs := []string{
		"normal\ninjection",
		"normal\rinjection",
		"normal\r\ninjection",
		"\nleading",
		"trailing\n",
		"multi\nline\r\nattack",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{UserAgent: input})
			if strings.ContainsAny(r.UserAgent, "\n\r") {
				t.Errorf("UserAgent %q still contains newline characters", r.UserAgent)
			}
		})
	}
}

func TestSanitizeAuditLog_UserAgentOverflow(t *testing.T) {
	long := strings.Repeat("a", 600)
	r := audit.SanitizeAuditLog(audit.AuditLogInput{UserAgent: long})
	if len(r.UserAgent) > 512 {
		t.Errorf("UserAgent length %d exceeds max 512", len(r.UserAgent))
	}
	if r.UserAgent != strings.Repeat("a", 512) {
		t.Errorf("UserAgent should be truncated to 512 chars, got %d", len(r.UserAgent))
	}
}

func TestSanitizeAuditLog_Detail(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"normal json", `{"key":"value"}`, `{"key":"value"}`},
		{"empty", "", ""},
		{"with newline", "line1\nline2", "line1 line2"},
		{"with carriage return", "line1\rline2", "line1 line2"},
		{"with both", "line1\nline2\rline3", "line1 line2 line3"},
		{"with script tag", "<script>alert(1)</script>", "<script>alert(1)</script>"},
		{"with sql injection", "'; DROP TABLE; --", "'; DROP TABLE; --"},
		{"long text", strings.Repeat("x", 1000), strings.Repeat("x", 1000)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{Detail: tt.input})
			if r.Detail != tt.want {
				t.Errorf("SanitizeAuditLog(detail=%q).Detail = %q, want %q", tt.input, r.Detail, tt.want)
			}
		})
	}
}

func TestSanitizeAuditLog_DetailNoNewlines(t *testing.T) {
	inputs := []string{
		"normal\ninjection",
		"normal\rinjection",
		"normal\r\ninjection",
		"\nleading",
		"trailing\n",
		"multi\nline\r\nattack",
	}

	for _, input := range inputs {
		t.Run(input, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{Detail: input})
			if strings.ContainsAny(r.Detail, "\n\r") {
				t.Errorf("Detail %q still contains newline characters", r.Detail)
			}
		})
	}
}

func TestSanitizeAuditLog_IPv6(t *testing.T) {
	ipv6Addrs := []string{
		"::1",
		"2001:db8::1",
		"2001:0db8:85a3:0000:0000:8a2e:0370:7334",
		"fe80::1",
		"::ffff:192.168.1.1",
	}

	for _, ip := range ipv6Addrs {
		t.Run(ip, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{IP: ip})
			if r.IP != ip {
				t.Errorf("IPv6 %q should be preserved, got %q", ip, r.IP)
			}
		})
	}
}

func TestSanitizeAuditLog_LogInjectionAttempt(t *testing.T) {
	// Simulate a log injection attack via UserAgent
	attack := "Mozilla/5.0\n2024-01-01 12:00:00 [INFO] user.login SUCCESS\nFake log entry"
	r := audit.SanitizeAuditLog(audit.AuditLogInput{UserAgent: attack})
	if strings.Contains(r.UserAgent, "\n") {
		t.Error("UserAgent should not contain newlines after sanitization")
	}
}

func TestSanitizeAuditLog_XSSAttempt(t *testing.T) {
	attack := "<script>document.cookie</script>"
	r := audit.SanitizeAuditLog(audit.AuditLogInput{
		Action:     "user.login",
		ActorID:    "actor-id",
		TargetID:   "target-id",
		TargetType: "user",
		Result:     "success",
		Severity:   "info",
		IP:         "127.0.0.1",
		UserAgent:  attack,
		Detail:     attack,
	})

	if r.Action != "user.login" {
		t.Errorf("Action should remain valid, got %q", r.Action)
	}
	if r.IP != "127.0.0.1" {
		t.Errorf("IP should remain valid, got %q", r.IP)
	}
}

func TestSanitizeAuditLog_SQLInjectionAttempt(t *testing.T) {
	action := "'; DROP TABLE audit_logs; --"
	ip := "1.2.3.4'; DROP TABLE users; --"
	detail := "'; DELETE FROM policies; --"

	r := audit.SanitizeAuditLog(audit.AuditLogInput{
		Action: action,
		IP:     ip,
		Detail: detail,
	})

	// Action strips special chars: '; DROP TABLE audit_logs; -- → DROPTABLEaudit_logs
	if strings.ContainsAny(r.Action, "';\t\n\r") {
		t.Errorf("Action should not contain dangerous chars: %q", r.Action)
	}
	// IP is invalid → falls back to 0.0.0.0
	if r.IP != "0.0.0.0" {
		t.Errorf("IP should fallback to 0.0.0.0 for invalid IP, got %q", r.IP)
	}
	// Detail only strips newlines, not quotes or SQL keywords
	if strings.Contains(r.Detail, "\n") || strings.Contains(r.Detail, "\r") {
		t.Errorf("Detail should not contain newlines: %q", r.Detail)
	}
}

func TestSanitizeAuditLog_AllFieldsMaxLength(t *testing.T) {
	r := audit.SanitizeAuditLog(audit.AuditLogInput{
		Action:     strings.Repeat("a", 100),
		ActorID:    strings.Repeat("b", 50),
		TargetID:   strings.Repeat("c", 50),
		TargetType: strings.Repeat("d", 50),
		IP:         strings.Repeat("e", 50),
		UserAgent:  strings.Repeat("f", 600),
		Detail:     strings.Repeat("g", 2000),
	})

	if len(r.Action) > 64 {
		t.Errorf("Action length %d exceeds 64", len(r.Action))
	}
	if len(r.ActorID) > 36 {
		t.Errorf("ActorID length %d exceeds 36", len(r.ActorID))
	}
	if len(r.TargetID) > 36 {
		t.Errorf("TargetID length %d exceeds 36", len(r.TargetID))
	}
	if len(r.TargetType) > 32 {
		t.Errorf("TargetType length %d exceeds 32", len(r.TargetType))
	}
	if len(r.IP) > 45 {
		t.Errorf("IP length %d exceeds 45", len(r.IP))
	}
	if len(r.UserAgent) > 512 {
		t.Errorf("UserAgent length %d exceeds 512", len(r.UserAgent))
	}
}

func TestSanitizeAuditLog_UnicodeInput(t *testing.T) {
	r := audit.SanitizeAuditLog(audit.AuditLogInput{
		Action:     "用户.注册",
		ActorID:    "ユーザーID",
		TargetID:   "资源ID",
		TargetType: "类型",
		Result:     "success",
		Severity:   "info",
		IP:         "192.168.1.1",
		UserAgent:  "Mozilla/5.0 (兼容; 中文)",
		Detail:     `{"message":"你好世界"}`,
	})

	if r.Action != "unknown" {
		t.Errorf("Action should be unknown when all unicode chars are stripped, got %q", r.Action)
	}
	if r.TargetType != "unknown" {
		t.Errorf("TargetType should fallback to unknown, got %q", r.TargetType)
	}
	if r.IP != "192.168.1.1" {
		t.Errorf("IP should remain valid, got %q", r.IP)
	}
	if r.Detail != `{"message":"你好世界"}` {
		t.Errorf("Detail should preserve unicode, got %q", r.Detail)
	}
}

func TestSanitizeAuditLog_Result(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"success", "success", "success"},
		{"failure", "failure", "failure"},
		{"denied", "denied", "denied"},
		{"empty", "", "success"},
		{"invalid", "unknown", "success"},
		{"mixed case", "Success", "success"},
		{"sql injection", "'; DROP TABLE; --", "success"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{Result: tt.input})
			if r.Result != tt.want {
				t.Errorf("SanitizeAuditLog(result=%q).Result = %q, want %q", tt.input, r.Result, tt.want)
			}
		})
	}
}

func TestSanitizeAuditLog_Severity(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"info", "info", "info"},
		{"warn", "warn", "warn"},
		{"error", "error", "error"},
		{"critical", "critical", "critical"},
		{"empty", "", "info"},
		{"invalid", "unknown", "info"},
		{"mixed case", "Info", "info"},
		{"sql injection", "'; DROP TABLE; --", "info"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := audit.SanitizeAuditLog(audit.AuditLogInput{Severity: tt.input})
			if r.Severity != tt.want {
				t.Errorf("SanitizeAuditLog(severity=%q).Severity = %q, want %q", tt.input, r.Severity, tt.want)
			}
		})
	}
}

func TestSanitizeAuditLog_Consistency(t *testing.T) {
	// Same input should always produce same output
	in := audit.AuditLogInput{
		Action:    "user.login",
		ActorID:   "550e8400-e29b-41d4-a716-446655440000",
		TargetID:  "660e8400-e29b-41d4-a716-446655440001",
		TargetType: "user",
		Result:    "success",
		Severity:  "info",
		IP:        "10.0.0.1",
		UserAgent: "curl/7.68.0",
		Detail:    `{"action":"test"}`,
	}

	first := audit.SanitizeAuditLog(in)
	for i := 0; i < 10; i++ {
		result := audit.SanitizeAuditLog(in)
		if result != first {
			t.Errorf("iteration %d: result differs from first", i)
		}
	}
}
