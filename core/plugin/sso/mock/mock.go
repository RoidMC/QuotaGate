// Package mock provides fake SSO providers for testing the SSO plugin layer
// without hitting real third-party APIs. The redirect mock returns a
// GitHub-shaped payload; the qr mock returns a WeChat MiniProgram-shaped
// payload. The field mappings below are *examples* of how a real provider
// would map its raw response into sso.Assertion — they are not a contract.
//
// Like real providers, mocks register a ProviderFactory in init() and
// produce per-request instances via New(cfg). The cfg carries the store
// (injected via Extra) so tests can supply an isolated kexswiftdb.
package mock

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"time"

	"github.com/roidmc/quotagate/internal/util/random"
	"github.com/roidmc/quotagate/pkg/kexswiftdb"
	"github.com/roidmc/quotagate/plugin/sso"
)

// nsMockRedirect / nsMockQR are the kexswiftdb prefixes for mock state.
const (
	nsMockRedirect kexswiftdb.Prefix = "sso:mock:redirect"
	nsMockQR       kexswiftdb.Prefix = "sso:mock:qr"
)

const (
	defaultStateTTL  = 5 * time.Minute
	defaultTicketTTL = 5 * time.Minute
)

// MockExtraBaseURL is the ProviderConfig.Extra key that configures the
// redirect mock's authorization-server base URL. Tests use it to point
// the mock at an httptest.Server; production code never touches it.
const MockExtraBaseURL = "mock_base_url"

// MockExtraTTL is the ProviderConfig.Extra key that overrides the QR mock's
// ticket TTL (a Go time.Duration string, e.g. "1s"). Tests use it to drive
// expiry scenarios without real-time waits.
const MockExtraTTL = "mock_ttl"

// ---- GitHub-shaped raw response (example mapping source) ----

// githubUserInfo is an example of a provider's raw response struct. A real
// github provider would define this from the GitHub API spec; the mock
// reproduces its shape so anything consuming GitHub-style responses can be
// tested against realistic data.
type githubUserInfo struct {
	Login      string `json:"login"`
	ID         int    `json:"id"`
	NodeID     string `json:"node_id"`
	AvatarURL  string `json:"avatar_url"`
	HTMLURL    string `json:"html_url"`
	Name       string `json:"name"`
	Company    string `json:"company"`
	Blog       string `json:"blog"`
	Location   string `json:"location"`
	Email      string `json:"email"`
	Bio        string `json:"bio"`
	CreatedAt  string `json:"created_at"`
	UpdatedAt  string `json:"updated_at"`
}

// ---- WeChat MiniProgram-shaped raw response (example mapping source) ----

// weChatMiniProgramSession is an example of a provider's raw response struct.
// A real wechat-mp provider would define this from the code2session API spec.
type weChatMiniProgramSession struct {
	Openid     string `json:"openid"`
	SessionKey string `json:"session_key"`
	Unionid    string `json:"unionid"`
	Errcode    int    `json:"errcode"`
	Errmsg     string `json:"errmsg"`
}

// qrTicketEntry is the persisted ticket state. Mirrors the shape of
// identity/qrcode.go's qrcodeEntry so the front-end polling contract is
// identical for internal and third-party QR.
type qrTicketEntry struct {
	Status    string                    `json:"status"`
	Code      string                    `json:"code,omitempty"`
	Session   *weChatMiniProgramSession `json:"session,omitempty"`
	ExpiresAt int64                     `json:"expires_at"`
}

func (e *qrTicketEntry) isExpired() bool { return time.Now().Unix() > e.ExpiresAt }
func (e *qrTicketEntry) remainingTTL() time.Duration {
	return time.Until(time.Unix(e.ExpiresAt, 0))
}

// ---- RedirectMock (GitHub-shaped) ----

type redirectInstance struct {
	store   kexswiftdb.Store
	baseURL string
}

func (m *redirectInstance) Name() string    { return "mock-github" }
func (m *redirectInstance) Flow() sso.Flow  { return sso.FlowRedirect }

func (m *redirectInstance) BeginAuth(ctx context.Context, state string) (string, error) {
	if err := kexswiftdb.SetJSON(ctx, m.store, nsMockRedirect, state, struct {
		CreatedAt time.Time `json:"created_at"`
	}{time.Now()}, defaultStateTTL); err != nil {
		// Infrastructure failure — not a state lifecycle error, so wrap
		// ErrProviderUnavailable rather than ErrStateNotFound (the latter
		// is for "state presented but not recognised").
		return "", fmt.Errorf("sso/mock: store state: %w: %v", sso.ErrProviderUnavailable, err)
	}
	// Build the URL with proper escaping — state is a UUID in normal use,
	// but robust URL construction avoids surprises if it ever carries other
	// characters and keeps the mock honest as a reference implementation.
	u, err := url.Parse(m.baseURL)
	if err != nil {
		return "", fmt.Errorf("sso/mock: parse base url: %w", err)
	}
	u = u.JoinPath("oauth", "authorize")
	q := u.Query()
	q.Set("state", state)
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (m *redirectInstance) CompleteAuth(ctx context.Context, code string) (*sso.Assertion, error) {
	// A real provider would exchange `code` for a token then call /user.
	// The mock synthesises a GitHub-shaped response derived from `code`.
	gh := githubUserInfo{
		Login:     fmt.Sprintf("mockuser-%s", code),
		ID:        3781234,
		NodeID:    "MDQ6VXNlcjM3O123456=",
		AvatarURL: "https://avatars.githubusercontent.com/u/3781234?v=4",
		HTMLURL:   "https://github.com/mockuser",
		Name:      "Mock User",
		Company:   "RoidMC Studios",
		Blog:      "https://roidmc.com",
		Location:  "Bay Area",
		Email:     fmt.Sprintf("mockuser-%s@example.com", code),
		Bio:       "My bio",
		CreatedAt: "2016-03-06T13:16:13Z",
		UpdatedAt: "2020-05-30T12:15:29Z",
	}

	// Example mapping (NOT a contract): a real github provider maps the same
	// way, a wechat-web provider maps differently, etc.
	return &sso.Assertion{
		Provider:    m.Name(),
		Subject:     fmt.Sprintf("%d", gh.ID),
		Username:    gh.Login,
		DisplayName: gh.Name,
		Email:       gh.Email,
		AvatarURL:   gh.AvatarURL,
		Raw: map[string]any{
			"login":      gh.Login,
			"id":         gh.ID,
			"node_id":    gh.NodeID,
			"html_url":   gh.HTMLURL,
			"company":    gh.Company,
			"location":   gh.Location,
			"bio":        gh.Bio,
			"created_at": gh.CreatedAt,
			"updated_at": gh.UpdatedAt,
		},
	}, nil
}

// ---- QRMock (WeChat MiniProgram-shaped) ----

type qrInstance struct {
	store kexswiftdb.Store
	ttl   time.Duration
}

func (m *qrInstance) Name() string   { return "mock-wechat-mp" }
func (m *qrInstance) Flow() sso.Flow { return sso.FlowQR }

func (m *qrInstance) Generate(ctx context.Context) (ticket, qrData string, err error) {
	ticket = random.MustUUIDString()
	entry := qrTicketEntry{
		Status:    sso.StatusPending,
		ExpiresAt: time.Now().Add(m.ttl).Unix(),
	}
	if err := kexswiftdb.SetJSON(ctx, m.store, nsMockQR, ticket, entry, m.ttl); err != nil {
		return "", "", fmt.Errorf("sso/mock: store ticket: %w: %v", sso.ErrProviderUnavailable, err)
	}
	return ticket, fmt.Sprintf("kex:sso:mock:qr:%s", ticket), nil
}

func (m *qrInstance) ResolveExchangeCode(ctx context.Context, ticket, code string) (*sso.Assertion, error) {
	result, err := kexswiftdb.MutateJSON[qrTicketEntry](ctx, m.store, nsMockQR, ticket, func(current *qrTicketEntry) (qrTicketEntry, bool, time.Duration, error) {
		if current == nil {
			return qrTicketEntry{}, false, 0, sso.ErrTicketNotFound
		}
		if current.isExpired() {
			return qrTicketEntry{}, false, 0, sso.ErrTicketExpired
		}
		if current.Status != sso.StatusPending {
			// Distinguish "already confirmed" (legit retry by the same app)
			// from other terminal states, so callers can suppress duplicate
			// confirmations without surfacing them as errors.
			if current.Status == sso.StatusConfirmed {
				return qrTicketEntry{}, false, 0, fmt.Errorf("sso/mock: ticket already confirmed: %w", sso.ErrTicketConflict)
			}
			return qrTicketEntry{}, false, 0, fmt.Errorf("sso/mock: ticket not pending (status=%s): %w", current.Status, sso.ErrTicketConflict)
		}
		// Simulate code2session: the exchange code becomes the openid.
		current.Status = sso.StatusConfirmed
		current.Code = code
		current.Session = &weChatMiniProgramSession{
			Openid:     fmt.Sprintf("mock-openid-%s", code),
			SessionKey: "mock-session-key",
			Unionid:    fmt.Sprintf("mock-unionid-%s", code),
			Errcode:    0,
		}
		return *current, true, current.remainingTTL(), nil
	})
	if err != nil {
		return nil, err
	}
	if result == nil {
		// Should not happen: MutateJSON returns the stored value on a
		// successful commit. Treat defensively as a conflict.
		return nil, sso.ErrTicketConflict
	}
	return m.sessionToAssertion(result.Session), nil
}

func (m *qrInstance) Poll(ctx context.Context, ticket string) (status string, asrt *sso.Assertion, err error) {
	entry, err := kexswiftdb.GetJSON[qrTicketEntry](ctx, m.store, nsMockQR, ticket)
	if err != nil {
		if errors.Is(err, kexswiftdb.ErrKeyNotFound) {
			// Missing ticket is a business-level "expired/unknown" status,
			// not an internal error — the polling client just sees Expired.
			return sso.StatusExpired, nil, nil
		}
		// Real store failure (badger IO, etc.). Surface as provider-side
		// outage so PollQR callers can map to 5xx rather than silently
		// returning Expired.
		return "", nil, fmt.Errorf("sso/mock: poll ticket: %w: %v", sso.ErrProviderUnavailable, err)
	}
	if entry.isExpired() {
		return sso.StatusExpired, nil, nil
	}
	if entry.Status == sso.StatusConfirmed && entry.Session != nil {
		return sso.StatusConfirmed, m.sessionToAssertion(entry.Session), nil
	}
	return entry.Status, nil, nil
}

// sessionToAssertion is an example mapping for WeChat MiniProgram. A real
// wechat-mp provider maps the same way: openid→Subject, unionid→UnionID.
// WeChat MiniProgram does not expose email/phone/avatar via code2session,
// so those stay empty.
func (m *qrInstance) sessionToAssertion(s *weChatMiniProgramSession) *sso.Assertion {
	if s == nil {
		return nil
	}
	return &sso.Assertion{
		Provider: m.Name(),
		Subject:  s.Openid,
		UnionID:  s.Unionid,
		Raw: map[string]any{
			"openid":      s.Openid,
			"session_key": s.SessionKey,
			"unionid":     s.Unionid,
		},
	}
}

// ---- factory self-registration ----
//
// Real provider packages (standard/github, china/wechat_mp, ...) register a
// stateless factory in their own init() — they don't need infrastructure
// injected, they call the third-party API with cfg.ClientID/ClientSecret.
//
// Mocks are different: they need a kexswiftdb.Store to persist state, which
// is infrastructure, not per-tenant config. So mocks expose NewRegistry(store)
// instead of self-registering into a global. Tests build an isolated registry
// per test; boot never wires mocks (they're a test-only aid).

// NewRegistry returns an sso.Registry with both mock factories registered,
// each closure-capturing the given store so tests get full isolation.
func NewRegistry(store kexswiftdb.Store) *sso.Registry {
	reg := sso.NewRegistry()
	reg.RegisterFactory(redirectMockFactory{store: store})
	reg.RegisterFactory(qrMockFactory{store: store})
	return reg
}

type redirectMockFactory struct{ store kexswiftdb.Store }

func (redirectMockFactory) Name() string   { return "mock-github" }
func (redirectMockFactory) Flow() sso.Flow { return sso.FlowRedirect }

func (f redirectMockFactory) New(cfg sso.ProviderConfig) (sso.Provider, error) {
	baseURL := cfg.Extra[MockExtraBaseURL]
	if baseURL == "" {
		baseURL = "https://idp.test"
	}
	return &redirectInstance{store: f.store, baseURL: baseURL}, nil
}

type qrMockFactory struct{ store kexswiftdb.Store }

func (qrMockFactory) Name() string   { return "mock-wechat-mp" }
func (qrMockFactory) Flow() sso.Flow { return sso.FlowQR }

func (f qrMockFactory) New(cfg sso.ProviderConfig) (sso.Provider, error) {
	ttl := defaultTicketTTL
	if ttlStr, ok := cfg.Extra[MockExtraTTL]; ok {
		if parsed, err := time.ParseDuration(ttlStr); err == nil {
			ttl = parsed
		}
	}
	return &qrInstance{store: f.store, ttl: ttl}, nil
}
