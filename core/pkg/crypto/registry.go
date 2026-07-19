package crypto

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"sync"

	gmsm "github.com/emmansun/gmsm/sm2"
)

// ErrNoProvider is returned when no provider is registered for an algorithm.
type ErrNoProvider struct{ Algorithm string }

func (e *ErrNoProvider) Error() string { return fmt.Sprintf("pkg/crypto: no provider for %s", e.Algorithm) }

// ProviderRegistry holds registered cryptographic providers.
type ProviderRegistry struct {
	mu         sync.RWMutex
	signers    map[string]SignProvider
	verifiers  map[string]VerifyProvider
	contentEnc map[string]ContentEncryptProvider
	contentDec map[string]ContentDecryptProvider
}

// NewProviderRegistry creates an empty registry.
func NewProviderRegistry() *ProviderRegistry {
	return &ProviderRegistry{
		signers:    make(map[string]SignProvider),
		verifiers:  make(map[string]VerifyProvider),
		contentEnc: make(map[string]ContentEncryptProvider),
		contentDec: make(map[string]ContentDecryptProvider),
	}
}

// DefaultRegistry is the global provider registry. Built-in gmsm providers
// are registered in init(); HSM/KMS vendors override them in their own init().
var DefaultRegistry = NewProviderRegistry()

func (r *ProviderRegistry) RegisterSigner(alg string, p SignProvider) {
	r.mu.Lock(); defer r.mu.Unlock()
	r.signers[alg] = p
}
func (r *ProviderRegistry) GetSigner(alg string) (SignProvider, bool) {
	r.mu.RLock(); defer r.mu.RUnlock()
	p, ok := r.signers[alg]
	return p, ok
}

func (r *ProviderRegistry) RegisterVerifier(alg string, p VerifyProvider) {
	r.mu.Lock(); defer r.mu.Unlock()
	r.verifiers[alg] = p
}
func (r *ProviderRegistry) GetVerifier(alg string) (VerifyProvider, bool) {
	r.mu.RLock(); defer r.mu.RUnlock()
	p, ok := r.verifiers[alg]
	return p, ok
}

func (r *ProviderRegistry) RegisterContentEncryptor(alg string, p ContentEncryptProvider) {
	r.mu.Lock(); defer r.mu.Unlock()
	r.contentEnc[alg] = p
}
func (r *ProviderRegistry) GetContentEncryptor(alg string) (ContentEncryptProvider, bool) {
	r.mu.RLock(); defer r.mu.RUnlock()
	p, ok := r.contentEnc[alg]
	return p, ok
}

func (r *ProviderRegistry) RegisterContentDecryptor(alg string, p ContentDecryptProvider) {
	r.mu.Lock(); defer r.mu.Unlock()
	r.contentDec[alg] = p
}
func (r *ProviderRegistry) GetContentDecryptor(alg string) (ContentDecryptProvider, bool) {
	r.mu.RLock(); defer r.mu.RUnlock()
	p, ok := r.contentDec[alg]
	return p, ok
}

// --- Built-in gmsm providers ---

// sm2SignProvider implements SM2+SM3 JWS signing per GB/T 32918.2-2016.
//
// IMPORTANT: We use sm2.SignASN1WithSM2 / VerifyASN1WithSM2 which perform the
// FULL SM2 pipeline internally: ZA precomputation (based on public key + UID)
// + SM3(ZA||msg) + ECDSA-on-SM2-P256. Callers pass the raw signing input
// (header.payload for JWS), NOT a pre-computed hash.
//
// The UID parameter is nil = use default "1234567812345678" per GB/T 32918.
//
// WARNING: Do NOT use sm2.SignASN1/VerifyASN1 for SM2 — those treat the input
// as a pre-computed hash digest and skip ZA, producing non-standard signatures
// that will not interoperate with other SM2 implementations.
type sm2SignProvider struct{}

func (sm2SignProvider) Algorithm() string { return SGD_SM3_SM2 }
func (sm2SignProvider) Sign(ctx context.Context, keyID, tokenType string, key interface{}, payload []byte) (string, error) {
	priv, ok := key.(*gmsm.PrivateKey)
	if !ok {
		return "", fmt.Errorf("sm2SignProvider: expected *sm2.PrivateKey, got %T", key)
	}

	// Build JWS protected header first — the signing input is header.payload
	// (per RFC 7515 §5.1), NOT the raw payload.
	header := map[string]interface{}{"alg": SGD_SM3_SM2, "typ": tokenType}
	if keyID != "" {
		header["kid"] = keyID
	}
	headerJSON, _ := json.Marshal(header)
	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := headerB64 + "." + payloadB64

	// priv.Sign with DefaultSM2SignerOpts performs full SM2:
	// ZA precomputation + SM3(ZA||msg) + ECDSA-on-SM2-P256.
	// DefaultSM2SignerOpts = NewSM2SignerOption(true, nil) where forceGMSign=true
	// (treat input as raw msg, do ZA+SM3 internally) and uid=nil (use default
	// "1234567812345678" per GB/T 32918).
	sig, err := priv.Sign(rand.Reader, []byte(signingInput), gmsm.DefaultSM2SignerOpts)
	if err != nil {
		return "", fmt.Errorf("SM2 sign: %w", err)
	}

	return headerB64 + "." + payloadB64 + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// sm2VerifyProvider implements SM2+SM3 JWS verification per GB/T 32918.2-2016.
//
// The signing input is header.payload (JWS compact form). We pass it to
// VerifyASN1WithSM2 which does ZA + SM3(ZA||signingInput) + verify internally.
type sm2VerifyProvider struct{}

func (sm2VerifyProvider) Algorithm() string { return SGD_SM3_SM2 }
func (sm2VerifyProvider) Verify(ctx context.Context, signingInput, signature []byte, key interface{}) error {
	pub, ok := key.(*ecdsa.PublicKey)
	if !ok {
		return fmt.Errorf("sm2VerifyProvider: expected *ecdsa.PublicKey, got %T", key)
	}
	// VerifyASN1WithSM2 does ZA+SM3 internally; pass the raw signing input.
	// uid=nil uses default "1234567812345678" per GB/T 32918.
	if !gmsm.VerifyASN1WithSM2(pub, nil, signingInput, signature) {
		return fmt.Errorf("SM2 signature verification failed")
	}
	return nil
}

// --- Built-in content encryption provider (SM4-GCM via gmsm/sm4 + crypto/cipher) ---

type sm4GCMContentProvider struct{}

func (sm4GCMContentProvider) Algorithm() string { return SGD_SM4_GCM }
func (sm4GCMContentProvider) Encrypt(ctx context.Context, key, iv, plaintext, aad []byte) ([]byte, error) {
	return sm4GCMEncrypt(key, iv, plaintext, aad)
}
func (sm4GCMContentProvider) Decrypt(ctx context.Context, key, iv, sealed, aad []byte) ([]byte, error) {
	return sm4GCMDecrypt(key, iv, sealed, aad)
}

func init() {
	DefaultRegistry.RegisterSigner(SGD_SM3_SM2, sm2SignProvider{})
	DefaultRegistry.RegisterVerifier(SGD_SM3_SM2, sm2VerifyProvider{})
	DefaultRegistry.RegisterContentEncryptor(SGD_SM4_GCM, sm4GCMContentProvider{})
	DefaultRegistry.RegisterContentDecryptor(SGD_SM4_GCM, sm4GCMContentProvider{})
}

