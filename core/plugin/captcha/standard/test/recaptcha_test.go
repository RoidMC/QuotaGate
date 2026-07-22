// Package test holds tests for the standard captcha providers. recaptcha_test
// covers host/endpoint resolution (no network); live verification tests live
// under env-var gating elsewhere.
package test

import (
	"strings"
	"testing"

	"github.com/roidmc/quotagate/plugin/captcha"
	"github.com/roidmc/quotagate/plugin/captcha/standard"
)

// TestRecaptchaEndpointResolution pins the google.com vs recaptcha.net host
// selection. This is the accessibility fix for GFW/WAF-affected networks:
// www.google.com is unreachable there, so the mirror www.recaptcha.net must be
// used for BOTH the backend siteverify call and the frontend widget script.
func TestRecaptchaEndpointResolution(t *testing.T) {
	cases := []struct {
		name   string
		domain string
		want   string
	}{
		{"empty defaults to google", "", "https://www.google.com/recaptcha/api/siteverify"},
		{"explicit google.com", "google.com", "https://www.google.com/recaptcha/api/siteverify"},
		{"recaptcha.net mirror", "recaptcha.net", "https://www.recaptcha.net/recaptcha/api/siteverify"},
		{"recaptchanet alias", "recaptchanet", "https://www.recaptcha.net/recaptcha/api/siteverify"},
		{"google alias", "google", "https://www.google.com/recaptcha/api/siteverify"},
		{"explicit https override keeps host", "https://proxy.internal.example.com/recaptcha", "https://proxy.internal.example.com/recaptcha/api/siteverify"},
		{"explicit host override", "proxy.internal.example.com", "https://proxy.internal.example.com/recaptcha/api/siteverify"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := standard.RecaptchaEndpoint(c.domain); got != c.want {
				t.Fatalf("RecaptchaEndpoint(%q) = %q, want %q", c.domain, got, c.want)
			}
			if got := standard.RecaptchaScriptURL(c.domain); got != strings.Replace(c.want, "/api/siteverify", "/api.js", 1) {
				t.Fatalf("RecaptchaScriptURL(%q) = %q, want %q", c.domain, got, strings.Replace(c.want, "/api/siteverify", "/api.js", 1))
			}
		})
	}
}

// TestRecaptchaNewResolvesEndpoint confirms the factory threads domain into the
// provider's endpoint + script, so the same config drives both server and UI.
func TestRecaptchaNewResolvesEndpoint(t *testing.T) {
	p, err := (&standard.RecaptchaFactory{}).New(captcha.ProviderConfig{
		TenantID: "test",
		Name:     standard.RecaptchaKey,
		Extra:    map[string]string{"domain": "recaptcha.net", "sitekey": "x", "secret": "y"},
	})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	v, ok := p.(interface {
		PublicConfig() map[string]string
	})
	if !ok {
		t.Fatal("recaptcha provider does not expose PublicConfig")
	}
	cfg := v.PublicConfig()
	wantScript := "https://www.recaptcha.net/recaptcha/api.js"
	if cfg["script"] != wantScript {
		t.Fatalf("PublicConfig script = %q, want %q", cfg["script"], wantScript)
	}
}
