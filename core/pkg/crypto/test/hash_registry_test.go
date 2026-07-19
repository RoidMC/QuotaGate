package crypto_test

import (
	"context"
	"testing"

	pkgcrypto "github.com/roidmc/quotagate/pkg/crypto"
)

func TestSM3HashString(t *testing.T) {
	h := pkgcrypto.SM3HashString("hello")
	if len(h) != 32 {
		t.Fatalf("SM3 hash should be 32 bytes, got %d", len(h))
	}
}

func TestSM3HashStringHex(t *testing.T) {
	hex := pkgcrypto.SM3HashStringHex("hello")
	if len(hex) != 64 {
		t.Fatalf("SM3 hex should be 64 chars, got %d", len(hex))
	}
}

func TestSM3Deterministic(t *testing.T) {
	h1 := pkgcrypto.SM3HashString("test")
	h2 := pkgcrypto.SM3HashString("test")
	if string(h1) != string(h2) {
		t.Fatal("SM3 should be deterministic")
	}
}

func TestSM3DifferentInputs(t *testing.T) {
	h1 := pkgcrypto.SM3HashString("a")
	h2 := pkgcrypto.SM3HashString("b")
	if string(h1) == string(h2) {
		t.Fatal("different inputs should produce different hashes")
	}
}

func TestSM3EmptyString(t *testing.T) {
	h := pkgcrypto.SM3HashString("")
	if len(h) != 32 {
		t.Fatal("empty string should still produce 32-byte hash")
	}
}

// ============================================================
// Registry override (HSM simulation)
// ============================================================

type mockSignProvider struct {
	alg string
}

func (m mockSignProvider) Algorithm() string { return m.alg }
func (m mockSignProvider) Sign(ctx context.Context, keyID, tokenType string, key interface{}, payload []byte) (string, error) {
	// All SM2 keys pass through; just return a mock JWS
	return "mock.hdr.mockpayload.mocksig", nil
}

type mockVerifyProvider struct {
	alg        string
	shouldPass bool
}

func (m mockVerifyProvider) Algorithm() string { return m.alg }
func (m mockVerifyProvider) Verify(ctx context.Context, signingInput, signature []byte, key interface{}) error {
	if !m.shouldPass {
		return &verifyError{"mock reject"}
	}
	return nil
}

type verifyError struct{ msg string }

func (e *verifyError) Error() string { return e.msg }

type mockContentEncProvider struct{}

func (m mockContentEncProvider) Algorithm() string { return pkgcrypto.SGD_SM4_GCM }
func (m mockContentEncProvider) Encrypt(ctx context.Context, key, iv, plaintext, aad []byte) ([]byte, error) {
	// Return plaintext with a mock tag appended
	tag := make([]byte, 16)
	copy(tag, "MOCKMOCKMOCKMOCK")
	return append(plaintext, tag...), nil
}

type mockContentDecProvider struct{}

func (m mockContentDecProvider) Algorithm() string { return pkgcrypto.SGD_SM4_GCM }
func (m mockContentDecProvider) Decrypt(ctx context.Context, key, iv, sealed, aad []byte) ([]byte, error) {
	if len(sealed) < 16 {
		return nil, &verifyError{"too short"}
	}
	return sealed[:len(sealed)-16], nil
}

func TestRegistryOverrideSigner(t *testing.T) {
	reg := pkgcrypto.NewProviderRegistry()
	reg.RegisterSigner(pkgcrypto.SGD_SM3_SM2, mockSignProvider{alg: pkgcrypto.SGD_SM3_SM2})

	_, ok := pkgcrypto.DefaultRegistry.GetSigner(pkgcrypto.SGD_SM3_SM2)
	if !ok {
		t.Fatal("DefaultRegistry should have SGD_SM3_SM2 signer from init()")
	}
	_, ok = pkgcrypto.DefaultRegistry.GetVerifier(pkgcrypto.SGD_SM3_SM2)
	if !ok {
		t.Fatal("DefaultRegistry should have SGD_SM3_SM2 verifier from init()")
	}
	_, ok = pkgcrypto.DefaultRegistry.GetContentEncryptor(pkgcrypto.SGD_SM4_GCM)
	if !ok {
		t.Fatal("DefaultRegistry should have SGD_SM4_GCM encryptor from init()")
	}
	_, ok = pkgcrypto.DefaultRegistry.GetContentDecryptor(pkgcrypto.SGD_SM4_GCM)
	if !ok {
		t.Fatal("DefaultRegistry should have SGD_SM4_GCM decryptor from init()")
	}
}

func TestNewProviderRegistryEmpty(t *testing.T) {
	reg := pkgcrypto.NewProviderRegistry()
	_, ok := reg.GetSigner(pkgcrypto.SGD_SM3_SM2)
	if ok {
		t.Fatal("new registry should be empty")
	}
}

func TestProviderRegistryLastWins(t *testing.T) {
	reg := pkgcrypto.NewProviderRegistry()
	reg.RegisterSigner("A", mockSignProvider{alg: "A"})
	reg.RegisterSigner("A", mockSignProvider{alg: "B"}) // second should win
	p, _ := reg.GetSigner("A")
	if p.Algorithm() != "B" {
		t.Fatal("last registration should win")
	}
}
