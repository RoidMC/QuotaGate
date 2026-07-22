package kexssrf_test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/roidmc/kex-utils/pkg/kexssrf"
)

func TestDefaultPolicyValidateURL(t *testing.T) {
	p := kexssrf.DefaultPolicy()

	tests := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"valid http", "http://example.com/webhook", false},
		{"valid https", "https://api.example.com/v1", false},
		{"missing scheme", "example.com/webhook", true},
		{"missing host", "http:///path", true},
		{"ftp scheme", "ftp://example.com/file", true},
		{"file scheme", "file:///etc/passwd", true},
		{"loopback IPv4", "http://127.0.0.1:8080", true},
		{"loopback IPv6", "http://[::1]:8080", true},
		{"private 10.x", "http://10.0.0.1/api", true},
		{"private 172.16", "http://172.16.0.1/api", true},
		{"private 192.168", "http://192.168.1.1/api", true},
		{"link-local 169.254", "http://169.254.169.254/latest/meta-data/", true},
		{"zero IP", "http://0.0.0.0/", true},
		{"public IP", "http://8.8.8.8/", false},
		{"public domain", "https://stripe.com/webhook", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := p.ValidateURL(tc.url)
			if tc.wantErr && err == nil {
				t.Errorf("ValidateURL(%q) expected error, got nil", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("ValidateURL(%q) unexpected error: %v", tc.url, err)
			}
		})
	}
}

func TestPolicyAllowLoopback(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	p.AllowLoopback = true

	if err := p.ValidateURL("http://127.0.0.1:8080"); err != nil {
		t.Errorf("expected loopback allowed, got: %v", err)
	}
	if err := p.ValidateURL("http://10.0.0.1/api"); err == nil {
		t.Error("expected private IP still blocked")
	}
}

func TestPolicyAllowPrivate(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	p.AllowPrivate = true

	if err := p.ValidateURL("http://192.168.1.1/api"); err != nil {
		t.Errorf("expected private allowed, got: %v", err)
	}
	if err := p.ValidateURL("http://127.0.0.1:8080"); err == nil {
		t.Error("expected loopback still blocked")
	}
}

func TestPolicyAllowLinkLocal(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	p.AllowLinkLocal = true

	if err := p.ValidateURL("http://169.254.169.254/"); err != nil {
		t.Errorf("expected link-local allowed, got: %v", err)
	}
}

func TestPolicyCustomSchemes(t *testing.T) {
	p := &kexssrf.Policy{
		AllowedSchemes: []string{"https"},
		MaxRedirects:   10,
	}

	if err := p.ValidateURL("https://example.com"); err != nil {
		t.Errorf("expected https allowed, got: %v", err)
	}
	if err := p.ValidateURL("http://example.com"); err == nil {
		t.Error("expected http blocked")
	}
}

func TestCheckRedirectMaxHops(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	p.MaxRedirects = 2
	check := p.CheckRedirect()

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	via := []*http.Request{{}, {}}

	if err := check(req, via); err != http.ErrUseLastResponse {
		t.Errorf("expected ErrUseLastResponse at max hops, got: %v", err)
	}
}

func TestCheckRedirectBlocksBadScheme(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	check := p.CheckRedirect()

	req, _ := http.NewRequest("GET", "file:///etc/passwd", nil)
	if err := check(req, nil); err != http.ErrUseLastResponse {
		t.Errorf("expected ErrUseLastResponse for file scheme, got: %v", err)
	}
}

func TestCheckRedirectBlocksPrivateIP(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	check := p.CheckRedirect()

	req, _ := http.NewRequest("GET", "http://10.0.0.1/", nil)
	if err := check(req, nil); err != http.ErrUseLastResponse {
		t.Errorf("expected ErrUseLastResponse for private IP, got: %v", err)
	}
}

func TestCheckRedirectAllowsPublic(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	check := p.CheckRedirect()

	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	if err := check(req, nil); err != nil {
		t.Errorf("unexpected error for public URL: %v", err)
	}
}

func TestCheckRedirectNilURL(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	check := p.CheckRedirect()

	req := &http.Request{URL: nil}
	if err := check(req, nil); err != http.ErrUseLastResponse {
		t.Errorf("expected ErrUseLastResponse for nil URL, got: %v", err)
	}
}

// --- DialContext tests ---

func TestDialContextBlocksLoopback(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	dial := p.DialContext()

	_, err := dial(context.Background(), "tcp", "127.0.0.1:80")
	if err == nil {
		t.Fatal("expected error for loopback IP")
	}
}

func TestDialContextBlocksPrivate(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	dial := p.DialContext()

	_, err := dial(context.Background(), "tcp", "10.0.0.1:80")
	if err == nil {
		t.Fatal("expected error for private IP")
	}
}

func TestDialContextBlocksLinkLocal(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	dial := p.DialContext()

	_, err := dial(context.Background(), "tcp", "169.254.169.254:80")
	if err == nil {
		t.Fatal("expected error for link-local IP")
	}
}

func TestDialContextBlocksZeroIP(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	dial := p.DialContext()

	_, err := dial(context.Background(), "tcp", "0.0.0.0:80")
	if err == nil {
		t.Fatal("expected error for 0.0.0.0")
	}
}

func TestDialContextBlocksUnspecifiedIPv6(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	dial := p.DialContext()

	_, err := dial(context.Background(), "tcp", "[::]:80")
	if err == nil {
		t.Fatal("expected error for ::")
	}
}

func TestDialContextAllowLoopback(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	p.AllowLoopback = true
	p.DialTimeout = 200 * time.Millisecond
	dial := p.DialContext()

	// Dial loopback �?should pass IP check but fail to connect (no service).
	conn, err := dial(context.Background(), "tcp", "127.0.0.1:1")
	if err != nil && conn == nil {
		// IP check passed (no "ssrf: blocked" error); connection refused is fine.
		// Just verify it's NOT the SSRF block error.
		if isSSRFError(err) {
			t.Fatalf("expected IP check to pass, got SSRF block: %v", err)
		}
	}
	if conn != nil {
		conn.Close()
	}
}

func TestDialContextInvalidAddress(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	dial := p.DialContext()

	// Missing port �?net.SplitHostPort should fail.
	_, err := dial(context.Background(), "tcp", "127.0.0.1")
	if err == nil {
		t.Fatal("expected error for invalid address (no port)")
	}
}

func TestDialContextUnresolvableHost(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	p.DialTimeout = 2 * time.Second
	dial := p.DialContext()

	// Use a domain that definitely doesn't resolve.
	_, err := dial(context.Background(), "tcp", "nonexistent.invalid:80")
	if err == nil {
		t.Fatal("expected error for unresolvable host")
	}
}

func TestNewHTTPClient(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	client := p.NewHTTPClient(5 * time.Second)

	if client == nil {
		t.Fatal("expected non-nil client")
	}
	if client.Timeout != 5*time.Second {
		t.Errorf("timeout = %v, want 5s", client.Timeout)
	}
	if client.Transport == nil {
		t.Error("expected non-nil transport")
	}
	if client.CheckRedirect == nil {
		t.Error("expected non-nil CheckRedirect")
	}
}

func TestNewHTTPTransport(t *testing.T) {
	p := kexssrf.DefaultPolicy()
	transport := p.NewHTTPTransport()

	if transport == nil {
		t.Fatal("expected non-nil transport")
	}
	if transport.DialContext == nil {
		t.Error("expected non-nil DialContext")
	}
	if transport.MaxIdleConns != 100 {
		t.Errorf("MaxIdleConns = %d, want 100", transport.MaxIdleConns)
	}
}

func TestPolicyDialTimeoutDefault(t *testing.T) {
	p := &kexssrf.Policy{} // zero value �?DialTimeout is 0, should default to 10s
	// We can't easily test the actual timeout without a real connection,
	// but we can verify DialContext doesn't panic with zero DialTimeout.
	dial := p.DialContext()
	if dial == nil {
		t.Fatal("expected non-nil DialContext")
	}
}

// isSSRFError returns true if the error is an SSRF block (not a connection error).
func isSSRFError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return contains(msg, "ssrf:") && contains(msg, "blocked")
}

func contains(s, substr string) bool {
	for i := 0; i+len(substr) <= len(s); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
