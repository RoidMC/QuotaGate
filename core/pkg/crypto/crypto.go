// Package crypto provides cryptographic primitives with a provider registry
// pattern. Built-in software implementations are registered in init(); HSM/KMS
// vendors override them by registering their own providers (last wins).
//
// Architecture (mirrors kexcore-oidc crypto + kexswiftdb/bus pattern):
//
//	Interface  →  ProviderRegistry  →  Built-in (gmsm) or HSM override
//	Consumer   →  Signer / EncryptJWE / DecryptJWE  (dispatch through Registry)
//
// This package is self-contained within pkg/ — it does NOT depend on
// root-level crypto/ (reference implementation) or internal/crypto.
//
// Algorithm identifiers follow GM/T 0125.1:
//
//	SGD_SM3_SM2   SM2+SM3 digital signature
//	SGD_SM4_GCM   SM4-GCM content encryption
package crypto

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"

	"github.com/emmansun/gmsm/sm3"
	"golang.org/x/crypto/argon2"
)

// GM/T 0125.1 algorithm identifiers.
const (
	SGD_SM3_SM2 = "SGD_SM3_SM2" // SM2+SM3 digital signature
	SGD_SM4_GCM = "SGD_SM4_GCM" // SM4-GCM content encryption
)

// SM4 constants.
const (
	SM4BlockSize    = 16
	SM4GCMNonceSize = 12
)

// SignProvider is the interface for JWS signing implementations.
// Built-in software signers and HSM/KMS vendors both implement this.
type SignProvider interface {
	// Algorithm returns the supported JWA signature algorithm.
	Algorithm() string
	// Sign signs the payload and returns compact JWS.
	// key is the signing key material (type depends on algorithm).
	// keyID and tokenType are used for JWS header construction.
	Sign(ctx context.Context, keyID, tokenType string, key interface{}, payload []byte) (string, error)
}

// VerifyProvider is the interface for JWS signature verification.
type VerifyProvider interface {
	Algorithm() string
	// Verify verifies the signature for the given signing input.
	Verify(ctx context.Context, signingInput, signature []byte, key interface{}) error
}

// ContentEncryptProvider is the interface for JWE content encryption.
// Used for "dir" mode. HSM/KMS can override just the symmetric encryption.
type ContentEncryptProvider interface {
	Algorithm() string
	// Encrypt encrypts plaintext with key, IV, and AAD. Returns ciphertext+tag.
	Encrypt(ctx context.Context, key, iv, plaintext, aad []byte) ([]byte, error)
}

// ContentDecryptProvider is the interface for JWE content decryption.
type ContentDecryptProvider interface {
	Algorithm() string
	// Decrypt decrypts sealed (ciphertext+tag) with key, IV, and AAD.
	Decrypt(ctx context.Context, key, iv, sealed, aad []byte) ([]byte, error)
}

// Signer wraps key material and algorithm, dispatching through ProviderRegistry.
// Consumers use Signer.Sign(payload) instead of importing gmsm directly.
type Signer struct {
	algorithm string
	key       interface{}
	keyID     string
	tokenType string
}

// NewSigner creates a Signer for the given algorithm and key.
// Supported: SGD_SM3_SM2 (key: *sm2.PrivateKey from gmsm).
func NewSigner(algorithm string, key interface{}, keyID string) (*Signer, error) {
	return &Signer{algorithm: algorithm, key: key, keyID: keyID}, nil
}

// SetTokenType sets the JWT typ header value (default "JWT").
func (s *Signer) SetTokenType(tokenType string) { s.tokenType = tokenType }

func (s *Signer) tokenTypeOrDefault() string {
	if s.tokenType != "" {
		return s.tokenType
	}
	return "JWT"
}

// Algorithm returns the JWA signature algorithm string.
func (s *Signer) Algorithm() string { return s.algorithm }

// Sign signs the payload through ProviderRegistry.
// HSM/KMS providers registered via init() override the built-in gmsm.
func (s *Signer) Sign(payload []byte) (string, error) {
	if provider, ok := DefaultRegistry.GetSigner(s.algorithm); ok {
		return provider.Sign(context.Background(), s.keyID, s.tokenTypeOrDefault(), s.key, payload)
	}
	return "", &ErrNoProvider{Algorithm: s.algorithm}
}

// --- SM3 hash ---

// SM3HashString returns the SM3 hash of the input string as raw bytes.
func SM3HashString(data string) []byte {
	h := sm3.New()
	h.Write([]byte(data))
	return h.Sum(nil)
}

// SM3HashStringHex returns the SM3 hash of the input string as a hex string.
func SM3HashStringHex(data string) string {
	h := sm3.New()
	h.Write([]byte(data))
	return fmt.Sprintf("%x", h.Sum(nil))
}

// --- Argon2id password hashing ---

// Argon2idParams defines the parameters for Argon2id key derivation.
type Argon2idParams struct {
	Memory      uint32
	Iterations  uint32
	Parallelism uint8
	SaltLength  uint32
	KeyLength   uint32
}

// DefaultArgon2idParams is the recommended parameter set.
var DefaultArgon2idParams = Argon2idParams{
	Memory:      64 * 1024,
	Iterations:  3,
	Parallelism: 2,
	SaltLength:  16,
	KeyLength:   32,
}

// Argon2idHash derives a key from password using Argon2id and returns
// the PHC-format hash string.
func Argon2idHash(password string, p Argon2idParams) (string, error) {
	salt := make([]byte, p.SaltLength)
	if _, err := rand.Read(salt); err != nil {
		return "", err
	}
	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version, p.Memory, p.Iterations, p.Parallelism,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(hash),
	), nil
}

// Argon2idVerify verifies a password against a PHC-format Argon2id hash.
func Argon2idVerify(password, encodedHash string) (bool, error) {
	parts := strings.Split(encodedHash, "$")
	if len(parts) != 6 {
		return false, fmt.Errorf("crypto: invalid argon2id hash format")
	}
	if parts[1] != "argon2id" {
		return false, fmt.Errorf("crypto: unsupported algorithm: %s", parts[1])
	}
	var version int
	if _, err := fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return false, fmt.Errorf("crypto: invalid argon2id version: %w", err)
	}
	p := Argon2idParams{}
	if _, err := fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &p.Memory, &p.Iterations, &p.Parallelism); err != nil {
		return false, fmt.Errorf("crypto: invalid argon2id params: %w", err)
	}
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false, fmt.Errorf("crypto: invalid argon2id salt: %w", err)
	}
	p.SaltLength = uint32(len(salt))
	expected, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false, fmt.Errorf("crypto: invalid argon2id hash: %w", err)
	}
	p.KeyLength = uint32(len(expected))
	hash := argon2.IDKey([]byte(password), salt, p.Iterations, p.Memory, p.Parallelism, p.KeyLength)
	return subtle.ConstantTimeCompare(hash, expected) == 1, nil
}
