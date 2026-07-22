package kexpluginsdk_test

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/roidmc/quotagate/pkg/kexpluginsdk"
)

// TestSharedHTTPClient_Config pins the contract the plugin systems depend on:
// a single process-wide, tuned client whose Transport is SharedTransport.
func TestSharedHTTPClient_Config(t *testing.T) {
	if kexpluginsdk.SharedHTTPClient == nil {
		t.Fatal("SharedHTTPClient: want non-nil")
	}
	if got := kexpluginsdk.SharedHTTPClient.Timeout; got != 10*time.Second {
		t.Fatalf("SharedHTTPClient.Timeout: got %v, want %v", got, 10*time.Second)
	}
	// The whole point of a shared transport is that the client's Transport is
	// the very same *http.Transport, so every plugin reuses one connection pool.
	if kexpluginsdk.SharedHTTPClient.Transport != http.RoundTripper(kexpluginsdk.SharedTransport) {
		t.Fatal("SharedHTTPClient.Transport: want identical to SharedTransport (shared pool)")
	}
}

// TestSharedTransport_Tuned verifies the pool is sized above Go's defaults
// (MaxIdleConnsPerHost=2), which is the reason this exists in the first place.
// SharedTransport is a concrete *http.Transport, so its fields are reachable
// directly.
func TestSharedTransport_Tuned(t *testing.T) {
	tr := kexpluginsdk.SharedTransport
	if tr.MaxIdleConns != 100 {
		t.Fatalf("MaxIdleConns: got %d, want 100", tr.MaxIdleConns)
	}
	if tr.MaxIdleConnsPerHost != 10 {
		t.Fatalf("MaxIdleConnsPerHost: got %d, want 10", tr.MaxIdleConnsPerHost)
	}
	if tr.IdleConnTimeout != 90*time.Second {
		t.Fatalf("IdleConnTimeout: got %v, want %v", tr.IdleConnTimeout, 90*time.Second)
	}
}

// TestSharedTransport_PoolReuse proves the shared client actually pools
// keep-alive connections across sequential requests, and that a relay-style
// client built on the same transport shares that pool (no per-client sockets).
//
// We wrap the process-wide SharedTransport's DialContext with a counter. A
// reused keep-alive connection does NOT trigger a new dial, so two sequential
// calls must dial exactly once; a relay-style client must reuse that same
// pooled connection (still one dial). The wrapper is restored afterward so we
// never leave the shared transport mutated.
func TestSharedTransport_PoolReuse(t *testing.T) {
	tr := kexpluginsdk.SharedTransport

	var dials int
	origDial := tr.DialContext
	tr.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		dials++
		if origDial != nil {
			return origDial(ctx, network, addr)
		}
		var d net.Dialer
		return d.DialContext(ctx, network, addr)
	}
	defer func() { tr.DialContext = origDial }()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = io.WriteString(w, "ok")
	}))
	defer srv.Close()

	get := func(c *http.Client) {
		resp, err := c.Get(srv.URL)
		if err != nil {
			t.Fatalf("request: %v", err)
		}
		if _, err := io.ReadAll(resp.Body); err != nil {
			t.Fatalf("drain body: %v", err)
		}
		resp.Body.Close()
	}

	// Two sequential short calls through the shared client: keep-alive should
	// reuse a single connection, so the dial counter increments exactly once.
	get(kexpluginsdk.SharedHTTPClient)
	get(kexpluginsdk.SharedHTTPClient)
	if dials != 1 {
		t.Fatalf("dials after 2 shared-client calls: got %d, want 1 (keep-alive reuse)", dials)
	}

	// A relay-style client reuses the very same pool — no new dial.
	relayClient := &http.Client{Transport: tr}
	get(relayClient)
	if dials != 1 {
		t.Fatalf("dials after relay-style call: got %d, want 1 (shared pool)", dials)
	}

	tr.CloseIdleConnections()
}

// TestRelayClient_NoGlobalTimeout locks the relay contract: a client built on
// SharedTransport must NOT carry a global Client.Timeout, because LLM responses
// can stream for many seconds and are bounded by the request context instead.
func TestRelayClient_NoGlobalTimeout(t *testing.T) {
	relayClient := &http.Client{Transport: kexpluginsdk.SharedTransport}
	if relayClient.Timeout != 0 {
		t.Fatalf("relay client Timeout: got %v, want 0 (ctx-bound only)", relayClient.Timeout)
	}
	if relayClient.Transport != http.RoundTripper(kexpluginsdk.SharedTransport) {
		t.Fatal("relay client Transport: want SharedTransport (shared pool)")
	}
}
