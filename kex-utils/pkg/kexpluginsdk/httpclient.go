// Package kexpluginsdk holds the generic plugin-registry skeleton shared by
// KexCore Universal Plugin System. It deliberately knows nothing about any
// specific plugin: it only manages registration and lookup of factories, and
// here also exposes small, shared, concurrency-safe primitives that every
// plugin system (captcha, sso, relay, ...) may reuse, so each plugin doesn't
// reinvent the same outbound-HTTP infrastructure on its own.
//
// Why a shared transport/client instead of one per plugin:
//   - An http.Client (and its underlying Transport) keeps a pool of idle
//     keep-alive connections. The Go default transport caps that pool at
//     MaxIdleConnsPerHost = 2, which thrashes under concurrent load. A single
//     process-wide tuned transport lets every provider reuse one pool.
//   - http.Client is safe for concurrent use, so sharing one instance across
//     all providers and goroutines is correct — no need to allocate one per
//     request or per provider instance.
//   - Centralizing the config means one place to reason about timeouts and
//     pool sizing, rather than each plugin hand-rolling a client (some with no
//     timeout at all, as relay used to).
package kexpluginsdk

import (
	"net/http"
	"time"
)

// SharedTransport is a process-wide, concurrency-safe *http.Transport with a
// tuned connection pool. It is the common pipe for every plugin's outbound
// HTTP, so keep-alive connections are reused across all providers and
// goroutines.
//
// It is exported separately from SharedHTTPClient so callers that must NOT
// impose a global Client.Timeout — e.g. relay providers, whose upstream LLM
// responses can stream for many seconds and are already bounded by the request
// context — can build their own http.Client around this transport.
var SharedTransport = newSharedTransport()

// SharedHTTPClient is a process-wide, concurrency-safe *http.Client built on
// SharedTransport with a 10s Timeout. Use it for short outbound calls (captcha
// siteverify, OIDC discovery/userinfo/jwks) where a global deadline is
// desirable. Every provider instance may share this single client.
var SharedHTTPClient = &http.Client{
	Timeout:   10 * time.Second,
	Transport: SharedTransport,
}

func newSharedTransport() *http.Transport {
	return &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		IdleConnTimeout:     90 * time.Second,
	}
}
