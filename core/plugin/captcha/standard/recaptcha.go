package standard

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/roidmc/quotagate/plugin/captcha"
)

const (
	// RecaptchaKey is the registry/provider name for Google reCAPTCHA.
	RecaptchaKey = "recaptcha"
	// recaptchaVersion is the implementation version, shared by the provider
	// instance and the factory so they can never drift apart.
	recaptchaVersion = "1.0.0"
	// recaptchaHostGoogle is the global reCAPTCHA host.
	recaptchaHostGoogle = "www.google.com"
	// recaptchaHostNet is Google's mirror for regions where www.google.com is
	// unreachable (mainland China, networks behind a WAF/GFW). It serves the
	// identical siteverify + widget API on a different domain, so switching the
	// host is enough to make reCAPTCHA work behind such networks.
	recaptchaHostNet = "www.recaptcha.net"
	// recaptchaSiteverifyPath and recaptchaScriptPath diverge after /recaptcha/:
	// siteverify is a sub-path (api/siteverify) while the widget script is a
	// single file (api.js). Both exist on both hosts.
	recaptchaSiteverifyPath = "/recaptcha/api/siteverify"
	recaptchaScriptPath     = "/recaptcha/api.js"
)

// recaptchaHost resolves the reCAPTCHA host for a domain hint:
//   - "" / "google" / "google.com"        -> www.google.com (global, default)
//   - "recaptcha.net" / "recaptchanet"    -> www.recaptcha.net (GFW/WAF-safe mirror)
//   - any other value is treated as an explicit host or full URL override
func recaptchaHost(domain string) string {
	switch domain {
	case "", "google", "google.com", recaptchaHostGoogle:
		return recaptchaHostGoogle
	case "recaptcha.net", "recaptchanet", recaptchaHostNet:
		return recaptchaHostNet
	default:
		// Allow an explicit full URL (e.g. an internal proxy). Keep only the
		// host so we can rebuild scheme + path consistently.
		if strings.HasPrefix(domain, "http://") || strings.HasPrefix(domain, "https://") {
			u := strings.TrimPrefix(strings.TrimPrefix(domain, "https://"), "http://")
			if i := strings.IndexByte(u, '/'); i >= 0 {
				u = u[:i]
			}
			return u
		}
		return domain
	}
}

// RecaptchaEndpoint returns the siteverify URL for a domain hint. The default
// (google.com) works globally; pass "recaptcha.net" for GFW/WAF-affected networks.
func RecaptchaEndpoint(domain string) string {
	return "https://" + recaptchaHost(domain) + recaptchaSiteverifyPath
}

// RecaptchaScriptURL returns the frontend widget script URL for a domain hint,
// mirroring RecaptchaEndpoint's host selection (recaptcha.net for blocked regions).
func RecaptchaScriptURL(domain string) string {
	return "https://" + recaptchaHost(domain) + recaptchaScriptPath
}

// RecaptchaProvider verifies Google reCAPTCHA v2 challenge tokens. It is a
// per-request instance: the secret comes from per-tenant config and is never
// cached in the singleton factory.
type RecaptchaProvider struct {
	secret    string
	sitekey   string
	endpoint  string
	scriptURL string
	client    *http.Client
}

var _ captcha.Provider = (*RecaptchaProvider)(nil)
var _ captcha.Verifier = (*RecaptchaProvider)(nil)
var _ captcha.PublicConfigProvider = (*RecaptchaProvider)(nil)

func (p *RecaptchaProvider) Name() string    { return RecaptchaKey }
func (p *RecaptchaProvider) Version() string { return recaptchaVersion }
func (p *RecaptchaProvider) Type() string    { return RecaptchaKey }

// Verify sends the frontend-supplied token to Google's siteverify endpoint and
// returns true only when the challenge passed. Callers must do this BEFORE
// processing any submitted business data. remoteIP is optional; when non-empty
// it is forwarded as remoteip for Google's abuse heuristics.
func (p *RecaptchaProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if token == "" {
		return false, fmt.Errorf("captcha/recaptcha: empty token")
	}
	return postSiteverify(ctx, p.client, p.endpoint, p.secret, token, remoteIP)
}

// PublicConfig returns the public sitekey (and widget script URL) the frontend
// needs to render the reCAPTCHA widget. Neither is a secret. script points at
// the recaptcha.net mirror when configured for GFW/WAF-affected networks.
func (p *RecaptchaProvider) PublicConfig() map[string]string {
	return map[string]string{"sitekey": p.sitekey, "script": p.scriptURL}
}

// RecaptchaFactory builds per-tenant RecaptchaProvider instances.
type RecaptchaFactory struct{}

func (f *RecaptchaFactory) Name() string    { return RecaptchaKey }
func (f *RecaptchaFactory) Version() string { return recaptchaVersion }
func (f *RecaptchaFactory) Type() string    { return RecaptchaKey }

// Capabilities declares the capability interfaces RecaptchaProvider satisfies,
// so Validate can confirm at boot that the produced instance actually
// implements them.
func (f *RecaptchaFactory) Capabilities() []string {
	return []string{captcha.CapVerifier, captcha.CapPublicConfig}
}

// New builds a RecaptchaProvider from per-tenant config. The secret (private
// key) is required at Verify time; we do not reject an empty config here so
// Validate can probe the factory with a zero-config instance at boot. The
// sitekey (public) and domain (host selection) are carried for parity.
//
// domain (cfg.Extra["domain"]) selects the reCAPTCHA host: "" / "google" ->
// www.google.com (global), "recaptcha.net" -> www.recaptcha.net (mirror for
// GFW/WAF-affected networks). It is forwarded by CaptchaService from
// config.CaptchaConfig.Recaptcha.Domain.
func (f *RecaptchaFactory) New(cfg captcha.ProviderConfig) (captcha.Provider, error) {
	domain := cfg.Extra["domain"]
	return &RecaptchaProvider{
		secret:    cfg.Extra["secret"],
		sitekey:   cfg.Extra["sitekey"],
		endpoint:  RecaptchaEndpoint(domain),
		scriptURL: RecaptchaScriptURL(domain),
		client:    &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func init() {
	captcha.DefaultRegistry().Register(&RecaptchaFactory{})
}
