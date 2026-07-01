// Package event provides webhook event signing and verification utilities.
//
// The signing scheme follows the timestamp-prefixed HMAC pattern used by
// Stripe, GitHub, and Slack webhooks to prevent replay attacks:
//
//	signed_content = timestamp + "." + payload
//	signature      = HMAC(secret, signed_content)
//	header         = "t=<unix_timestamp>,<version>=<hex_signature>"
//
// Multiple hash algorithms are supported via the Signer struct:
//   - SHA-256 (default, version tag "v1")
//   - SHA-384 (version tag "v384")
//   - SHA-512 (version tag "v512")
//   - SM3     (version tag "sm3", requires gmsm)
//
// Verification checks both the signature validity and that the timestamp
// falls within a configurable tolerance window (default 5 minutes).
package event

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"strconv"
	"strings"
	"time"

	"github.com/emmansun/gmsm/sm3"
)

// SignatureHeader is the HTTP header key used to carry the webhook signature.
const SignatureHeader = "X-Webhook-Signature"

// DefaultTolerance is the default time window within which a webhook
// timestamp is considered valid. Signatures with timestamps outside
// this window are rejected to prevent replay attacks.
const DefaultTolerance = 5 * time.Minute

// SignatureSep separates the timestamp from the payload in the signed content.
const SignatureSep = "."

const (
	headerFieldSep    = ","
	headerKeyValueSep = "="
)

// HashAlgorithm identifies the hash function used for HMAC signing.
type HashAlgorithm string

const (
	// HashSHA256 uses HMAC-SHA256. This is the default and most widely
	// compatible option. Header version tag: "v1".
	HashSHA256 HashAlgorithm = "sha256"
	// HashSHA384 uses HMAC-SHA384. Header version tag: "v384".
	HashSHA384 HashAlgorithm = "sha384"
	// HashSHA512 uses HMAC-SHA512. Header version tag: "v512".
	HashSHA512 HashAlgorithm = "sha512"
	// HashSM3 uses HMAC-SM3 (Chinese national cryptography standard GM/T 0004).
	// Header version tag: "sm3".
	HashSM3 HashAlgorithm = "sm3"
)

// algorithmInfo maps each HashAlgorithm to its hash factory and header version tag.
type algorithmInfo struct {
	newHash func() hash.Hash
	version string
}

var algorithmRegistry = map[HashAlgorithm]algorithmInfo{
	HashSHA256: {newHash: sha256.New, version: "v1"},
	HashSHA384: {newHash: sha512.New384, version: "v384"},
	HashSHA512: {newHash: sha512.New, version: "v512"},
	HashSM3:    {newHash: sm3.New, version: "sm3"},
}

var (
	// ErrInvalidSignature is returned when the computed signature does not match
	// the one provided in the header.
	ErrInvalidSignature = errors.New("quotagate/event: invalid webhook signature")
	// ErrEmptySecret is returned when an empty secret is passed to Sign or Verify.
	ErrEmptySecret = errors.New("quotagate/event: secret must not be empty")
	// ErrTimestampExpired is returned when the webhook timestamp falls outside
	// the allowed tolerance window.
	ErrTimestampExpired = errors.New("quotagate/event: webhook timestamp outside tolerance")
	// ErrMalformedHeader is returned when the signature header cannot be parsed.
	ErrMalformedHeader = errors.New("quotagate/event: malformed signature header")
	// ErrUnknownAlgorithm is returned when an unsupported HashAlgorithm is
	// passed to NewSigner.
	ErrUnknownAlgorithm = errors.New("quotagate/event: unknown hash algorithm")
)

// SignatureResult holds the outputs of a signing operation.
type SignatureResult struct {
	// Timestamp is the Unix timestamp included in the signature.
	Timestamp int64
	// Signature is the hex-encoded HMAC digest.
	Signature string
	// Header is the fully formatted value for the X-Webhook-Signature header,
	// in the form "t=<ts>,<version>=<sig>".
	Header string
}

// Signer computes and verifies webhook signatures using a configurable
// hash algorithm. Create one with NewSigner.
type Signer struct {
	alg     HashAlgorithm
	newHash func() hash.Hash
	version string
}

// NewSigner creates a Signer that uses the specified hash algorithm.
// Pass HashSHA256 for the standard HMAC-SHA256 scheme, or HashSM3 for
// Chinese national cryptography compliance.
func NewSigner(alg HashAlgorithm) (*Signer, error) {
	info, ok := algorithmRegistry[alg]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrUnknownAlgorithm, alg)
	}
	return &Signer{
		alg:     alg,
		newHash: info.newHash,
		version: info.version,
	}, nil
}

// Algorithm returns the hash algorithm used by this Signer.
func (s *Signer) Algorithm() HashAlgorithm {
	return s.alg
}

// Version returns the header version tag used by this Signer
// (e.g. "v1" for SHA-256, "sm3" for SM3).
func (s *Signer) Version() string {
	return s.version
}

// SignPayload computes an HMAC signature over the concatenation of the
// current timestamp and the payload, returning a SignatureResult whose
// Header field should be set as the value of the X-Webhook-Signature HTTP header.
//
// The signed content format is: "<timestamp>.<payload>"
// The header format is:        "t=<timestamp>,<version>=<hex_signature>"
func (s *Signer) SignPayload(payload []byte, secret string, now time.Time) (*SignatureResult, error) {
	if secret == "" {
		return nil, ErrEmptySecret
	}

	ts := now.Unix()
	mac := hmac.New(s.newHash, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d%s", ts, SignatureSep)))
	mac.Write(payload)
	sig := hex.EncodeToString(mac.Sum(nil))

	header := fmt.Sprintf("t=%d,%s=%s", ts, s.version, sig)

	return &SignatureResult{
		Timestamp: ts,
		Signature: sig,
		Header:    header,
	}, nil
}

// VerifySignature validates a webhook signature header against the given
// payload and secret. It performs two checks:
//
//  1. The timestamp in the header must be within tolerance of the current time.
//  2. The HMAC signature must match the recomputed value using this Signer's
//     algorithm and version tag.
//
// Returns nil on success, or a specific error (ErrEmptySecret,
// ErrInvalidSignature, ErrTimestampExpired, ErrMalformedHeader) on failure.
func (s *Signer) VerifySignature(payload []byte, secret string, header string, tolerance time.Duration) error {
	if secret == "" {
		return ErrEmptySecret
	}
	if header == "" {
		return ErrInvalidSignature
	}

	ts, sig, err := parseSignatureHeader(header, s.version)
	if err != nil {
		return err
	}

	now := time.Now().Unix()
	diff := now - ts
	if diff < 0 {
		diff = -diff
	}
	if diff > int64(tolerance/time.Second) {
		return ErrTimestampExpired
	}

	mac := hmac.New(s.newHash, []byte(secret))
	mac.Write([]byte(fmt.Sprintf("%d%s", ts, SignatureSep)))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(expected), []byte(sig)) {
		return ErrInvalidSignature
	}

	return nil
}

// parseSignatureHeader extracts the timestamp and signature from a header
// string in the format "t=<timestamp>,<version>=<signature>".
// The version parameter specifies which key to look for the signature value.
func parseSignatureHeader(header string, version string) (int64, string, error) {
	parts := strings.Split(header, headerFieldSep)
	var tsStr, sigStr string

	for _, part := range parts {
		kv := strings.SplitN(part, headerKeyValueSep, 2)
		if len(kv) != 2 {
			return 0, "", ErrMalformedHeader
		}
		switch kv[0] {
		case "t":
			tsStr = kv[1]
		case version:
			sigStr = kv[1]
		}
	}

	if tsStr == "" || sigStr == "" {
		return 0, "", ErrMalformedHeader
	}

	ts, err := strconv.ParseInt(tsStr, 10, 64)
	if err != nil {
		return 0, "", ErrMalformedHeader
	}

	return ts, sigStr, nil
}

// DefaultSigner is a package-level Signer using HMAC-SHA256.
// It is used by the package-level convenience functions SignPayload
// and VerifySignature.
var DefaultSigner, _ = NewSigner(HashSHA256)

// SignPayload is a convenience wrapper around DefaultSigner.SignPayload.
// For custom hash algorithms, create a Signer with NewSigner instead.
func SignPayload(payload []byte, secret string, now time.Time) (*SignatureResult, error) {
	return DefaultSigner.SignPayload(payload, secret, now)
}

// VerifySignature is a convenience wrapper around DefaultSigner.VerifySignature.
// For custom hash algorithms, create a Signer with NewSigner instead.
func VerifySignature(payload []byte, secret string, header string, tolerance time.Duration) error {
	return DefaultSigner.VerifySignature(payload, secret, header, tolerance)
}
