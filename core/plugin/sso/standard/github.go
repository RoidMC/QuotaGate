// Package standard hosts OAuth/OIDC providers for international IdPs
// (GitHub, Google, custom OIDC, ...). Each provider package self-registers
// its factory into the parent sso.defaultRegistry via init().
package standard

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	"golang.org/x/oauth2"

	"github.com/roidmc/quotagate/plugin/sso"
)

func init() {
	sso.DefaultRegistry().Register(githubFactory{})
}

// githubFactory is a stateless ProviderFactory for GitHub OAuth. Credentials
// come from ProviderConfig (loaded from sso_provider_configs by the service
// layer); the factory itself holds nothing.
type githubFactory struct{}

func (githubFactory) Name() string   { return "github" }
func (githubFactory) Flow() sso.Flow { return sso.FlowRedirect }

func (githubFactory) New(cfg sso.ProviderConfig) (sso.Provider, error) {
	if cfg.ClientID == "" || cfg.ClientSecret == "" {
		return nil, fmt.Errorf("sso/github: ClientID and ClientSecret are required")
	}
	scopes := cfg.Scopes
	if len(scopes) == 0 {
		// GitHub default scopes mirror casdoor's github provider: read:user
		// gives profile, user:email gives access to the emails API.
		scopes = []string{"user:email", "read:user"}
	}

	// Endpoint URLs default to public GitHub. Extra overrides allow:
	//   - github_auth_url  / github_token_url: point OAuth at a GitHub Enterprise
	//     instance or a test double.
	//   - github_api_base:  point /user and /user/emails at an alternate API root
	//     (Enterprise uses <host>/api/v3) or a test double.
	authURL := cfg.Extra["github_auth_url"]
	if authURL == "" {
		authURL = "https://github.com/login/oauth/authorize"
	}
	tokenURL := cfg.Extra["github_token_url"]
	if tokenURL == "" {
		tokenURL = "https://github.com/login/oauth/access_token"
	}
	apiBase := cfg.Extra["github_api_base"]
	if apiBase == "" {
		apiBase = githubAPIBase
	}

	ocfg := &oauth2.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		RedirectURL:  cfg.RedirectURL,
		Scopes:       scopes,
		Endpoint: oauth2.Endpoint{
			AuthURL:  authURL,
			TokenURL: tokenURL,
		},
	}
	hc := &http.Client{Timeout: 10 * time.Second}
	return &githubProvider{config: ocfg, client: hc, apiBase: apiBase}, nil
}

// githubAPIBase is the default GitHub REST API root. Overridable via the
// apiBase field on githubProvider for tests (fake server).
const githubAPIBase = "https://api.github.com"

type githubProvider struct {
	config  *oauth2.Config
	client  *http.Client
	apiBase string // root for /user and /user/emails; default githubAPIBase
}

func (p *githubProvider) Name() string   { return "github" }
func (p *githubProvider) Flow() sso.Flow { return sso.FlowRedirect }

func (p *githubProvider) BeginAuth(ctx context.Context, state string) (string, error) {
	// AccessTypeOnline avoids GitHub issuing a refresh token we don't need
	// for login. The URL also carries state for CSRF; the caller persists it
	// and verifies it on callback.
	return p.config.AuthCodeURL(state, oauth2.AccessTypeOnline), nil
}

func (p *githubProvider) CompleteAuth(ctx context.Context, code string) (*sso.Assertion, error) {
	// 1. code → access_token. oauth2.Exchange sets Accept: application/json
	//    automatically via the GitHub endpoint.
	token, err := p.config.Exchange(ctx, code)
	if err != nil {
		// oauth2 surfaces rejected codes, bad signatures, and network errors
		// all as Exchange errors. Map to ErrExchangeFailed so callers can
		// distinguish "user-side reject" from "provider down" by inspecting
		// the wrapped chain (use errors.Is(err, sso.ErrProviderUnavailable)
		// to detect the latter if oauth2 ever exposes a typed network error).
		return nil, fmt.Errorf("sso/github: exchange code: %w: %v", sso.ErrExchangeFailed, err)
	}

	// 2. token → /user profile. We use the token's own HTTP client so the
	//    Authorization header is set automatically.
	src := p.config.TokenSource(ctx, token)

	ghUser, err := p.fetchGitHubUser(ctx, src)
	if err != nil {
		return nil, err
	}

	// 3. GitHub may return an empty email if the user hid it. Fall back to
	//    /user/emails and pick the primary verified one — same logic as
	//    casdoor's github provider.
	emailVerified := false // default: GH /user does not surface verification
	if ghUser.Email == "" {
		if email, verified, err := p.fetchPrimaryEmail(ctx, src); err == nil {
			ghUser.Email = email
			emailVerified = verified
		}
		// Non-fatal: a user with no resolvable email still logs in; the
		// AccountLinker decides whether email is required for auto-create.
	}

	// 4. Map into Assertion. Id→Subject (as string), Login→Username,
	//    Name→DisplayName, Email→Email, AvatarUrl→AvatarURL.
	return &sso.Assertion{
		Provider:      p.Name(),
		Subject:       strconv.Itoa(ghUser.ID),
		Username:      ghUser.Login,
		DisplayName:   ghUser.Name,
		Email:         ghUser.Email,
		EmailVerified: emailVerified,
		AvatarURL:     ghUser.AvatarURL,
		Raw: map[string]any{
			"login":        ghUser.Login,
			"id":           ghUser.ID,
			"node_id":      ghUser.NodeID,
			"html_url":     ghUser.HTMLURL,
			"company":      ghUser.Company,
			"location":     ghUser.Location,
			"bio":          ghUser.Bio,
			"public_repos": ghUser.PublicRepos,
			"followers":    ghUser.Followers,
			"following":    ghUser.Following,
			"created_at":   ghUser.CreatedAt,
			"updated_at":   ghUser.UpdatedAt,
		},
	}, nil
}

// githubUser is the subset of GET /user fields we promote into Assertion.
// Extra fields are kept in Raw for audit/debugging.
type githubUser struct {
	Login       string `json:"login"`
	ID          int    `json:"id"`
	NodeID      string `json:"node_id"`
	AvatarURL   string `json:"avatar_url"`
	HTMLURL     string `json:"html_url"`
	Name        string `json:"name"`
	Company     string `json:"company"`
	Location    string `json:"location"`
	Email       string `json:"email"`
	Bio         string `json:"bio"`
	PublicRepos int    `json:"public_repos"`
	Followers   int    `json:"followers"`
	Following   int    `json:"following"`
	CreatedAt   string `json:"created_at"`
	UpdatedAt   string `json:"updated_at"`
}

// githubEmail is one entry from GET /user/emails.
type githubEmail struct {
	Email      string `json:"email"`
	Primary    bool   `json:"primary"`
	Verified   bool   `json:"verified"`
	Visibility string `json:"visibility"`
}

func (p *githubProvider) fetchGitHubUser(ctx context.Context, src oauth2.TokenSource) (*githubUser, error) {
	client := oauth2.NewClient(ctx, src)
	resp, err := client.Get(p.apiBase + "/user")
	if err != nil {
		return nil, fmt.Errorf("sso/github: fetch /user: %w: %v", sso.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		// 4xx (401/404/...) is a provider-side rejection of our token or
		// request shape; 5xx is provider outage. Both surface as
		// ErrProviderUnavailable — distinguishing would require parsing
		// GitHub's error contract, which isn't worth it for login.
		return nil, fmt.Errorf("sso/github: /user returned %d: %w: %s",
			resp.StatusCode, sso.ErrProviderUnavailable, body)
	}
	var u githubUser
	if err := json.NewDecoder(resp.Body).Decode(&u); err != nil {
		return nil, fmt.Errorf("sso/github: decode /user: %w: %v", sso.ErrProviderUnavailable, err)
	}
	return &u, nil
}

func (p *githubProvider) fetchPrimaryEmail(ctx context.Context, src oauth2.TokenSource) (string, bool, error) {
	client := oauth2.NewClient(ctx, src)
	resp, err := client.Get(p.apiBase + "/user/emails")
	if err != nil {
		return "", false, fmt.Errorf("sso/github: fetch /user/emails: %w: %v", sso.ErrProviderUnavailable, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("sso/github: /user/emails returned %d: %w", resp.StatusCode, sso.ErrProviderUnavailable)
	}
	var emails []githubEmail
	if err := json.NewDecoder(resp.Body).Decode(&emails); err != nil {
		return "", false, fmt.Errorf("sso/github: decode /user/emails: %w: %v", sso.ErrProviderUnavailable, err)
	}

	// Prefer primary + verified; fall back to any verified; then any primary.
	// Carry the verified flag up so the AccountLinker can populate
	// model.User.EmailVerified accurately — a primary email that the user
	// hid from their GitHub profile may be unverified, and blanket-trusting
	// it opens an account-takeover vector via a malicious GitHub account
	// that set its primary email to a victim's address.
	primaryVerified := ""
	anyVerified := ""
	anyPrimary := ""
	for _, e := range emails {
		if !e.Verified {
			continue
		}
		if e.Primary {
			primaryVerified = e.Email
			break
		}
		if anyVerified == "" {
			anyVerified = e.Email
		}
	}
	if primaryVerified != "" {
		return primaryVerified, true, nil
	}
	if anyVerified != "" {
		return anyVerified, true, nil
	}
	for _, e := range emails {
		if e.Primary && anyPrimary == "" {
			anyPrimary = e.Email
		}
	}
	// anyPrimary (when reached) is a primary but unverified email. Surface
	// verified=false so the caller can decide policy (e.g. require email
	// verification before allowing SSO auto-create).
	return anyPrimary, false, nil
}
