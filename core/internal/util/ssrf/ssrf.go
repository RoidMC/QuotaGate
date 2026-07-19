package ssrf

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Policy configures SSRF protection rules.
type Policy struct {
	// AllowedSchemes is the list of permitted URL schemes.
	// Defaults to ["http", "https"] if nil.
	AllowedSchemes []string

	// MaxRedirects limits the number of HTTP redirects to follow.
	// Defaults to 10 if zero.
	MaxRedirects int

	// AllowPrivate permits requests to RFC 1918 private addresses.
	AllowPrivate bool

	// AllowLoopback permits requests to loopback addresses (127.0.0.1, ::1).
	AllowLoopback bool

	// AllowLinkLocal permits requests to link-local addresses (169.254.0.0/16).
	AllowLinkLocal bool

	// DialTimeout is the timeout for establishing a TCP connection.
	// Defaults to 10s if zero. Used by DialContext.
	DialTimeout time.Duration
}

// DefaultPolicy returns a Policy with safe defaults:
//   - Only http/https schemes
//   - Max 10 redirects
//   - Blocks private, loopback, and link-local IPs
//   - 10s dial timeout
func DefaultPolicy() *Policy {
	return &Policy{
		AllowedSchemes: []string{"http", "https"},
		MaxRedirects:   10,
		AllowPrivate:   false,
		AllowLoopback:  false,
		AllowLinkLocal: false,
		DialTimeout:    10 * time.Second,
	}
}

// ValidateURL checks whether rawURL conforms to the Policy.
// It validates scheme, host presence, and IP restrictions.
//
// Note: ValidateURL alone does NOT protect against DNS rebinding.
// Use DialContext for connection-time IP validation.
func (p *Policy) ValidateURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL: %w", err)
	}
	if u.Scheme == "" {
		return fmt.Errorf("URL scheme is missing")
	}
	if !p.isAllowedScheme(u.Scheme) {
		return fmt.Errorf("unsupported URL scheme %q", u.Scheme)
	}
	if u.Host == "" {
		return fmt.Errorf("URL host is missing")
	}

	host := u.Hostname()
	if ip := net.ParseIP(host); ip != nil {
		if p.isBlockedIP(ip) {
			return fmt.Errorf("blocked IP address: %s", ip)
		}
	}

	return nil
}

// CheckRedirect returns an http.CheckRedirect function that enforces
// the Policy on every redirect hop.
func (p *Policy) CheckRedirect() func(req *http.Request, via []*http.Request) error {
	max := p.MaxRedirects
	if max <= 0 {
		max = 10
	}
	return func(req *http.Request, via []*http.Request) error {
		if len(via) >= max {
			return http.ErrUseLastResponse
		}
		if req.URL == nil {
			return http.ErrUseLastResponse
		}
		if !p.isAllowedScheme(req.URL.Scheme) {
			return http.ErrUseLastResponse
		}
		host := req.URL.Hostname()
		if ip := net.ParseIP(host); ip != nil {
			if p.isBlockedIP(ip) {
				return http.ErrUseLastResponse
			}
		}
		return nil
	}
}

// DialContext returns a net.Dialer-compatible function that validates the
// resolved IP at connection time. This is the ONLY way to fully prevent DNS
// rebinding attacks — ValidateURL and CheckRedirect both check the IP before
// the actual connection, so a malicious DNS server can return a safe IP for
// the pre-check and a private IP for the actual connection.
//
// Usage:
//
//	transport := &http.Transport{
//	    DialContext: policy.DialContext(),
//	}
//	client := &http.Client{Transport: transport}
func (p *Policy) DialContext() func(ctx context.Context, network, addr string) (net.Conn, error) {
	timeout := p.DialTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	dialer := &net.Dialer{
		Timeout:       timeout,
		FallbackDelay: 300 * time.Millisecond,
	}
	return func(ctx context.Context, network, addr string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			return nil, fmt.Errorf("ssrf: invalid address %q: %w", addr, err)
		}

		// If host is already an IP, validate it directly.
		if ip := net.ParseIP(host); ip != nil {
			if p.isBlockedIP(ip) {
				return nil, fmt.Errorf("ssrf: blocked IP %s", ip)
			}
			return dialer.DialContext(ctx, network, addr)
		}

		// Resolve the hostname and validate ALL resolved IPs.
		// If any IP is blocked, reject the connection.
		ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("ssrf: failed to resolve %q: %w", host, err)
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("ssrf: no IP addresses for %q", host)
		}
		for _, ip := range ips {
			if p.isBlockedIP(ip.IP) {
				return nil, fmt.Errorf("ssrf: blocked IP %s (resolved from %s)", ip.IP, host)
			}
		}

		// Dial the first resolved IP directly. This prevents the OS resolver
		// from re-resolving and returning a different (potentially blocked) IP.
		target := net.JoinHostPort(ips[0].IP.String(), port)
		return dialer.DialContext(ctx, network, target)
	}
}

// NewHTTPTransport returns an *http.Transport pre-configured with the Policy's
// DialContext. This is the recommended way to create a transport for outbound
// HTTP clients that need SSRF protection.
func (p *Policy) NewHTTPTransport() *http.Transport {
	return &http.Transport{
		DialContext: p.DialContext(),
		// Connection pooling for performance.
		MaxIdleConns:          100,
		MaxIdleConnsPerHost:   10,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
}

// NewHTTPClient returns an *http.Client pre-configured with SSRF protection:
//   - DialContext validates IP at connection time (prevents DNS rebinding)
//   - CheckRedirect validates each redirect hop
//   - Timeout is set to the given value
func (p *Policy) NewHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Transport:     p.NewHTTPTransport(),
		Timeout:       timeout,
		CheckRedirect: p.CheckRedirect(),
	}
}

// isAllowedScheme reports whether scheme is in AllowedSchemes.
func (p *Policy) isAllowedScheme(scheme string) bool {
	schemes := p.AllowedSchemes
	if len(schemes) == 0 {
		schemes = []string{"http", "https"}
	}
	for _, s := range schemes {
		if strings.EqualFold(s, scheme) {
			return true
		}
	}
	return false
}

// isBlockedIP reports whether ip is blocked by the Policy.
func (p *Policy) isBlockedIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if !p.AllowLoopback && ip.IsLoopback() {
		return true
	}
	if !p.AllowLinkLocal && (ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast()) {
		return true
	}
	if !p.AllowPrivate && ip.IsPrivate() {
		return true
	}
	if ip.Equal(net.IPv4zero) {
		return true
	}
	// Block unspecified address (0.0.0.0, ::)
	if ip.IsUnspecified() {
		return true
	}
	return false
}

