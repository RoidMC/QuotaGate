package audit_test

import (
	"testing"

	"github.com/roidmc/quotagate/internal/util/audit"
)

func TestSanitizeAuditLog(t *testing.T) {
	tests := []struct {
		name   string
		input  audit.AuditLogInput
		expect audit.AuditLogInput
	}{
		{
			name: "valid input",
			input: audit.AuditLogInput{
				Action:     "user.register",
				ActorID:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				TargetID:   "b2c3d4e5-f6a7-8901-bcde-f12345678901",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				RequestID:  "r1",
				IP:         "192.168.1.1",
				UserAgent:  "Mozilla/5.0",
				Message:    "user registered",
				Detail:     `{"email":"test@example.com"}`,
				Before:     `{"status":"pending"}`,
				After:      `{"status":"active"}`,
			},
			expect: audit.AuditLogInput{
				Action:     "user.register",
				ActorID:    "a1b2c3d4-e5f6-7890-abcd-ef1234567890",
				TargetID:   "b2c3d4e5-f6a7-8901-bcde-f12345678901",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				RequestID:  "r1",
				IP:         "192.168.1.1",
				UserAgent:  "Mozilla/5.0",
				Message:    "user registered",
				Detail:     `{"email":"test@example.com"}`,
				Before:     `{"status":"pending"}`,
				After:      `{"status":"active"}`,
			},
		},
		{
			name: "invalid IP",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				IP:         "not-an-ip",
				UserAgent:  "test",
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				IP:         "0.0.0.0",
				UserAgent:  "test",
			},
		},
		{
			name: "empty action",
			input: audit.AuditLogInput{
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
			},
			expect: audit.AuditLogInput{
				Action:     "unknown",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
			},
		},
		{
			name: "empty tenant",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "unknown",
			},
		},
		{
			name: "invalid result",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "invalid_result",
				Severity:   "info",
				TenantID:   "t1",
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
			},
		},
		{
			name: "invalid severity",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "invalid_severity",
				TenantID:   "t1",
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
			},
		},
		{
			name: "user agent with newlines (log injection)",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				UserAgent:  "Mozilla/5.0\nX-Injected: evil",
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				UserAgent:  "Mozilla/5.0 X-Injected: evil",
			},
		},
		{
			name: "message with newlines",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				Message:    "line1\nline2\rline3",
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				Message:    "line1 line2 line3",
			},
		},
		{
			name: "detail with newlines",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				Detail:     "{\"key\":\"value\nwith newline\"}",
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				Detail:     "{\"key\":\"value with newline\"}",
			},
		},
		{
			name: "before/after with newlines",
			input: audit.AuditLogInput{
				Action:     "user.update",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				Before:     "{\"status\":\"pending\n\"}",
				After:      "{\"status\":\"active\n\"}",
			},
			expect: audit.AuditLogInput{
				Action:     "user.update",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				Before:     "{\"status\":\"pending \"}",
				After:      "{\"status\":\"active \"}",
			},
		},
		{
			name: "IPv6 address",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				IP:         "::1",
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				IP:         "::1",
			},
		},
		{
			name: "long message truncation",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				Message:    string(make([]byte, 300)),
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				Message:    string(make([]byte, 256)),
			},
		},
		{
			name: "long user agent truncation",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				UserAgent:  string(make([]byte, 600)),
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
				UserAgent:  string(make([]byte, 512)),
			},
		},
		{
			name: "action with unsafe characters",
			input: audit.AuditLogInput{
				Action:     "user<register>",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
			},
			expect: audit.AuditLogInput{
				Action:     "userregister",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
			},
		},
		{
			name: "target type with unsafe characters",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user<script>",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
			},
			expect: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "userscript",
				Result:     "success",
				Severity:   "info",
				TenantID:   "t1",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := audit.SanitizeAuditLog(tt.input)
			if result.Action != tt.expect.Action {
				t.Errorf("Action = %q, want %q", result.Action, tt.expect.Action)
			}
			if result.ActorID != tt.expect.ActorID {
				t.Errorf("ActorID = %q, want %q", result.ActorID, tt.expect.ActorID)
			}
			if result.TargetID != tt.expect.TargetID {
				t.Errorf("TargetID = %q, want %q", result.TargetID, tt.expect.TargetID)
			}
			if result.TargetType != tt.expect.TargetType {
				t.Errorf("TargetType = %q, want %q", result.TargetType, tt.expect.TargetType)
			}
			if result.Result != tt.expect.Result {
				t.Errorf("Result = %q, want %q", result.Result, tt.expect.Result)
			}
			if result.Severity != tt.expect.Severity {
				t.Errorf("Severity = %q, want %q", result.Severity, tt.expect.Severity)
			}
			if result.TenantID != tt.expect.TenantID {
				t.Errorf("TenantID = %q, want %q", result.TenantID, tt.expect.TenantID)
			}
			if result.IP != tt.expect.IP {
				t.Errorf("IP = %q, want %q", result.IP, tt.expect.IP)
			}
			if result.UserAgent != tt.expect.UserAgent {
				t.Errorf("UserAgent = %q, want %q", result.UserAgent, tt.expect.UserAgent)
			}
			if result.Message != tt.expect.Message {
				t.Errorf("Message = %q, want %q", result.Message, tt.expect.Message)
			}
			if result.Detail != tt.expect.Detail {
				t.Errorf("Detail = %q, want %q", result.Detail, tt.expect.Detail)
			}
			if result.Before != tt.expect.Before {
				t.Errorf("Before = %q, want %q", result.Before, tt.expect.Before)
			}
			if result.After != tt.expect.After {
				t.Errorf("After = %q, want %q", result.After, tt.expect.After)
			}
		})
	}
}

func TestComputeSignature(t *testing.T) {
	tests := []struct {
		name   string
		input  audit.AuditLogInput
		secret string
	}{
		{
			name: "basic signature",
			input: audit.AuditLogInput{
				Action:     "user.login",
				ActorID:    "user-1",
				TargetID:   "user-1",
				TargetType: "user",
				Result:     "success",
				TenantID:   "t1",
				RequestID:  "r1",
				Message:    "user logged in",
			},
			secret: "test-secret",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sig1 := audit.ComputeSignature(tt.input, tt.secret)
			sig2 := audit.ComputeSignature(tt.input, tt.secret)
			if sig1 != sig2 {
				t.Errorf("signature not deterministic: %s != %s", sig1, sig2)
			}

			sig3 := audit.ComputeSignature(tt.input, "different-secret")
			if sig1 == sig3 {
				t.Errorf("signature should differ with different secret")
			}

			modified := tt.input
			modified.Message = "different message"
			sig4 := audit.ComputeSignature(modified, tt.secret)
			if sig1 == sig4 {
				t.Errorf("signature should differ with different input")
			}
		})
	}
}
