package standard

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/roidmc/quotagate/plugin/captcha"
)

const (
	// TurnstileKey is the registry/provider name for Cloudflare Turnstile.
	TurnstileKey = "turnstile"
	// turnstileVersion is the implementation version, shared by the provider
	// instance and the factory so they can never drift apart.
	turnstileVersion = "1.0.0"
	// turnstileSiteverify is Cloudflare's token-verification endpoint.
	turnstileSiteverify = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
)

// siteverifyResponse is the JSON shape returned by Cloudflare Turnstile on
// token verification (Google reCAPTCHA uses the same envelope).
type siteverifyResponse struct {
	Success     bool     `json:"success"`
	ChallengeTS string   `json:"challenge_ts"`
	Hostname    string   `json:"hostname"`
	ErrorCodes  []string `json:"error-codes"`
}

// TurnstileProvider verifies Cloudflare Turnstile challenge tokens. It is a
// per-request instance: the secret comes from per-tenant config and is never
// cached in the singleton factory.
type TurnstileProvider struct {
	secret  string
	sitekey string
	client  *http.Client
}

var _ captcha.Provider = (*TurnstileProvider)(nil)
var _ captcha.Verifier = (*TurnstileProvider)(nil)
var _ captcha.PublicConfigProvider = (*TurnstileProvider)(nil)

func (p *TurnstileProvider) Name() string    { return TurnstileKey }
func (p *TurnstileProvider) Version() string { return turnstileVersion }
func (p *TurnstileProvider) Type() string    { return TurnstileKey }

// Verify sends the frontend-supplied token to Cloudflare's siteverify endpoint
// and returns true only when the challenge passed. Callers must do this BEFORE
// processing any submitted business data. remoteIP is optional; when non-empty
// it is forwarded as remoteip for Cloudflare's abuse heuristics.
func (p *TurnstileProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if token == "" {
		return false, fmt.Errorf("captcha/turnstile: empty token")
	}
	return postSiteverify(ctx, p.client, turnstileSiteverify, p.secret, token, remoteIP)
}

// PublicConfig returns the public sitekey the frontend needs to render the
// Turnstile widget. Not a secret.
func (p *TurnstileProvider) PublicConfig() map[string]string {
	return map[string]string{"sitekey": p.sitekey}
}

// postSiteverify performs the standard challenge-response verification shared by
// Turnstile and (later) reCAPTCHA: POST secret+response(+remoteip) as form data
// and parse the {success,...} answer.
func postSiteverify(ctx context.Context, client *http.Client, endpoint, secret, token, remoteIP string) (bool, error) {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	form := url.Values{}
	form.Set("secret", secret)
	form.Set("response", token)
	if remoteIP != "" {
		form.Set("remoteip", remoteIP)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return false, fmt.Errorf("captcha: build siteverify request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return false, fmt.Errorf("captcha: siteverify request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("captcha: siteverify unexpected status %d", resp.StatusCode)
	}

	var out siteverifyResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return false, fmt.Errorf("captcha: decode siteverify response: %w", err)
	}
	if !out.Success {
		return false, fmt.Errorf("captcha: challenge failed: %s", strings.Join(out.ErrorCodes, ","))
	}
	return true, nil
}

// TurnstileFactory builds per-tenant TurnstileProvider instances.
type TurnstileFactory struct{}

func (f *TurnstileFactory) Name() string    { return TurnstileKey }
func (f *TurnstileFactory) Version() string { return turnstileVersion }
func (f *TurnstileFactory) Type() string    { return TurnstileKey }

// Capabilities declares the capability interfaces TurnstileProvider satisfies,
// so Validate can confirm at boot that the produced instance actually
// implements them.
func (f *TurnstileFactory) Capabilities() []string {
	return []string{captcha.CapVerifier, captcha.CapPublicConfig}
}

// New builds a TurnstileProvider from per-tenant config. The secret (private
// key) is required at Verify time; we do not reject an empty config here so
// Validate can probe the factory with a zero-config instance at boot. The
// sitekey (public) is carried for parity but not used server-side.
func (f *TurnstileFactory) New(cfg captcha.ProviderConfig) (captcha.Provider, error) {
	secret := cfg.Extra["secret"]
	return &TurnstileProvider{
		secret:  secret,
		sitekey: cfg.Extra["sitekey"],
		client:  &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func init() {
	captcha.DefaultRegistry().Register(&TurnstileFactory{})
}
