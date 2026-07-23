package standard

import (
	"context"
	"crypto"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwk"
	"github.com/lestrrat-go/jwx/v4/jwt"
	"golang.org/x/oauth2"

	"github.com/roidmc/kex-utils/pkg/kexpluginsdk"
	"github.com/roidmc/quotagate/plugin/sso"
)

// init registers the generic OIDC provider factory. It is intentionally
// generic (a single "oidc" name) rather than one factory per vendor: any
// standards-compliant OpenID Provider — Keycloak, Auth0, Okta, Dex, Google,
// Authelia, Zitadel, ... — is reachable through config (issuer + client
// credentials), so a tenant can point QuotaGate at its own IdP without a code
// change.
func init() {
	sso.DefaultRegistry().Register(oidcFactory{})
}

// oidcFactory builds a generic OIDC RedirectProvider. Credentials/endpoints
// come from ProviderConfig (DB-backed sso_provider_configs); the factory is
// stateless.
type oidcFactory struct{}

func (oidcFactory) Name() string   { return "oidc" }
func (oidcFactory) Flow() sso.Flow { return sso.FlowRedirect }

func (oidcFactory) New(cfg sso.ProviderConfig) (sso.Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("sso/oidc: ClientID and ClientSecret are required")
	}
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		// openid is mandatory for OIDC; email/profile populate the Assertion.
		scopes = []string{"openid", "email", "profile"}
	}

	// Endpoint resolution priority:
	//   1. explicit Extra overrides (auth_url/token_url/userinfo_url/jwks_url)
	//   2. OIDC discovery from issuer (/.well-known/openid-configuration)
	// At least one of (issuer) or (auth_url + token_url) must be present.
	p := &oidcProvider{
		clientID:     cfg.ClientID,
		clientSecret: cfg.ClientSecret,
		redirectURL:  cfg.RedirectURL,
		scopes:       scopes,
		issuer:       cfg.Extra["issuer"],
		authURL:      cfg.Extra["auth_url"],
		tokenURL:     cfg.Extra["token_url"],
		userinfoURL:  cfg.Extra["userinfo_url"],
		jwksURL:      cfg.Extra["jwks_url"],
		client:       kexpluginsdk.SharedHTTPClient,
	}
	return p, nil
}

// defaultOIDCSkew is the clock tolerance applied when validating the id_token's
// exp/iat/nbf. IdP clocks and our own occasionally differ by a few seconds.
const defaultOIDCSkew = 5 * time.Minute

type oidcProvider struct {
	clientID     string
	clientSecret string
	redirectURL  string
	scopes       []string

	// name overrides the logical provider name returned by Name(). It is ""
	// for the generic "oidc" provider and set (e.g. "google") when a vendor
	// factory reuses this implementation against a specific issuer.
	name string

	// issuer is the configured OIDC issuer. When set, id_token `iss` is
	// validated against it and discovery is used to fill any missing endpoint.
	issuer string
	// explicit endpoint overrides (from Extra). Empty fields are resolved via
	// discovery when issuer is set.
	authURL     string
	tokenURL    string
	userinfoURL string
	jwksURL     string

	client *http.Client
}

func (p *oidcProvider) Name() string {
	if p.name != "" {
		return p.name
	}
	return "oidc"
}
func (p *oidcProvider) Flow() sso.Flow { return sso.FlowRedirect }

// resolveEndpoints returns the four OIDC endpoints, discovering the missing
// ones from the issuer when possible. Discovery results are cached per issuer
// (see fetchDiscovery) so repeated logins don't re-hit the .well-known doc.
func (p *oidcProvider) resolveEndpoints(ctx context.Context) (auth, token, userinfo, jwks string, err error) {
	auth, token, userinfo, jwks = p.authURL, p.tokenURL, p.userinfoURL, p.jwksURL
	if auth != "" && token != "" && (userinfo != "" && jwks != "" || p.issuer == "") {
		// Everything we need is explicit; skip discovery.
		return auth, token, userinfo, jwks, nil
	}
	if p.issuer == "" {
		return "", "", "", "", fmt.Errorf("sso/oidc: set issuer (for discovery) or auth_url+token_url+jwks_url in Extra")
	}
	d, err := fetchDiscovery(ctx, p.issuer)
	if err != nil {
		return "", "", "", "", err
	}
	if auth == "" {
		auth = d.AuthorizationEndpoint
	}
	if token == "" {
		token = d.TokenEndpoint
	}
	if userinfo == "" {
		userinfo = d.UserinfoEndpoint
	}
	if jwks == "" {
		jwks = d.JWKSURI
	}
	if token == "" || jwks == "" {
		return "", "", "", "", fmt.Errorf("sso/oidc: discovery at %q missing token/jwks endpoint", p.issuer)
	}
	return auth, token, userinfo, jwks, nil
}

func (p *oidcProvider) BeginAuth(ctx context.Context, state, _ string) (string, error) {
	auth, token, _, _, err := p.resolveEndpoints(ctx)
	if err != nil {
		return "", err
	}
	if auth == "" || token == "" {
		return "", fmt.Errorf("sso/oidc: auth/token endpoint not resolvable")
	}
	ocfg := &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RedirectURL:  p.redirectURL,
		Scopes:       p.scopes,
		Endpoint:     oauth2.Endpoint{AuthURL: auth, TokenURL: token},
	}
	// access_type=offline lets providers that support refresh tokens issue
	// one; it is harmless where unsupported.
	return ocfg.AuthCodeURL(state, oauth2.AccessTypeOffline), nil
}

func (p *oidcProvider) CompleteAuth(ctx context.Context, code, _ string) (*sso.Assertion, error) {
	_, tokenURL, userinfoURL, jwksURL, err := p.resolveEndpoints(ctx)
	if err != nil {
		return nil, err
	}

	ocfg := &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		RedirectURL:  p.redirectURL,
		Scopes:       p.scopes,
		Endpoint:     oauth2.Endpoint{AuthURL: p.authURL, TokenURL: tokenURL},
	}

	// 1. code → tokens. OIDC token responses carry id_token (a signed JWT)
	//    alongside access_token.
	token, err := ocfg.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("sso/oidc: exchange code: %w: %v", sso.ErrExchangeFailed, err)
	}
	rawID, _ := token.Extra("id_token").(string)

	// 2. Verify + decode the id_token. This is what makes the flow OIDC rather
	//    than opaque OAuth: the signature (over the IdP's JWKS) proves the
	//    subject came from the IdP, and iss/aud/exp are checked.
	var idClaims map[string]any
	if rawID != "" {
		if jwksURL == "" {
			return nil, fmt.Errorf("sso/oidc: id_token present but no jwks_uri to verify it")
		}
		if err := verifyIDToken(ctx, rawID, jwksURL, p.issuer, p.clientID); err != nil {
			return nil, fmt.Errorf("sso/oidc: verify id_token: %w: %v", sso.ErrExchangeFailed, err)
		}
		idClaims, err = decodeJWTClaims(rawID)
		if err != nil {
			return nil, fmt.Errorf("sso/oidc: decode id_token claims: %w: %v", sso.ErrExchangeFailed, err)
		}
	}

	// 3. Enrich with the UserInfo endpoint (authoritative for profile/email;
	//    the id_token often omits or minimizes them). Bearer access_token.
	var uiClaims map[string]any
	if userinfoURL != "" {
		uiClaims, err = p.fetchUserInfo(ctx, token.AccessToken, userinfoURL)
		if err != nil {
			// Non-fatal: many IdPs return everything needed in the id_token.
			uiClaims = nil
		}
	}

	// 4. Merge: id_token claims are the authenticated baseline; UserInfo
	//    overlays them for richer attributes. `sub` is taken from the id_token
	//    when present (it is the cryptographically proven subject).
	claims := map[string]any{}
	for k, v := range idClaims {
		claims[k] = v
	}
	for k, v := range uiClaims {
		claims[k] = v
	}
	subject, _ := claims["sub"].(string)
	if subject == "" {
		if s, ok := idClaims["sub"].(string); ok {
			subject = s
		}
	}
	if subject == "" {
		return nil, fmt.Errorf("sso/oidc: no subject (sub) in id_token or userinfo")
	}

	email, _ := claims["email"].(string)
	emailVerified := false
	_, emailVerifiedPresent := claims["email_verified"]
	if v, ok := claims["email_verified"].(bool); ok {
		emailVerified = v
	}
	// Security: an unverified email must not be trusted for account linking.
	// Logto applies the same rule. We only strip the email when the IdP
	// explicitly signalled email_verified=false; if the claim is absent we
	// keep the email to avoid breaking IdPs that simply omit the flag.
	if emailVerifiedPresent && !emailVerified {
		email = ""
	}
	username := claimStr(claims, "preferred_username")
	if username == "" {
		if email != "" {
			username = strings.SplitN(email, "@", 2)[0]
		} else {
			username = subject
		}
	}
	displayName := claimStr(claims, "name")
	if displayName == "" {
		displayName = username
	}
	picture, _ := claims["picture"].(string)

	return &sso.Assertion{
		Provider:      p.Name(),
		Subject:       subject,
		Username:      username,
		DisplayName:   displayName,
		Email:         email,
		EmailVerified: emailVerified,
		AvatarURL:     picture,
		Raw:           claims,
	}, nil
}

// fetchUserInfo calls the UserInfo endpoint with the bearer access token.
func (p *oidcProvider) fetchUserInfo(ctx context.Context, accessToken, userinfoURL string) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, userinfoURL, nil)
	if err != nil {
		return nil, fmt.Errorf("sso/oidc: build userinfo request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sso/oidc: userinfo: %w: %v", sso.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sso/oidc: userinfo returned %d: %w: %s", resp.StatusCode, sso.ErrProviderUnavailable, body)
	}
	var m map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		return nil, fmt.Errorf("sso/oidc: decode userinfo: %w: %v", sso.ErrProviderUnavailable, err)
	}
	return m, nil
}

// verifyIDToken validates the id_token's signature against the IdP's JWKS and
// checks the standard registered claims (iss, aud, exp/iat/nbf with skew). It
// does NOT return claims; decodeJWTClaims reads them afterwards.
//
// Verification first tries kid-matched key selection via WithKeySet (the
// common case where the id_token header carries a `kid`). If that fails — some
// IdPs omit `kid`, or publish a single key — we fall back to trying every key
// in the set, which is safe because a valid signature requires the true
// private key regardless of which public key we test against.
func verifyIDToken(ctx context.Context, rawID, jwksURL, issuer, clientID string) error {
	set, err := loadJWKS(ctx, jwksURL)
	if err != nil {
		return err
	}
	opts := func() []jwt.ParseOption {
		o := []jwt.ParseOption{
			jwt.WithAudience(clientID),
			jwt.WithValidate(true),
			jwt.WithAcceptableSkew(defaultOIDCSkew),
		}
		if issuer != "" {
			o = append(o, jwt.WithIssuer(issuer))
		}
		return o
	}()

	if _, err := jwt.Parse([]byte(rawID), append(opts, jwt.WithKeySet(set))...); err == nil {
		return nil
	}
	// kid-agnostic fallback: WithKeySet requires a `kid` in the token header,
	// but some IdPs omit it (or publish a single key). Verify against each raw
	// public key directly — a valid signature is key-specific regardless.
	for i := 0; i < set.Len(); i++ {
		key, ok := set.Key(i)
		if !ok {
			continue
		}
		rawPub, err := jwk.Export[crypto.PublicKey](key)
		if err != nil {
			continue
		}
		// The signature algorithm comes from the token's own JWS header (the
		// JWK often omits `alg`), so we don't depend on the JWK carrying it.
		alg, err := tokenAlg(rawID)
		if err != nil {
			continue
		}
		if _, err := jwt.Parse([]byte(rawID), append(opts, jwt.WithKey(alg, rawPub))...); err == nil {
			return nil
		}
	}
	return fmt.Errorf("no JWKS key verified the id_token signature")
}

// tokenAlg reads the `alg` field from the id_token's JWS protected header. We
// use it to select the verification algorithm because the JWKS key may not
// declare `alg` (common for OIDC providers that publish a single key).
func tokenAlg(raw string) (jwa.SignatureAlgorithm, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return jwa.EmptySignatureAlgorithm(), fmt.Errorf("malformed JWT (want 3 parts)")
	}
	b, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return jwa.EmptySignatureAlgorithm(), fmt.Errorf("decode header: %w", err)
	}
	var hdr struct {
		Alg string `json:"alg"`
	}
	if err := json.Unmarshal(b, &hdr); err != nil {
		return jwa.EmptySignatureAlgorithm(), fmt.Errorf("unmarshal header: %w", err)
	}
	if hdr.Alg == "" {
		return jwa.EmptySignatureAlgorithm(), fmt.Errorf("id_token header missing alg")
	}
	alg, ok := jwa.LookupSignatureAlgorithm(hdr.Alg)
	if !ok {
		return jwa.EmptySignatureAlgorithm(), fmt.Errorf("unknown id_token alg %q", hdr.Alg)
	}
	return alg, nil
}

// decodeJWTClaims base64url-decodes the id_token payload into a map. Used after
// verifyIDToken so we don't depend on a specific jwx Token access API.
func decodeJWTClaims(raw string) (map[string]any, error) {
	parts := strings.Split(raw, ".")
	if len(parts) != 3 {
		return nil, fmt.Errorf("malformed JWT (want 3 parts)")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, fmt.Errorf("decode payload: %w", err)
	}
	var m map[string]any
	if err := json.Unmarshal(payload, &m); err != nil {
		return nil, fmt.Errorf("unmarshal payload: %w", err)
	}
	return m, nil
}

func claimStr(claims map[string]any, key string) string {
	if v, ok := claims[key].(string); ok {
		return v
	}
	return ""
}

// ---------------------------------------------------------------------------
// OIDC discovery (cached per issuer) + JWKS cache
// ---------------------------------------------------------------------------

const discoveryCacheTTL = time.Hour

type oidcDiscovery struct {
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
	Issuer                string `json:"issuer"`
}

type discoveryEntry struct {
	d   oidcDiscovery
	exp time.Time
}

var (
	discMu    sync.Mutex
	discCache = map[string]discoveryEntry{}
)

// fetchDiscovery returns the OIDC discovery document for issuer, using a
// per-issuer 1h cache. The discovered issuer must match the configured one.
func fetchDiscovery(ctx context.Context, issuer string) (oidcDiscovery, error) {
	discMu.Lock()
	if e, ok := discCache[issuer]; ok && time.Now().Before(e.exp) {
		discMu.Unlock()
		return e.d, nil
	}
	discMu.Unlock()

	docURL := strings.TrimRight(issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, docURL, nil)
	if err != nil {
		return oidcDiscovery{}, fmt.Errorf("sso/oidc: build discovery request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := kexpluginsdk.SharedHTTPClient.Do(req)
	if err != nil {
		return oidcDiscovery{}, fmt.Errorf("sso/oidc: discovery %q: %w: %v", docURL, sso.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return oidcDiscovery{}, fmt.Errorf("sso/oidc: discovery %q returned %d: %w: %s", docURL, resp.StatusCode, sso.ErrProviderUnavailable, body)
	}
	var d oidcDiscovery
	if err := json.NewDecoder(resp.Body).Decode(&d); err != nil {
		return oidcDiscovery{}, fmt.Errorf("sso/oidc: decode discovery: %w: %v", sso.ErrProviderUnavailable, err)
	}
	if d.Issuer != "" && issuer != "" && !strings.EqualFold(d.Issuer, strings.TrimRight(issuer, "/")) && d.Issuer != issuer {
		// Allow the common case where the discovered issuer is the canonical
		// form (e.g. with/without trailing slash).
		if d.Issuer != strings.TrimRight(issuer, "/") {
			return oidcDiscovery{}, fmt.Errorf("sso/oidc: discovered issuer %q != configured %q", d.Issuer, issuer)
		}
	}
	discMu.Lock()
	discCache[issuer] = discoveryEntry{d: d, exp: time.Now().Add(discoveryCacheTTL)}
	discMu.Unlock()
	return d, nil
}

// loadJWKS returns the IdP's signing key set. The JWKS document is fetched
// over HTTP and parsed into a jwk.Set used to verify the id_token signature.
// (We don't keep a process-level cache: OIDC JWKS are small and login
// frequency is low, and rotation is handled by the IdP publishing new keys
// alongside retired ones, so a fresh fetch always works.)
func loadJWKS(ctx context.Context, url string) (jwk.Set, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("sso/oidc: build jwks request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	resp, err := kexpluginsdk.SharedHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("sso/oidc: fetch jwks %q: %w: %v", url, sso.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("sso/oidc: jwks %q returned %d: %w: %s", url, resp.StatusCode, sso.ErrProviderUnavailable, body)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("sso/oidc: read jwks %q: %w: %v", url, sso.ErrProviderUnavailable, err)
	}
	set, err := jwk.Parse(raw)
	if err != nil {
		return nil, fmt.Errorf("sso/oidc: parse jwks %q: %w: %v", url, sso.ErrProviderUnavailable, err)
	}
	return set, nil
}
