package crypto_test

import (
	"crypto/rand"
	"encoding/base64"
	"strings"
	"testing"

	pkgcrypto "github.com/roidmc/kex-utils/pkg/crypto"
)

func TestSM4JWEEncryptDecrypt(t *testing.T) {
	key := sm4Key()
	plaintext := []byte("secret JWE payload")

	jweCompact, err := pkgcrypto.EncryptJWE(plaintext, key)
	if err != nil {
		t.Fatalf("EncryptJWE: %v", err)
	}

	// JWE compact: 5 parts
	parts := strings.Split(jweCompact, ".")
	if len(parts) != 5 {
		t.Fatalf("JWE has %d parts, want 5", len(parts))
	}
	if parts[1] != "" {
		t.Fatal("dir mode: encrypted key must be empty")
	}

	// Decrypt
	decrypted, err := pkgcrypto.DecryptJWE(jweCompact, key)
	if err != nil {
		t.Fatalf("DecryptJWE: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("decrypted: %q, want %q", decrypted, plaintext)
	}
}

func TestSM4JWEWrongKey(t *testing.T) {
	key1 := sm4Key()
	key2 := sm4Key()

	jwe, _ := pkgcrypto.EncryptJWE([]byte("secret"), key1)
	_, err := pkgcrypto.DecryptJWE(jwe, key2)
	if err == nil {
		t.Fatal("expected error with wrong key")
	}
}

func TestSM4JWEInvalidKeySize(t *testing.T) {
	_, err := pkgcrypto.EncryptJWE([]byte("data"), []byte("short"))
	if err == nil {
		t.Fatal("expected error for short key")
	}
}

func TestSM4JWETamperedCiphertext(t *testing.T) {
	key := sm4Key()
	jwe, _ := pkgcrypto.EncryptJWE([]byte("secret"), key)

	parts := strings.Split(jwe, ".")
	// Tamper with ciphertext (part 3)
	ct, _ := base64.RawURLEncoding.DecodeString(parts[3])
	if len(ct) > 0 {
		ct[0] ^= 0xFF
	}
	parts[3] = base64.RawURLEncoding.EncodeToString(ct)
	tampered := strings.Join(parts, ".")

	_, err := pkgcrypto.DecryptJWE(tampered, key)
	if err == nil {
		t.Fatal("expected error with tampered ciphertext")
	}
}

func TestSM4JWETamperedTag(t *testing.T) {
	key := sm4Key()
	jwe, _ := pkgcrypto.EncryptJWE([]byte("secret"), key)

	parts := strings.Split(jwe, ".")
	tag, _ := base64.RawURLEncoding.DecodeString(parts[4])
	if len(tag) > 0 {
		tag[0] ^= 0xFF
	}
	parts[4] = base64.RawURLEncoding.EncodeToString(tag)
	tampered := strings.Join(parts, ".")

	_, err := pkgcrypto.DecryptJWE(tampered, key)
	if err == nil {
		t.Fatal("expected error with tampered tag")
	}
}

func TestSM4JWEMultipleEncryptsDiffer(t *testing.T) {
	key := sm4Key()
	j1, _ := pkgcrypto.EncryptJWE([]byte("same"), key)
	j2, _ := pkgcrypto.EncryptJWE([]byte("same"), key)
	if j1 == j2 {
		t.Fatal("JWE outputs should differ (random IV)")
	}
}

func TestSM4JWEEmptyPayload(t *testing.T) {
	key := sm4Key()
	jwe, err := pkgcrypto.EncryptJWE([]byte{}, key)
	if err != nil {
		t.Fatalf("EncryptJWE empty: %v", err)
	}
	dec, err := pkgcrypto.DecryptJWE(jwe, key)
	if err != nil {
		t.Fatalf("DecryptJWE empty: %v", err)
	}
	if len(dec) != 0 {
		t.Fatalf("expected empty, got %d bytes", len(dec))
	}
}

func TestSM4JWEBinaryPayload(t *testing.T) {
	key := sm4Key()
	payload := make([]byte, 256)
	_, _ = rand.Read(payload)

	jwe, _ := pkgcrypto.EncryptJWE(payload, key)
	dec, err := pkgcrypto.DecryptJWE(jwe, key)
	if err != nil {
		t.Fatalf("DecryptJWE binary: %v", err)
	}
	if string(dec) != string(payload) {
		t.Fatal("binary payload mismatch")
	}
}

func TestSM4JWELargePayload(t *testing.T) {
	key := sm4Key()
	payload := make([]byte, 65536)
	_, _ = rand.Read(payload)

	jwe, _ := pkgcrypto.EncryptJWE(payload, key)
	dec, err := pkgcrypto.DecryptJWE(jwe, key)
	if err != nil {
		t.Fatalf("DecryptJWE large: %v", err)
	}
	if string(dec) != string(payload) {
		t.Fatal("large payload mismatch")
	}
}

func TestSM4JWEInvalidCompact(t *testing.T) {
	key := sm4Key()
	// 4 parts is neither JWS (3) nor JWE (5)
	_, err := pkgcrypto.DecryptJWE("a.b.c.d", key)
	if err == nil {
		t.Fatal("expected error for invalid JWE")
	}
}

func TestSM4JWETooManyParts(t *testing.T) {
	key := sm4Key()
	_, err := pkgcrypto.DecryptJWE("a.b.c.d.e.f", key)
	if err == nil {
		t.Fatal("expected error for too many parts")
	}
}

func TestSM4JWENonEmptyEncryptedKey(t *testing.T) {
	key := sm4Key()
	// Encrypted key part (index 1) must be empty for dir mode
	_, err := pkgcrypto.DecryptJWE("h.a.b.c.d", key)
	if err == nil {
		t.Fatal("expected error for non-empty encrypted key")
	}
}

func TestDecryptJWEJWSPassthrough(t *testing.T) {
	// 3-part JWS should be passed through by DecryptJWE
	jws := "header.payload.signature"
	result, err := pkgcrypto.DecryptJWE(jws, nil)
	if err != nil {
		t.Fatalf("JWS passthrough: %v", err)
	}
	if string(result) != jws {
		t.Fatalf("passthrough mismatch: %q vs %q", result, jws)
	}
}
