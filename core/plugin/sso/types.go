// Package sso defines the third-party Identity Provider (IdP) plugin system
// for QuotaGate. It hosts all federated identity flows where an external
// provider asserts a user's identity, covering both browser-redirect OAuth
// (Google/GitHub/WeChat-web/DingTalk OAuth) and third-party scan-confirm QR
// flows (WeChat MiniProgram / WeCom scan / DingTalk scan).
//
// # Compilation vs runtime
//
// All provider implementations are compiled into the binary unconditionally
// (no per-provider build tags). Each provider registers its factory in
// init() so the registry knows it exists at compile time. Whether a provider
// is actually *enabled* for a given tenant is a runtime decision: the tenant's
// SSO configuration row in the database carries the client_id / client_secret
// and the link mode. A request instantiates the provider with that config
// just-in-time; providers are NOT long-lived singletons holding credentials.
//
// # Flow split
//
// The base Provider interface only exposes identity metadata (Name/Flow).
// The two actual flows are modelled as optional capability interfaces,
// RedirectProvider and QRProvider, because their method shapes are
// fundamentally incompatible: redirect is a synchronous BeginAuth→CompleteAuth
// cycle driven by browser navigation, while QR is an async Generate→Poll
// cycle driven by a mobile app and a polling web client.
//
// Both flows converge on the same Assertion type, which is consumed by the
// internal AccountLinker to resolve a local user (see LinkMode).
//
// # Account linking
//
// When a provider returns an Assertion, the AccountLinker looks up
// (tenant_id, provider, subject) in user_identities. What happens on a miss
// depends on the tenant's LinkMode for that provider:
//   - LinkModeCreate: create a new local user and bind the identity to it.
//   - LinkModeBindOnly: reject the login; the user must first sign in with a
//     local credential and then go through a binding flow to attach the
//     third-party identity. This is the "allow SSO login, disallow
//     auto-registration" mode.
package sso

import (
	"context"
	"errors"
)

// Sentinel errors for SSO provider failures. These mirror the style of
// plugin/identity's exported sentinel errors (ErrQRCodeNotFound etc.) so
// callers can branch with errors.Is instead of matching error strings.
//
// Two families:
//
//   - Ticket / state lifecycle (used by QRProvider and the OAuth state
//     helpers): ErrTicketNotFound, ErrTicketExpired, ErrTicketConflict,
//     ErrTicketAlreadyScanned, ErrStateNotFound.
//   - External IdP interaction (used by RedirectProvider.CompleteAuth and
//     QRProvider.ResolveExchangeCode): ErrExchangeFailed, ErrProviderUnavailable.
//
// Providers wrap underlying detail with fmt.Errorf("...: %w", sentinel) so
// the sentinel stays identifiable while the wrapped message carries context
// (HTTP status, response body, network error).
var (
	// ErrTicketNotFound means the QR ticket does not exist in the store or
	// has already been consumed/evicted. Returned by QRProvider.Poll and
	// ResolveExchangeCode.
	ErrTicketNotFound = errors.New("sso: ticket not found or expired")

	// ErrTicketExpired means the ticket exists but its TTL has elapsed.
	// Distinct from ErrTicketNotFound so callers can differentiate
	// "re-issue" vs "unknown ticket" UX if needed.
	ErrTicketExpired = errors.New("sso: ticket expired")

	// ErrTicketConflict means the ticket is in a state that disallows the
	// requested transition (e.g. ResolveExchangeCode on an already-confirmed
	// ticket). Equivalent to identity.ErrQRCodeConflict.
	ErrTicketConflict = errors.New("sso: ticket state conflict")

	// ErrTicketAlreadyScanned means a different user already scanned the
	// ticket. Equivalent to identity.ErrQRCodeScanned.
	ErrTicketAlreadyScanned = errors.New("sso: ticket already scanned by another user")

	// ErrStateNotFound means the OAuth state presented at the callback was
	// never issued (or has been consumed). Used by redirect-flow helpers.
	ErrStateNotFound = errors.New("sso: oauth state not found or expired")

	// ErrExchangeFailed means the third-party code exchange (OAuth code →
	// access token, or wx.login code → session) failed at the protocol
	// level (rejected code, bad signature, ...). Wrap with %w to attach
	// provider-side detail.
	ErrExchangeFailed = errors.New("sso: code exchange failed")

	// ErrProviderUnavailable means the third-party IdP returned an
	// unexpected HTTP status or the network call failed. Use this for
	// 5xx, timeouts, and decode failures — not for business-level
	// rejections (those use ErrExchangeFailed).
	ErrProviderUnavailable = errors.New("sso: provider unavailable")
)

// Flow declares which login protocol a Provider implements.
type Flow int

const (
	// FlowRedirect is the browser-redirect OAuth/OIDC flow.
	FlowRedirect Flow = iota
	// FlowQR is the third-party scan-confirm flow (WeChat MiniProgram etc.).
	FlowQR
)

// LinkMode controls what happens when a third-party Assertion does not match
// an existing user_identities row. It is a per-(tenant, provider) setting
// stored in the database, not a compile-time property.
type LinkMode string

const (
	// LinkModeCreate auto-creates a local user on first SSO login and binds
	// the third-party identity to it.
	LinkModeCreate LinkMode = "create"

	// LinkModeBindOnly refuses SSO login for unknown identities. The user
	// must authenticate locally first, then bind the third-party identity
	// through a separate binding endpoint. SSO is allowed, auto-registration
	// is not.
	LinkModeBindOnly LinkMode = "bind_only"
)

// ProviderConfig is the runtime, per-(tenant, provider) configuration loaded
// from the database. It is passed to ProviderFactory.New to instantiate a
// provider for a single request. Providers must NOT cache credentials beyond
// the request scope.
type ProviderConfig struct {
	// TenantID is the tenant this config belongs to.
	TenantID string

	// Name is the provider identifier, e.g. "github", "wechat-mp".
	// Must match the factory's registered Name.
	Name string

	// ClientID and ClientSecret are the OAuth/MiniProgram credentials
	// provisioned for this tenant by the third-party platform.
	ClientID     string
	ClientSecret string

	// RedirectURL is the callback URL the third-party should redirect back to.
	// Constructed per-request from the tenant's domain + the standard path.
	RedirectURL string

	// LinkMode governs account creation on first login (see LinkMode).
	LinkMode LinkMode

	// Scopes is the OAuth scope list for redirect providers. QR providers
	// ignore this.
	Scopes []string

	// Extra is provider-specific config that doesn't fit the common fields
	// (e.g. WeChat requires appid vs web appid; custom OIDC needs
	// discovery URL). Providers unmarshal what they need from here.
	Extra map[string]string
}

// Provider is the base contract. It only declares identity metadata; actual
// login behaviour comes from the capability interfaces. Instances are created
// per-request by a ProviderFactory and carry the ProviderConfig.
type Provider interface {
	// Name is the unique provider identifier, e.g. "github", "wechat-mp".
	// Used in route paths (/auth/sso/{name}) and stored as the `provider`
	// key in user_identities.
	Name() string

	// Flow declares which capability interface this provider implements.
	Flow() Flow
}

// RedirectProvider is the capability for browser-redirect OAuth/OIDC flows.
type RedirectProvider interface {
	Provider

	// BeginAuth returns the URL the browser should be redirected to. `state`
	// must be persisted by the caller and verified in CompleteAuth to defeat
	// CSRF.
	BeginAuth(ctx context.Context, state string) (authURL string, err error)

	// CompleteAuth exchanges the authorization `code` for a token, fetches
	// user info, and normalises it into an Assertion. The caller must have
	// verified `state` already.
	CompleteAuth(ctx context.Context, code string) (*Assertion, error)
}

// QRProvider is the capability for third-party scan-confirm QR flows.
//
// Internal (self-asserted) QR login does NOT use this — it lives in
// plugin/identity.DelegatedAuthenticator, because there the identity is
// asserted by QuotaGate itself, not by a third party.
type QRProvider interface {
	Provider

	// Generate creates a new login ticket and the QR payload.
	Generate(ctx context.Context) (ticket, qrData string, err error)

	// Poll reports ticket status. When confirmed, asrt is populated.
	Poll(ctx context.Context, ticket string) (status string, asrt *Assertion, err error)

	// ResolveExchangeCode is called when the third-party app (e.g. WeChat
	// MiniProgram) posts its login code back. The provider resolves the code
	// with the third-party backend (e.g. code2session), records the identity
	// against the ticket, and returns the Assertion.
	ResolveExchangeCode(ctx context.Context, ticket, code string) (*Assertion, error)
}

// ProviderFactory produces a configured Provider instance for a single
// request. Factories register themselves in init() and are looked up by name
// at runtime; the actual credentials live in ProviderConfig (from DB), not
// in the factory.
type ProviderFactory interface {
	// Name is the provider identifier this factory builds, e.g. "github".
	// Used as the registry key. Declared on the factory (not inferred by
	// instantiating) so registration doesn't require valid credentials.
	Name() string

	// Flow declares which capability the produced provider implements.
	// Lets Methods() report flow without instantiating.
	Flow() Flow

	// New returns a Provider (which will also implement RedirectProvider or
	// QRProvider) configured with the given per-tenant config. The returned
	// instance is single-use; callers should not cache it.
	New(cfg ProviderConfig) (Provider, error)
}

// Assertion is the normalised identity claim produced by an SSO provider.
// It is the SSO equivalent of plugin/identity.Identity, but carries
// provider-side user attributes (not a local user_id) because the local
// user is resolved later by AccountLinker using (provider, subject) against
// the user_identities table.
//
// The field set is QuotaGate's own minimal contract. Each provider is
// responsible for mapping its raw response into these fields; fields the
// provider does not expose are left zero.
type Assertion struct {
	// Provider is the originating provider name, e.g. "github".
	Provider string `json:"provider"`

	// Subject is the user's unique identifier at the provider.
	// GitHub: numeric id as string; WeChat: openid (unionid preferred when
	// available, see UnionID); OIDC: the `sub` claim.
	Subject string `json:"subject"`

	// UnionID is the cross-app identifier some providers expose
	// (e.g. WeChat unionid). AccountLinker prefers UnionID over Subject when
	// both are present, so the same person logging in via different apps of
	// the same vendor resolves to one local user.
	UnionID string `json:"union_id,omitempty"`

	// Username is the provider-side login/handle, e.g. GitHub login.
	Username string `json:"username,omitempty"`

	// DisplayName is the human-readable name from the provider.
	DisplayName string `json:"display_name,omitempty"`

	// Email is the verified or primary email, when the provider exposes it.
	Email string `json:"email,omitempty"`

	// EmailVerified reports whether the provider actually verified this
	// email (e.g. GitHub /user/emails carries a verified flag; OIDC id_token
	// carries email_verified). Providers that only surface an unverified
	// primary email MUST set this to false. The AccountLinker uses this to
	// populate model.User.EmailVerified accurately instead of blanket
	// trusting every provider-asserted address.
	EmailVerified bool `json:"email_verified,omitempty"`

	// Phone and CountryCode are populated by providers that expose them.
	Phone       string `json:"phone,omitempty"`
	CountryCode string `json:"country_code,omitempty"`

	// AvatarURL is the user's avatar at the provider.
	AvatarURL string `json:"avatar_url,omitempty"`

	// Raw is the full user-info response from the provider, kept for
	// debugging, audit, and for fields not promoted to the struct above.
	Raw map[string]any `json:"raw,omitempty"`
}

// QR ticket lifecycle statuses. Mirrors the naming of
// plugin/identity.Delegation* so the front-end can treat both families
// uniformly.
const (
	StatusPending   = "pending"
	StatusScanned   = "scanned"
	StatusConfirmed = "confirmed"
	StatusCancelled = "cancelled"
	StatusExpired   = "expired"
)
