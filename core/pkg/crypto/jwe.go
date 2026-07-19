package crypto

import (
	"context"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/emmansun/gmsm/sm4"
)

// EncryptJWE encrypts plaintext using SM4-GCM dir mode through ProviderRegistry.
// HSM/KMS can override the content encryption via RegisterContentEncryptor.
func EncryptJWE(plaintext []byte, key []byte) (string, error) {
	if len(key) != SM4BlockSize {
		return "", fmt.Errorf("pkg/crypto: SM4 key must be %d bytes", SM4BlockSize)
	}

	enc := SGD_SM4_GCM

	// ProviderRegistry path (HSM override)
	if provider, ok := DefaultRegistry.GetContentEncryptor(enc); ok {
		header := map[string]string{"alg": "dir", "enc": enc}
		headerJSON, _ := json.Marshal(header)
		headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
		iv := make([]byte, SM4GCMNonceSize)
		if _, err := rand.Read(iv); err != nil {
			return "", err
		}
		sealed, err := provider.Encrypt(context.Background(), key, iv, plaintext, []byte(headerB64))
		if err != nil {
			return "", err
		}
		tagSize := 16
		if len(sealed) < tagSize {
			return "", errors.New("encryption output too short")
		}
		return headerB64 + ".." +
			base64.RawURLEncoding.EncodeToString(iv) + "." +
			base64.RawURLEncoding.EncodeToString(sealed[:len(sealed)-tagSize]) + "." +
			base64.RawURLEncoding.EncodeToString(sealed[len(sealed)-tagSize:]), nil
	}

	// Built-in path
	block, err := sm4.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	protected := map[string]string{"alg": "dir", "enc": enc}
	protectedJSON, _ := json.Marshal(protected)
	protectedB64 := base64.RawURLEncoding.EncodeToString(protectedJSON)
	iv := make([]byte, SM4GCMNonceSize)
	if _, err := rand.Read(iv); err != nil {
		return "", err
	}
	sealed := aesgcm.Seal(nil, iv, plaintext, []byte(protectedB64))
	tagSize := aesgcm.Overhead()
	return protectedB64 + ".." +
		base64.RawURLEncoding.EncodeToString(iv) + "." +
		base64.RawURLEncoding.EncodeToString(sealed[:len(sealed)-tagSize]) + "." +
		base64.RawURLEncoding.EncodeToString(sealed[len(sealed)-tagSize:]), nil
}

// DecryptJWE decrypts a JWE compact serialization (SM4-GCM dir mode).
// Uses ProviderRegistry; HSM/KMS can override via RegisterContentDecryptor.
//
// JWS compact (3 parts) is passed through unchanged — some callers use this
// as a unified entry point for both JWE and JWS.
func DecryptJWE(compact string, key []byte) ([]byte, error) {
	parts := strings.Split(compact, ".")

	// JWS passthrough: 3 parts = JWS, not JWE. No decryption needed.
	// This check comes BEFORE key validation so callers can pass nil key
	// when they only have a JWS.
	if len(parts) == 3 {
		return []byte(compact), nil
	}

	if len(key) != SM4BlockSize {
		return nil, fmt.Errorf("pkg/crypto: SM4 key must be %d bytes, got %d", SM4BlockSize, len(key))
	}

	if len(parts) != 5 {
		return nil, fmt.Errorf("pkg/crypto: invalid JWE: expected 5 parts, got %d", len(parts))
	}

	if parts[1] != "" {
		return nil, errors.New("pkg/crypto: expected empty encrypted key for dir mode")
	}

	iv, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, fmt.Errorf("pkg/crypto: invalid IV: %w", err)
	}
	if len(iv) != SM4GCMNonceSize {
		return nil, fmt.Errorf("pkg/crypto: IV must be %d bytes, got %d", SM4GCMNonceSize, len(iv))
	}

	ct, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, fmt.Errorf("pkg/crypto: invalid ciphertext: %w", err)
	}
	tag, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, fmt.Errorf("pkg/crypto: invalid tag: %w", err)
	}
	if len(tag) != 16 {
		return nil, fmt.Errorf("pkg/crypto: GCM tag must be 16 bytes, got %d", len(tag))
	}

	sealed := append(ct, tag...)
	aad := []byte(parts[0])

	if provider, ok := DefaultRegistry.GetContentDecryptor(SGD_SM4_GCM); ok {
		return provider.Decrypt(context.Background(), key, iv, sealed, aad)
	}

	return sm4GCMDecrypt(key, iv, sealed, aad)
}

// --- Internal SM4-GCM helpers ---

func sm4GCMEncrypt(key, iv, plaintext, aad []byte) ([]byte, error) {
	if len(key) != SM4BlockSize {
		return nil, fmt.Errorf("pkg/crypto: SM4 key must be %d bytes, got %d", SM4BlockSize, len(key))
	}
	if len(iv) != SM4GCMNonceSize {
		return nil, fmt.Errorf("pkg/crypto: GCM nonce must be %d bytes, got %d", SM4GCMNonceSize, len(iv))
	}
	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return g.Seal(nil, iv, plaintext, aad), nil
}

func sm4GCMDecrypt(key, iv, sealed, aad []byte) ([]byte, error) {
	if len(key) != SM4BlockSize {
		return nil, fmt.Errorf("pkg/crypto: SM4 key must be %d bytes, got %d", SM4BlockSize, len(key))
	}
	if len(iv) != SM4GCMNonceSize {
		return nil, fmt.Errorf("pkg/crypto: GCM nonce must be %d bytes, got %d", SM4GCMNonceSize, len(iv))
	}
	// GCM tag is 16 bytes; sealed = ciphertext + tag
	if len(sealed) < 16 {
		return nil, errors.New("pkg/crypto: sealed data too short (need at least GCM tag)")
	}
	block, err := sm4.NewCipher(key)
	if err != nil {
		return nil, err
	}
	g, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return g.Open(nil, iv, sealed, aad)
}
