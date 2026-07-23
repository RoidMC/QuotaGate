package standard_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/roidmc/quotagate/plugin/sso"
	// Side-effect import: registers githubFactory into sso.DefaultRegistry.
	_ "github.com/roidmc/quotagate/plugin/sso/standard"
)

// fakeGitHub stands up a fake GitHub OAuth + API server. It serves:
//
//	GET  /login/oauth/authorize    → 302 to redirect_url?code=fake-code&state=...
//	POST /login/oauth/access_token → {"access_token":"fake-token","token_type":"bearer"}
//	GET  /user                     → profile JSON
//	GET  /user/emails              → email list JSON
type fakeGitHub struct {
	t        *testing.T
	server   *httptest.Server
	userResp map[string]any
	emails   []map[string]any
}

func newFakeGitHub(t *testing.T) *fakeGitHub {
	f := &fakeGitHub{t: t}
	mux := http.NewServeMux()

	mux.HandleFunc("/login/oauth/authorize", func(w http.ResponseWriter, r *http.Request) {
		state := r.URL.Query().Get("state")
		redirect := r.URL.Query().Get("redirect_uri")
		http.Redirect(w, r, redirect+"?code=fake-code&state="+state, http.StatusFound)
	})

	mux.HandleFunc("/login/oauth/access_token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"access_token":"fake-token","token_type":"bearer","scope":"user:email"}`))
	})

	mux.HandleFunc("/user", func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer fake-token" {
			t.Errorf("/user: Authorization = %q, want %q", got, "Bearer fake-token")
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(f.userResp)
	})

	mux.HandleFunc("/user/emails", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(f.emails)
	})

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

// newProvider builds a github RedirectProvider via the public registry API,
// with Extra overrides pointing all GitHub URLs at the fake server.
func newProvider(t *testing.T, baseURL, redirectURL string) sso.RedirectProvider {
	t.Helper()
	reg := sso.DefaultRegistry()
	p, err := reg.New(sso.ProviderConfig{
		TenantID:     "test",
		Name:         "github",
		ClientID:     "test-client-id",
		ClientSecret: "test-secret",
		RedirectURL:  redirectURL,
		Extra: map[string]string{
			"github_auth_url":  baseURL + "/login/oauth/authorize",
			"github_token_url": baseURL + "/login/oauth/access_token",
			"github_api_base":  baseURL,
		},
	})
	if err != nil {
		t.Fatalf("registry.New: %v", err)
	}
	rp, ok := p.(sso.RedirectProvider)
	if !ok {
		t.Fatalf("provider is not RedirectProvider: %T", p)
	}
	return rp
}

func TestGitHub_BeginAuth_URL(t *testing.T) {
	f := newFakeGitHub(t)
	p := newProvider(t, f.server.URL, "https://app.test/callback")
	ctx := context.Background()

	state := "csrf-state-123"
	authURL, err := p.BeginAuth(ctx, state, "")
	if err != nil {
		t.Fatalf("BeginAuth: %v", err)
	}

	u, err := url.Parse(authURL)
	if err != nil {
		t.Fatalf("parse authURL: %v", err)
	}
	if u.Scheme != "http" || u.Host != strings.TrimPrefix(f.server.URL, "http://") {
		t.Fatalf("authURL host: got %s, want %s", u.Host, f.server.URL)
	}
	if u.Path != "/login/oauth/authorize" {
		t.Fatalf("authURL path: got %s", u.Path)
	}
	if got := u.Query().Get("state"); got != state {
		t.Fatalf("state: got %q want %q", got, state)
	}
	if got := u.Query().Get("client_id"); got != "test-client-id" {
		t.Fatalf("client_id: got %q", got)
	}
	if got := u.Query().Get("redirect_uri"); got != "https://app.test/callback" {
		t.Fatalf("redirect_uri: got %q", got)
	}
	scopes := u.Query().Get("scope")
	if !strings.Contains(scopes, "user:email") {
		t.Fatalf("scope missing user:email: %q", scopes)
	}
}

func TestGitHub_CompleteAuth_WithEmailInProfile(t *testing.T) {
	f := newFakeGitHub(t)
	f.userResp = map[string]any{
		"login":        "octocat",
		"id":           1,
		"node_id":      "MDQ6VXNlcjE=",
		"avatar_url":   "https://github.com/images/error/octocat_happy.gif",
		"html_url":     "https://github.com/octocat",
		"name":         "The Octocat",
		"company":      "GitHub",
		"location":     "San Francisco",
		"email":        "octocat@github.com",
		"bio":          "",
		"public_repos": 2,
		"followers":    20,
		"following":    0,
		"created_at":   "2011-01-25T18:44:36Z",
		"updated_at":   "2011-01-25T18:44:36Z",
	}
	p := newProvider(t, f.server.URL, "https://app.test/callback")
	ctx := context.Background()

	asrt, err := p.CompleteAuth(ctx, "fake-code", "")
	if err != nil {
		t.Fatalf("CompleteAuth: %v", err)
	}

	if asrt.Provider != "github" {
		t.Fatalf("Provider: got %q", asrt.Provider)
	}
	if asrt.Subject != "1" {
		t.Fatalf("Subject: got %q want %q", asrt.Subject, "1")
	}
	if asrt.Username != "octocat" {
		t.Fatalf("Username: got %q", asrt.Username)
	}
	if asrt.DisplayName != "The Octocat" {
		t.Fatalf("DisplayName: got %q", asrt.DisplayName)
	}
	if asrt.Email != "octocat@github.com" {
		t.Fatalf("Email: got %q", asrt.Email)
	}
	if asrt.AvatarURL != f.userResp["avatar_url"] {
		t.Fatalf("AvatarURL: got %q", asrt.AvatarURL)
	}
	if asrt.Raw["login"] != "octocat" {
		t.Fatalf("Raw.login: got %v", asrt.Raw["login"])
	}
	// Raw["id"] comes from ghUser.ID (an int) placed directly into the map
	// by the provider — not re-decoded from JSON — so it stays int.
	if asrt.Raw["id"] != 1 {
		t.Fatalf("Raw.id: got %v", asrt.Raw["id"])
	}
}

func TestGitHub_CompleteAuth_EmailFallback(t *testing.T) {
	f := newFakeGitHub(t)
	f.userResp = map[string]any{
		"login": "private-user",
		"id":    42,
		"email": "", // hidden
	}
	f.emails = []map[string]any{
		{"email": "secondary@example.com", "primary": false, "verified": true},
		{"email": "primary@example.com", "primary": true, "verified": true},
		{"email": "unverified@example.com", "primary": false, "verified": false},
	}
	p := newProvider(t, f.server.URL, "https://app.test/callback")
	ctx := context.Background()

	asrt, err := p.CompleteAuth(ctx, "fake-code", "")
	if err != nil {
		t.Fatalf("CompleteAuth: %v", err)
	}
	if asrt.Email != "primary@example.com" {
		t.Fatalf("Email fallback: got %q want %q", asrt.Email, "primary@example.com")
	}
}

func TestGitHub_FactoryRequiresCredentials(t *testing.T) {
	reg := sso.DefaultRegistry()
	_, err := reg.New(sso.ProviderConfig{TenantID: "test", Name: "github"})
	if err == nil {
		t.Fatal("expected error for empty ClientID/ClientSecret")
	}
}

func TestGitHub_FactoryRegistersInDefaultRegistry(t *testing.T) {
	reg := sso.DefaultRegistry()
	if !reg.Has("github") {
		t.Fatal("DefaultRegistry: github not registered (init did not run?)")
	}
	methods := reg.Methods()
	found := false
	for _, m := range methods {
		if m.Name == "github" && m.Flow == "redirect" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("github not in Methods(): %+v", methods)
	}
}
