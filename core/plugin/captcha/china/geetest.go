package china

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/roidmc/quotagate/plugin/captcha"
)

const (
	// GeetestKey is the registry/provider name for GeeTest (极验). It is also the
	// provider Type, since this package ships exactly one GeeTest implementation.
	GeetestKey = "geetest"
	// geetestVersion is the implementation version, shared by both the provider
	// instance and the factory so they can never drift apart.
	geetestVersion = "1.0.0"
	// geetestV4Host is GeeTest's v4 validate host. v3 (api.geetest.com) is legacy;
	// modern credentials are v4, which uses HMAC-signed validation and is loaded
	// entirely by the frontend SDK (no server-side register call).
	geetestV4Host = "https://gcaptcha4.geetest.com"
	// geetestSDK is the SDK identifier sent to GeeTest's /validate endpoint so the
	// upstream can attribute the call. Derived from geetestVersion to keep a single
	// source of truth for the version number (no duplicated "1.0.0").
	geetestSDK = "quotagate-golang:v" + geetestVersion
)

// GeetestProvider verifies GeeTest (极验) v4 behavior captchas. It is a
// per-request instance: the captcha ID and private key come from per-tenant
// config and are never cached in the singleton factory.
//
// v4 model: the frontend loads the widget via GeeTest's gt4.js SDK using the
// public captcha_id (served by PublicConfig). The SDK performs the challenge
// fetch/load itself — the backend NEVER calls /register. The backend only
// validates the solved result via Verify, which is HMAC-authenticated and
// therefore independent of the caller's domain (localhost works fine).
type GeetestProvider struct {
	captchaID  string
	privateKey string
	host       string // override for tests; defaults to geetestV4Host
	client     *http.Client
}

var _ captcha.Provider = (*GeetestProvider)(nil)
var _ captcha.Verifier = (*GeetestProvider)(nil)
var _ captcha.PublicConfigProvider = (*GeetestProvider)(nil)

func (p *GeetestProvider) Name() string    { return GeetestKey }
func (p *GeetestProvider) Version() string { return geetestVersion }
func (p *GeetestProvider) Type() string    { return GeetestKey }

func (p *GeetestProvider) hostOrDefault() string {
	if p.host != "" {
		return p.host
	}
	return geetestV4Host
}

// PublicConfig returns the public captcha_id the frontend needs to initialize
// GeeTest's gt4.js SDK. This is NOT a secret and is safe to expose to the
// browser.
func (p *GeetestProvider) PublicConfig() map[string]string {
	return map[string]string{"captcha_id": p.captchaID}
}

// GeetestResult is the solved-captcha payload the frontend returns from the v4
// widget callback (the four fields gt4.js hands back on success).
type GeetestResult struct {
	LotNumber     string `json:"lot_number"`
	CaptchaOutput string `json:"captcha_output"`
	PassToken     string `json:"pass_token"`
	GenTime       string `json:"gen_time"`
}

// Verify implements captcha.Verifier. token is the JSON-encoded GeetestResult
// the frontend obtained from the widget; the backend signs lot_number with the
// private key and POSTs to GeeTest's validate endpoint. remoteIP is accepted for
// interface parity but v4 validation does not consume it. Returns true only
// when both status and result are "success".
func (p *GeetestProvider) Verify(ctx context.Context, token, remoteIP string) (bool, error) {
	if token == "" {
		return false, fmt.Errorf("captcha/geetest: empty token")
	}
	var r GeetestResult
	if err := json.Unmarshal([]byte(token), &r); err != nil {
		return false, fmt.Errorf("captcha/geetest: invalid token json: %w", err)
	}
	return p.Validate(ctx, r)
}

// Validate verifies a solved v4 challenge. It computes sign_token =
// HMAC-SHA256(privateKey, lot_number) and POSTs to GeeTest's validate endpoint.
// Returns true only when both status and result are "success".
func (p *GeetestProvider) Validate(ctx context.Context, r GeetestResult) (bool, error) {
	if r.LotNumber == "" || r.CaptchaOutput == "" || r.PassToken == "" || r.GenTime == "" {
		return false, fmt.Errorf("captcha/geetest: incomplete solved result")
	}
	signToken := hmacHex(p.privateKey, r.LotNumber)

	form := url.Values{}
	form.Set("captcha_id", p.captchaID)
	form.Set("lot_number", r.LotNumber)
	form.Set("pass_token", r.PassToken)
	form.Set("gen_time", r.GenTime)
	form.Set("captcha_output", r.CaptchaOutput)
	form.Set("sign_token", signToken)
	form.Set("sdk", geetestSDK)

	raw, status, err := p.post(ctx, p.hostOrDefault()+"/validate", form)
	if err != nil {
		return false, fmt.Errorf("captcha/geetest: validate: %w", err)
	}
	if status != http.StatusOK {
		// Upstream rejected (network-level 403 from an allowlist/WAF is not
		// expected on validate, which is HMAC-authenticated; still degrade
		// gracefully instead of crashing the caller).
		return false, nil
	}
	var resp struct {
		Status string `json:"status"`
		Result string `json:"result"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return false, fmt.Errorf("captcha/geetest: decode validate: %w", err)
	}
	return resp.Status == "success" && resp.Result == "success", nil
}

// --- http + helpers ---

func (p *GeetestProvider) post(ctx context.Context, u string, form url.Values) ([]byte, int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, u, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return p.do(req)
}

func (p *GeetestProvider) do(req *http.Request) ([]byte, int, error) {
	client := p.client
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	return body, resp.StatusCode, err
}

func hmacHex(key, msg string) string {
	mac := hmac.New(sha256.New, []byte(key))
	mac.Write([]byte(msg))
	return hex.EncodeToString(mac.Sum(nil))
}

// GeetestFactory builds per-tenant GeetestProvider instances.
type GeetestFactory struct{}

func (f *GeetestFactory) Name() string    { return GeetestKey }
func (f *GeetestFactory) Version() string { return geetestVersion }
func (f *GeetestFactory) Type() string    { return GeetestKey }

// Capabilities declares the capability interfaces GeetestProvider satisfies, so
// Validate can confirm at boot that the produced instance actually implements
// them.
func (f *GeetestFactory) Capabilities() []string {
	return []string{captcha.CapVerifier, captcha.CapPublicConfig}
}

// New builds a GeetestProvider from per-tenant config. captcha_id (the public
// "验证ID") and private_key (the "Key") are required at Verify time; we do not
// reject an empty config here so Validate can probe the factory with a
// zero-config instance at boot.
func (f *GeetestFactory) New(cfg captcha.ProviderConfig) (captcha.Provider, error) {
	id := cfg.Extra["captcha_id"]
	key := cfg.Extra["private_key"]
	return &GeetestProvider{
		captchaID:  id,
		privateKey: key,
		client:     &http.Client{Timeout: 10 * time.Second},
	}, nil
}

func init() {
	captcha.DefaultRegistry().Register(&GeetestFactory{})
}
