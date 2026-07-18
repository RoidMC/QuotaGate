package jwt_test

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"strings"
	"testing"
	"time"

	"github.com/roidmc/quotagate/internal/crypto"
	kexjwt "github.com/roidmc/quotagate/pkg/jwt"
)

const testSignKey = "quotagate-test-sign-key"
const testEncKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"
const testEncKey128 = "AAAAAAAAAAAAAAAAAAAAAA"

func mustNewManager(t *testing.T, mode kexjwt.Mode, signKey, encKey string, accessExpiry, refreshExpiry time.Duration, opts ...kexjwt.ManagerOption) *kexjwt.JWTManager {
	t.Helper()
	m, err := kexjwt.NewManager(mode, signKey, encKey, accessExpiry, refreshExpiry, opts...)
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return m
}

func generateSM2KeyBase64(t *testing.T) string {
	t.Helper()
	privKey, err := crypto.SM2GenerateKey()
	if err != nil {
		t.Fatalf("SM2GenerateKey: %v", err)
	}
	keyBytes, err := crypto.SM2PrivateKeyToBytes(privKey)
	if err != nil {
		t.Fatalf("SM2PrivateKeyToBytes: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(keyBytes)
}

func generateSM4KeyBase64(t *testing.T) string {
	t.Helper()
	key, err := crypto.SM4GenerateKey()
	if err != nil {
		t.Fatalf("SM4GenerateKey: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(key)
}

func generateRSAKeyBase64(t *testing.T) string {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("RSA GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(der)
}

func generateECDSAKeyBase64(t *testing.T, curve elliptic.Curve) string {
	t.Helper()
	key, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		t.Fatalf("ECDSA GenerateKey: %v", err)
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatalf("MarshalPKCS8PrivateKey: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(der)
}

func TestJWSGenerateAndParseAccessToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)

	token, err := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.UserID != "user-123" {
		t.Errorf("expected UserID=user-123, got %s", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Errorf("expected Role=admin, got %s", claims.Role)
	}
	if claims.TokenType != "access" {
		t.Errorf("expected TokenType=access, got %s", claims.TokenType)
	}
}

func TestJWEGenerateAndParseAccessToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)

	token, err := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken failed: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.UserID != "user-123" {
		t.Errorf("expected UserID=user-123, got %s", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Errorf("expected Role=admin, got %s", claims.Role)
	}
	if claims.TokenType != "access" {
		t.Errorf("expected TokenType=access, got %s", claims.TokenType)
	}
}

func TestJWSRefreshToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)

	token, err := m.GenerateRefreshToken("user-456", "user", "", []string{"user"})
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.TokenType != "refresh" {
		t.Errorf("expected TokenType=refresh, got %s", claims.TokenType)
	}
}

func TestJWERefreshToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)

	token, err := m.GenerateRefreshToken("user-456", "user", "", []string{"user"})
	if err != nil {
		t.Fatalf("GenerateRefreshToken failed: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	if claims.TokenType != "refresh" {
		t.Errorf("expected TokenType=refresh, got %s", claims.TokenType)
	}
}

func TestJWSExpiredToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", -1*time.Second, 7*24*time.Hour)

	token, _ := m.GenerateAccessToken("user-789", "user", "", []string{"user"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestJWEExpiredToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, -1*time.Second, 7*24*time.Hour)

	token, _ := m.GenerateAccessToken("user-789", "user", "", []string{"user"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestJWSInvalidToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)

	_, err := m.ParseToken("invalid.token.string")
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWEInvalidToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)

	_, err := m.ParseToken("invalid.token.string")
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWSWrongSignKey(t *testing.T) {
	m1 := mustNewManager(t, kexjwt.ModeJWS, "correct-key", "", 15*time.Minute, 7*24*time.Hour)
	m2 := mustNewManager(t, kexjwt.ModeJWS, "wrong-key", "", 15*time.Minute, 7*24*time.Hour)

	token, _ := m1.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken, got %v", err)
	}
}

func TestJWEWrongEncKey(t *testing.T) {
	m1 := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)
	m2 := mustNewManager(t, kexjwt.ModeJWE, testSignKey, "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", 15*time.Minute, 7*24*time.Hour)

	token, _ := m1.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err == nil {
		t.Error("expected error with wrong encryption key, got nil")
	}
}

func TestJWSTokenFormat(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)

	token, _ := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Errorf("JWS compact serialization should have 3 parts, got %d", len(parts))
	}
}

func TestJWETokenFormat(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)

	token, _ := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	parts := strings.Split(token, ".")
	if len(parts) != 5 {
		t.Errorf("JWE compact serialization should have 5 parts, got %d", len(parts))
	}
}

func TestTokenUniqueness(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)

	token1, _ := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	token2, _ := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})

	if token1 == token2 {
		t.Error("two tokens for same user should be different")
	}
}

func TestNewManager_JWEInvalidEncKey(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, "not-base64!!!", 15*time.Minute, 7*24*time.Hour)
	if err == nil {
		t.Error("expected error for invalid base64 encryption key, got nil")
	}
}

func TestNewManager_JWEShortEncKey(t *testing.T) {
	shortKey := "AAAA"
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, shortKey, 15*time.Minute, 7*24*time.Hour)
	if err == nil {
		t.Error("expected error for short encryption key, got nil")
	}
}

func TestNotBeforeValidation(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)

	token, _ := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}

	if claims.NotBefore == 0 {
		t.Error("expected nbf to be set")
	}
}

func TestSignAlgorithm_HS384(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("HS384"),
	)

	token, err := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected UserID=user-123, got %s", claims.UserID)
	}
}

func TestSignAlgorithm_HS512(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("HS512"),
	)

	token, err := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected UserID=user-123, got %s", claims.UserID)
	}
}

func TestSignAlgorithm_CrossAlgFails(t *testing.T) {
	mHS256 := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("HS256"),
	)
	mHS512 := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("HS512"),
	)

	token, _ := mHS256.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	_, err := mHS512.ParseToken(token)
	if err == nil {
		t.Error("expected error when parsing HS256 token with HS512 verifier")
	}
}

func TestContentEncryption_A128GCM(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey128, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithContentEncryption("A128GCM"),
	)

	token, err := m.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("expected UserID=user-123, got %s", claims.UserID)
	}
}

func TestContentEncryption_WrongKeyLength(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, testEncKey128, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithContentEncryption("A256GCM"),
	)
	if err == nil {
		t.Error("expected error for 16-byte key with A256GCM (needs 32)")
	}
}

func TestContentEncryption_UnsupportedAlgorithm(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithContentEncryption("FAKE-GCM"),
	)
	if err == nil {
		t.Error("expected error for unsupported content encryption algorithm")
	}
}

func TestSM2SignAndVerify(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2"),
	)

	token, err := m.GenerateAccessToken("user-sm2", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("SM2 JWS should have 3 parts, got %d", len(parts))
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-sm2" {
		t.Errorf("expected UserID=user-sm2, got %s", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Errorf("expected Role=admin, got %s", claims.Role)
	}
}

func TestSM2RefreshToken(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2"),
	)

	token, err := m.GenerateRefreshToken("user-sm2", "user", "", []string{"user"})
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("expected TokenType=refresh, got %s", claims.TokenType)
	}
}

func TestSM2ExpiredToken(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", -1*time.Second, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2"),
	)

	token, _ := m.GenerateAccessToken("user-sm2", "admin", "", []string{"admin"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestSM2WrongKey(t *testing.T) {
	key1B64 := generateSM2KeyBase64(t)
	key2B64 := generateSM2KeyBase64(t)

	m1 := mustNewManager(t, kexjwt.ModeJWS, key1B64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2"),
	)
	m2 := mustNewManager(t, kexjwt.ModeJWS, key2B64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2"),
	)

	token, _ := m1.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken with wrong SM2 key, got %v", err)
	}
}

func TestSM2InvalidKey(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWS, "not-valid-base64-key!!!", "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2"),
	)
	if err == nil {
		t.Error("expected error for invalid SM2 key")
	}
}

func TestSM4GCMBasic(t *testing.T) {
	sm4KeyB64 := generateSM4KeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, sm4KeyB64, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithContentEncryption("SM4-GCM"),
	)

	token, err := m.GenerateAccessToken("user-sm4", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	parts := strings.Split(token, ".")
	if len(parts) != 5 {
		t.Fatalf("SM4-GCM JWE should have 5 parts, got %d", len(parts))
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-sm4" {
		t.Errorf("expected UserID=user-sm4, got %s", claims.UserID)
	}
}

func TestSM4GCMRefreshToken(t *testing.T) {
	sm4KeyB64 := generateSM4KeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, sm4KeyB64, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithContentEncryption("SM4-GCM"),
	)

	token, err := m.GenerateRefreshToken("user-sm4", "user", "", []string{"user"})
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("expected TokenType=refresh, got %s", claims.TokenType)
	}
}

func TestSM4GCMExpiredToken(t *testing.T) {
	sm4KeyB64 := generateSM4KeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, sm4KeyB64, -1*time.Second, 7*24*time.Hour,
		kexjwt.WithContentEncryption("SM4-GCM"),
	)

	token, _ := m.GenerateAccessToken("user-sm4", "admin", "", []string{"admin"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestSM4GCMWrongKey(t *testing.T) {
	key1B64 := generateSM4KeyBase64(t)
	key2B64 := generateSM4KeyBase64(t)

	m1 := mustNewManager(t, kexjwt.ModeJWE, testSignKey, key1B64, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithContentEncryption("SM4-GCM"),
	)
	m2 := mustNewManager(t, kexjwt.ModeJWE, testSignKey, key2B64, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithContentEncryption("SM4-GCM"),
	)

	token, _ := m1.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err == nil {
		t.Error("expected error with wrong SM4 key")
	}
}

func TestSM4GCMInvalidKeySize(t *testing.T) {
	wrongKeyB64 := base64.RawURLEncoding.EncodeToString([]byte("short"))
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, wrongKeyB64, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithContentEncryption("SM4-GCM"),
	)
	if err == nil {
		t.Error("expected error for SM4 key with wrong size")
	}
}

func TestSM2SM4FullJWE(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)
	sm4KeyB64 := generateSM4KeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWE, sm2KeyB64, sm4KeyB64, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2"),
		kexjwt.WithContentEncryption("SM4-GCM"),
	)

	token, err := m.GenerateAccessToken("user-guomi", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-guomi" {
		t.Errorf("expected UserID=user-guomi, got %s", claims.UserID)
	}
	if claims.Role != "admin" {
		t.Errorf("expected Role=admin, got %s", claims.Role)
	}
}

func TestSM2SM4TokenUniqueness(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)
	sm4KeyB64 := generateSM4KeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWE, sm2KeyB64, sm4KeyB64, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2"),
		kexjwt.WithContentEncryption("SM4-GCM"),
	)

	token1, _ := m.GenerateAccessToken("user-guomi", "admin", "", []string{"admin"})
	token2, _ := m.GenerateAccessToken("user-guomi", "admin", "", []string{"admin"})

	if token1 == token2 {
		t.Error("two SM4-GCM tokens for same user should be different (different IV)")
	}
}

func TestSM2AlgorithmAliases(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)

	m1 := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2"),
	)
	m2 := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("SM2-SM3"),
	)

	token1, _ := m1.GenerateAccessToken("user-1", "admin", "", []string{"admin"})
	token2, _ := m2.GenerateAccessToken("user-1", "admin", "", []string{"admin"})

	claims1, err := m1.ParseToken(token1)
	if err != nil {
		t.Fatalf("ParseToken SM2: %v", err)
	}
	claims2, err := m2.ParseToken(token2)
	if err != nil {
		t.Fatalf("ParseToken SM2-SM3: %v", err)
	}

	if claims1.UserID != claims2.UserID {
		t.Error("SM2 and SM2-SM3 should be equivalent aliases")
	}
}

func TestRS256SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("RS256"),
	)

	token, err := m.GenerateAccessToken("user-rsa", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-rsa" {
		t.Errorf("expected UserID=user-rsa, got %s", claims.UserID)
	}
}

func TestRS384SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("RS384"),
	)

	token, err := m.GenerateAccessToken("user-rsa384", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-rsa384" {
		t.Errorf("expected UserID=user-rsa384, got %s", claims.UserID)
	}
}

func TestRS512SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("RS512"),
	)

	token, err := m.GenerateAccessToken("user-rsa512", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-rsa512" {
		t.Errorf("expected UserID=user-rsa512, got %s", claims.UserID)
	}
}

func TestPS256SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("PS256"),
	)

	token, err := m.GenerateAccessToken("user-ps256", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ps256" {
		t.Errorf("expected UserID=user-ps256, got %s", claims.UserID)
	}
}

func TestPS384SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("PS384"),
	)

	token, err := m.GenerateAccessToken("user-ps384", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ps384" {
		t.Errorf("expected UserID=user-ps384, got %s", claims.UserID)
	}
}

func TestPS512SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("PS512"),
	)

	token, err := m.GenerateAccessToken("user-ps512", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ps512" {
		t.Errorf("expected UserID=user-ps512, got %s", claims.UserID)
	}
}

func TestES256SignAndVerify(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P256())

	m := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("ES256"),
	)

	token, err := m.GenerateAccessToken("user-ec256", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ec256" {
		t.Errorf("expected UserID=user-ec256, got %s", claims.UserID)
	}
}

func TestES384SignAndVerify(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P384())

	m := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("ES384"),
	)

	token, err := m.GenerateAccessToken("user-ec384", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ec384" {
		t.Errorf("expected UserID=user-ec384, got %s", claims.UserID)
	}
}

func TestES512SignAndVerify(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P521())

	m := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("ES512"),
	)

	token, err := m.GenerateAccessToken("user-ec512", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ec512" {
		t.Errorf("expected UserID=user-ec512, got %s", claims.UserID)
	}
}

func TestRSAWrongKey(t *testing.T) {
	key1B64 := generateRSAKeyBase64(t)
	key2B64 := generateRSAKeyBase64(t)

	m1 := mustNewManager(t, kexjwt.ModeJWS, key1B64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("RS256"),
	)
	m2 := mustNewManager(t, kexjwt.ModeJWS, key2B64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("RS256"),
	)

	token, _ := m1.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken with wrong RSA key, got %v", err)
	}
}

func TestECDSAWrongKey(t *testing.T) {
	key1B64 := generateECDSAKeyBase64(t, elliptic.P256())
	key2B64 := generateECDSAKeyBase64(t, elliptic.P256())

	m1 := mustNewManager(t, kexjwt.ModeJWS, key1B64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("ES256"),
	)
	m2 := mustNewManager(t, kexjwt.ModeJWS, key2B64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("ES256"),
	)

	token, _ := m1.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("expected ErrInvalidToken with wrong ECDSA key, got %v", err)
	}
}

func TestRSAInvalidKey(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWS, "not-valid-base64!!!", "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("RS256"),
	)
	if err == nil {
		t.Error("expected error for invalid RSA key")
	}
}

func TestECDSAInvalidKey(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWS, "not-valid-base64!!!", "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("ES256"),
	)
	if err == nil {
		t.Error("expected error for invalid ECDSA key")
	}
}

func TestRS256WithJWE(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWE, rsaKeyB64, testEncKey, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("RS256"),
	)

	token, err := m.GenerateAccessToken("user-rsa-jwe", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-rsa-jwe" {
		t.Errorf("expected UserID=user-rsa-jwe, got %s", claims.UserID)
	}
}

func TestES256WithJWE(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P256())

	m := mustNewManager(t, kexjwt.ModeJWE, ecKeyB64, testEncKey, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("ES256"),
	)

	token, err := m.GenerateAccessToken("user-ec-jwe", "admin", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}

	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ec-jwe" {
		t.Errorf("expected UserID=user-ec-jwe, got %s", claims.UserID)
	}
}

func TestRSACrossAlgFails(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)

	mRS256 := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("RS256"),
	)
	mPS256 := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("PS256"),
	)

	token, _ := mRS256.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	_, err := mPS256.ParseToken(token)
	if err == nil {
		t.Error("expected error when parsing RS256 token with PS256 verifier")
	}
}

func TestECDSACrossAlgFails(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P256())

	mES256 := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("ES256"),
	)
	mHS256 := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("HS256"),
	)

	token, _ := mES256.GenerateAccessToken("user-123", "admin", "", []string{"admin"})
	_, err := mHS256.ParseToken(token)
	if err == nil {
		t.Error("expected error when parsing ES256 token with HS256 verifier")
	}
}

func TestRSAExpiredToken(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)

	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", -1*time.Second, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("RS256"),
	)

	token, _ := m.GenerateAccessToken("user-rsa", "admin", "", []string{"admin"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestECDSAExpiredToken(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P256())

	m := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", -1*time.Second, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("ES256"),
	)

	token, _ := m.GenerateAccessToken("user-ec", "admin", "", []string{"admin"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("expected ErrExpiredToken, got %v", err)
	}
}

func TestUnsupportedSignAlgorithm(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm("EdDSA"),
	)
	if err == nil {
		t.Error("expected error for unsupported sign algorithm")
	}
}
