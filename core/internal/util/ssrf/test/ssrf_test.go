package ssrf_test

import (
	"net/http"
	"testing"

	"github.com/roidmc/quotagate/internal/util/ssrf"
)

func TestDefaultPolicyValidateURL(t *testing.T) {
	p := ssrf.DefaultPolicy()

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
	p := ssrf.DefaultPolicy()
	p.AllowLoopback = true

	if err := p.ValidateURL("http://127.0.0.1:8080"); err != nil {
		t.Errorf("expected loopback allowed, got: %v", err)
	}
	if err := p.ValidateURL("http://10.0.0.1/api"); err == nil {
		t.Error("expected private IP still blocked")
	}
}

func TestPolicyAllowPrivate(t *testing.T) {
	p := ssrf.DefaultPolicy()
	p.AllowPrivate = true

	if err := p.ValidateURL("http://192.168.1.1/api"); err != nil {
		t.Errorf("expected private allowed, got: %v", err)
	}
	if err := p.ValidateURL("http://127.0.0.1:8080"); err == nil {
		t.Error("expected loopback still blocked")
	}
}

func TestPolicyAllowLinkLocal(t *testing.T) {
	p := ssrf.DefaultPolicy()
	p.AllowLinkLocal = true

	if err := p.ValidateURL("http://169.254.169.254/"); err != nil {
		t.Errorf("expected link-local allowed, got: %v", err)
	}
}

func TestPolicyCustomSchemes(t *testing.T) {
	p := &ssrf.Policy{
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
	p := ssrf.DefaultPolicy()
	p.MaxRedirects = 2
	check := p.CheckRedirect()

	req, _ := http.NewRequest("GET", "http://example.com", nil)
	via := []*http.Request{{}, {}}

	if err := check(req, via); err != http.ErrUseLastResponse {
		t.Errorf("expected ErrUseLastResponse at max hops, got: %v", err)
	}
}

func TestCheckRedirectBlocksBadScheme(t *testing.T) {
	p := ssrf.DefaultPolicy()
	check := p.CheckRedirect()

	req, _ := http.NewRequest("GET", "file:///etc/passwd", nil)
	if err := check(req, nil); err != http.ErrUseLastResponse {
		t.Errorf("expected ErrUseLastResponse for file scheme, got: %v", err)
	}
}

func TestCheckRedirectBlocksPrivateIP(t *testing.T) {
	p := ssrf.DefaultPolicy()
	check := p.CheckRedirect()

	req, _ := http.NewRequest("GET", "http://10.0.0.1/", nil)
	if err := check(req, nil); err != http.ErrUseLastResponse {
		t.Errorf("expected ErrUseLastResponse for private IP, got: %v", err)
	}
}

func TestCheckRedirectAllowsPublic(t *testing.T) {
	p := ssrf.DefaultPolicy()
	check := p.CheckRedirect()

	req, _ := http.NewRequest("GET", "http://example.com/", nil)
	if err := check(req, nil); err != nil {
		t.Errorf("unexpected error for public URL: %v", err)
	}
}

func TestCheckRedirectNilURL(t *testing.T) {
	p := ssrf.DefaultPolicy()
	check := p.CheckRedirect()

	req := &http.Request{URL: nil}
	if err := check(req, nil); err != http.ErrUseLastResponse {
		t.Errorf("expected ErrUseLastResponse for nil URL, got: %v", err)
	}
}
