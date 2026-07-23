package mock_test

import (
	"context"
	"testing"
	"time"

	"github.com/roidmc/kex-utils/pkg/kexswiftdb"
	"github.com/roidmc/quotagate/plugin/sso"
	"github.com/roidmc/quotagate/plugin/sso/mock"
)

func newStore(t *testing.T) kexswiftdb.Store {
	t.Helper()
	store, err := kexswiftdb.NewInMemoryBadgerStore()
	if err != nil {
		t.Fatalf("NewInMemoryBadgerStore: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// cfgWith returns a ProviderConfig as boot would build one from a DB row.
func cfgWith(name string, extra map[string]string) sso.ProviderConfig {
	return sso.ProviderConfig{
		TenantID: "test-tenant",
		Name:     name,
		LinkMode: sso.LinkModeCreate,
		Extra:    extra,
	}
}

func TestRedirectMock_BeginAndComplete(t *testing.T) {
	store := newStore(t)
	reg := mock.NewRegistry(store)

	cfg := cfgWith("mock-github", map[string]string{
		mock.MockExtraBaseURL: "https://idp.test",
	})
	p, err := reg.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	rp, ok := p.(sso.RedirectProvider)
	if !ok {
		t.Fatalf("not a RedirectProvider: %T", p)
	}
	ctx := context.Background()

	if p.Name() != "mock-github" {
		t.Fatalf("Name: got %q", p.Name())
	}
	if p.Flow() != sso.FlowRedirect {
		t.Fatalf("Flow: got %v", p.Flow())
	}

	state := "state-abc"
	url, err := rp.BeginAuth(ctx, state, "")
	if err != nil {
		t.Fatalf("BeginAuth: %v", err)
	}
	if url == "" {
		t.Fatal("BeginAuth: empty url")
	}
	t.Logf("redirect url: %s", url)

	asrt, err := rp.CompleteAuth(ctx, "code-123", "")
	if err != nil {
		t.Fatalf("CompleteAuth: %v", err)
	}
	if asrt.Provider != "mock-github" {
		t.Fatalf("Provider: got %q", asrt.Provider)
	}
	if asrt.Subject != "3781234" {
		t.Fatalf("Subject: got %q", asrt.Subject)
	}
	if asrt.Username != "mockuser-code-123" {
		t.Fatalf("Username: got %q", asrt.Username)
	}
	if asrt.Email != "mockuser-code-123@example.com" {
		t.Fatalf("Email: got %q", asrt.Email)
	}
}

func TestQRMock_FullFlow(t *testing.T) {
	store := newStore(t)
	reg := mock.NewRegistry(store)

	cfg := cfgWith("mock-wechat-mp", map[string]string{})
	p, err := reg.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	qp, ok := p.(sso.QRProvider)
	if !ok {
		t.Fatalf("not a QRProvider: %T", p)
	}
	ctx := context.Background()

	if p.Name() != "mock-wechat-mp" {
		t.Fatalf("Name: got %q", p.Name())
	}
	if p.Flow() != sso.FlowQR {
		t.Fatalf("Flow: got %v", p.Flow())
	}

	ticket, qrData, err := qp.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if ticket == "" || qrData == "" {
		t.Fatalf("Generate: empty ticket=%q qrData=%q", ticket, qrData)
	}
	t.Logf("ticket=%s qrData=%s", ticket, qrData)

	// pending
	status, asrt, err := qp.Poll(ctx, ticket)
	if err != nil {
		t.Fatalf("Poll (pending): %v", err)
	}
	if status != sso.StatusPending {
		t.Fatalf("status: got %q want %q", status, sso.StatusPending)
	}
	if asrt != nil {
		t.Fatalf("expected nil assertion when pending, got %+v", asrt)
	}

	// mini program posts wx.login code
	asrt, err = qp.ResolveExchangeCode(ctx, ticket, "wx-code-xyz")
	if err != nil {
		t.Fatalf("ResolveExchangeCode: %v", err)
	}
	if asrt == nil {
		t.Fatal("ResolveExchangeCode: nil assertion")
	}
	if asrt.Subject != "mock-openid-wx-code-xyz" {
		t.Fatalf("Subject: got %q", asrt.Subject)
	}
	if asrt.UnionID != "mock-unionid-wx-code-xyz" {
		t.Fatalf("UnionID: got %q", asrt.UnionID)
	}

	// confirmed
	status, asrt, err = qp.Poll(ctx, ticket)
	if err != nil {
		t.Fatalf("Poll (confirmed): %v", err)
	}
	if status != sso.StatusConfirmed {
		t.Fatalf("status: got %q want %q", status, sso.StatusConfirmed)
	}
	if asrt == nil || asrt.Subject != "mock-openid-wx-code-xyz" {
		t.Fatalf("confirmed assertion mismatch: %+v", asrt)
	}
}

func TestQRMock_ExpiredTicket(t *testing.T) {
	store := newStore(t)
	reg := mock.NewRegistry(store)

	cfg := cfgWith("mock-wechat-mp", map[string]string{
		mock.MockExtraTTL: "50ms",
	})
	p, err := reg.New(cfg)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	qp := p.(sso.QRProvider)
	ctx := context.Background()

	ticket, _, err := qp.Generate(ctx)
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// 50ms TTL + 100ms wait keeps the test fast while avoiding flakiness
	// from scheduler jitter on slow CI machines.
	time.Sleep(100 * time.Millisecond)

	status, _, err := qp.Poll(ctx, ticket)
	if err != nil {
		t.Fatalf("Poll (expired): %v", err)
	}
	if status != sso.StatusExpired {
		t.Fatalf("status: got %q want %q", status, sso.StatusExpired)
	}
}

func TestRegistry_CompiledInAndDispatch(t *testing.T) {
	store := newStore(t)
	reg := mock.NewRegistry(store)

	if !reg.Has("mock-github") {
		t.Fatal("Has mock-github: false")
	}
	if !reg.Has("mock-wechat-mp") {
		t.Fatal("Has mock-wechat-mp: false")
	}
	if reg.Has("nonexistent") {
		t.Fatal("Has nonexistent: should be false")
	}

	methods := reg.Methods()
	if len(methods) != 2 {
		t.Fatalf("Methods: got %d want 2", len(methods))
	}
	if methods[0].Name != "mock-github" || methods[0].Flow != "redirect" {
		t.Fatalf("methods[0]: %+v", methods[0])
	}
	if methods[1].Name != "mock-wechat-mp" || methods[1].Flow != "qr" {
		t.Fatalf("methods[1]: %+v", methods[1])
	}
}

func TestLinkMode_Configurable(t *testing.T) {
	store := newStore(t)
	reg := mock.NewRegistry(store)

	for _, mode := range []sso.LinkMode{sso.LinkModeCreate, sso.LinkModeBindOnly} {
		cfg := cfgWith("mock-github", map[string]string{mock.MockExtraBaseURL: "https://idp.test"})
		cfg.LinkMode = mode
		p, err := reg.New(cfg)
		if err != nil {
			t.Fatalf("New (mode=%s): %v", mode, err)
		}
		if p.Name() != "mock-github" {
			t.Fatalf("Name (mode=%s): got %q", mode, p.Name())
		}
	}
}
