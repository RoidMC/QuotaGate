package ssrf

import (
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
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
}

// DefaultPolicy returns a Policy with safe defaults:
//   - Only http/https schemes
//   - Max 10 redirects
//   - Blocks private, loopback, and link-local IPs
func DefaultPolicy() *Policy {
	return &Policy{
		AllowedSchemes: []string{"http", "https"},
		MaxRedirects:   10,
		AllowPrivate:   false,
		AllowLoopback:  false,
		AllowLinkLocal: false,
	}
}

// ValidateURL checks whether rawURL conforms to the Policy.
// It validates scheme, host presence, and IP restrictions.
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
	return false
}
