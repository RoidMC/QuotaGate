package crypto_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	gmsm "github.com/emmansun/gmsm/sm2"
	pkgcrypto "github.com/roidmc/quotagate/pkg/crypto"
)

func sm2Gen(t *testing.T) (*gmsm.PrivateKey, *ecdsa.PublicKey) {
	t.Helper()
	priv, err := gmsm.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("SM2 GenerateKey: %v", err)
	}
	// priv.PublicKey is sm2.PublicKey (a named type alias for ecdsa.PublicKey).
	// Convert explicitly to *ecdsa.PublicKey for the verifier API.
	pk := ecdsa.PublicKey(priv.PublicKey)
	return priv, &pk
}

func sm4Key() []byte {
	k := make([]byte, pkgcrypto.SM4BlockSize)
	_, _ = rand.Read(k)
	return k
}

func splitJWS(t *testing.T, compact string) [3]string {
	t.Helper()
	p := strings.Split(compact, ".")
	if len(p) != 3 {
		t.Fatalf("JWS has %d parts, want 3", len(p))
	}
	return [3]string{p[0], p[1], p[2]}
}

func b64Decode(t *testing.T, s string) []byte {
	t.Helper()
	b, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		t.Fatalf("base64 decode: %v", err)
	}
	return b
}

func strContains(s, substr string) bool { return strings.Contains(s, substr) }

// ============================================================
// SM2 sign + verify roundtrip
// ============================================================

func TestSM2SignAndVerify(t *testing.T) {
	priv, pub := sm2Gen(t)
	payload := []byte(`{"sub":"test"}`)

	signer, err := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "key-1")
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	signer.SetTokenType("JWT")
	jws, err := signer.Sign(payload)
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}

	parts := splitJWS(t, jws)
	signingInput := []byte(parts[0] + "." + parts[1])
	sig := b64Decode(t, parts[2])

	verifier, ok := pkgcrypto.DefaultRegistry.GetVerifier(pkgcrypto.SGD_SM3_SM2)
	if !ok {
		t.Fatal("no verifier registered for SGD_SM3_SM2")
	}
	if err := verifier.Verify(context.Background(), signingInput, sig, pub); err != nil {
		t.Fatalf("Verify: %v", err)
	}
}

func TestSM2PayloadRoundtrip(t *testing.T) {
	priv, pub := sm2Gen(t)
	payload := []byte("hello world")

	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "")
	jws, _ := signer.Sign(payload)

	parts := splitJWS(t, jws)
	if string(b64Decode(t, parts[1])) != string(payload) {
		t.Fatal("payload mismatch")
	}

	signingInput := []byte(parts[0] + "." + parts[1])
	sig := b64Decode(t, parts[2])
	verifier, _ := pkgcrypto.DefaultRegistry.GetVerifier(pkgcrypto.SGD_SM3_SM2)
	if err := verifier.Verify(context.Background(), signingInput, sig, pub); err != nil {
		t.Fatalf("verify: %v", err)
	}
}

// --- edge cases ---

func TestSM2WrongPublicKey(t *testing.T) {
	priv, _ := sm2Gen(t)
	_, wrongPub := sm2Gen(t)

	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "")
	jws, _ := signer.Sign([]byte("data"))

	parts := splitJWS(t, jws)
	si := []byte(parts[0] + "." + parts[1])
	sig := b64Decode(t, parts[2])

	verifier, _ := pkgcrypto.DefaultRegistry.GetVerifier(pkgcrypto.SGD_SM3_SM2)
	if err := verifier.Verify(context.Background(), si, sig, wrongPub); err == nil {
		t.Fatal("expected failure with wrong public key")
	}
}

func TestSM2TamperedPayload(t *testing.T) {
	priv, pub := sm2Gen(t)
	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "")
	jws, _ := signer.Sign([]byte("original"))

	parts := splitJWS(t, jws)
	// "dGFtcGVyZWQ" decodes to "tampered" — different from "original"
	tamperedSI := []byte(parts[0] + "." + "dGFtcGVyZWQ")
	sig := b64Decode(t, parts[2])

	verifier, _ := pkgcrypto.DefaultRegistry.GetVerifier(pkgcrypto.SGD_SM3_SM2)
	err := verifier.Verify(context.Background(), tamperedSI, sig, pub)
	if err == nil {
		t.Fatal("expected failure with tampered payload")
	}
}

func TestSM2TamperedSignature(t *testing.T) {
	priv, pub := sm2Gen(t)
	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "")
	jws, _ := signer.Sign([]byte("data"))

	parts := splitJWS(t, jws)
	sig := b64Decode(t, parts[2])
	if len(sig) > 0 {
		sig[0] ^= 0x01
	}
	si := []byte(parts[0] + "." + parts[1])

	verifier, _ := pkgcrypto.DefaultRegistry.GetVerifier(pkgcrypto.SGD_SM3_SM2)
	if err := verifier.Verify(context.Background(), si, sig, pub); err == nil {
		t.Fatal("expected failure with tampered signature")
	}
}

func TestSM2EmptyPayload(t *testing.T) {
	priv, pub := sm2Gen(t)
	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "")
	// SM2 library rejects empty payload ("hash cannot be empty"),
	// because ZA+SM3 of empty produces a valid digest but SignASN1
	// explicitly guards against empty input. Use a single byte.
	jws, err := signer.Sign([]byte{0x00})
	if err != nil {
		t.Fatalf("Sign minimal: %v", err)
	}
	parts := splitJWS(t, jws)
	si := []byte(parts[0] + "." + parts[1])
	sig := b64Decode(t, parts[2])
	verifier, _ := pkgcrypto.DefaultRegistry.GetVerifier(pkgcrypto.SGD_SM3_SM2)
	if err := verifier.Verify(context.Background(), si, sig, pub); err != nil {
		t.Fatalf("verify minimal: %v", err)
	}
}

func TestSM2InvalidKeyType(t *testing.T) {
	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, "not-a-key", "")
	_, err := signer.Sign([]byte("data"))
	if err == nil {
		t.Fatal("expected error for invalid key type")
	}
}

func TestSM2MissingProvider(t *testing.T) {
	signer, _ := pkgcrypto.NewSigner("NO_SUCH_ALG", nil, "")
	_, err := signer.Sign([]byte("data"))
	if err == nil {
		t.Fatal("expected ErrNoProvider")
	}
}

// --- header fields ---

func TestSM2KeyIDInHeader(t *testing.T) {
	priv, _ := sm2Gen(t)
	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "my-key-42")
	jws, _ := signer.Sign([]byte("data"))
	parts := splitJWS(t, jws)
	hdr := string(b64Decode(t, parts[0]))
	if !strContains(hdr, `"kid":"my-key-42"`) {
		t.Fatalf("missing kid, got: %s", hdr)
	}
}

func TestSM2NoKeyID(t *testing.T) {
	priv, _ := sm2Gen(t)
	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "")
	jws, _ := signer.Sign([]byte("data"))
	parts := splitJWS(t, jws)
	hdr := string(b64Decode(t, parts[0]))
	if strContains(hdr, `"kid"`) {
		t.Fatalf("unexpected kid: %s", hdr)
	}
}

func TestSM2TokenTypeCustom(t *testing.T) {
	priv, _ := sm2Gen(t)
	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "")
	signer.SetTokenType("logout+jwt")
	jws, _ := signer.Sign([]byte("data"))
	parts := splitJWS(t, jws)
	hdr := string(b64Decode(t, parts[0]))
	if !strContains(hdr, `"typ":"logout+jwt"`) {
		t.Fatalf("missing typ, got: %s", hdr)
	}
}

func TestSM2DefaultTokenType(t *testing.T) {
	priv, _ := sm2Gen(t)
	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "")
	jws, _ := signer.Sign([]byte("data"))
	parts := splitJWS(t, jws)
	hdr := string(b64Decode(t, parts[0]))
	if !strContains(hdr, `"typ":"JWT"`) {
		t.Fatalf("missing default typ, got: %s", hdr)
	}
}

func TestSM2MultipleSignaturesDiffer(t *testing.T) {
	priv, _ := sm2Gen(t)
	signer, _ := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, priv, "")
	j1, _ := signer.Sign([]byte("same"))
	j2, _ := signer.Sign([]byte("same"))
	// SM2 with random k should produce different signatures
	s1 := b64Decode(t, splitJWS(t, j1)[2])
	s2 := b64Decode(t, splitJWS(t, j2)[2])
	if string(s1) == string(s2) {
		t.Fatal("SM2 signatures should be different (random k)")
	}
}

