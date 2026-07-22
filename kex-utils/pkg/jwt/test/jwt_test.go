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

	gmsm "github.com/emmansun/gmsm/sm2"

	pkgcrypto "github.com/roidmc/kex-utils/pkg/crypto"
	kexjwt "github.com/roidmc/kex-utils/pkg/jwt"
)

const testSignKey = "quotagate-test-sign-key"
const testEncKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"  // 32 bytes
const testEncKey128 = "AAAAAAAAAAAAAAAAAAAAAA"                    // 16 bytes
const testEncKey192 = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"          // 24 bytes

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
	priv, err := gmsm.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatalf("SM2 GenerateKey: %v", err)
	}
	ecdhKey, err := priv.ECDH()
	if err != nil {
		t.Fatalf("SM2 ECDH: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(ecdhKey.Bytes())
}

func generateSM4KeyBase64(t *testing.T) string {
	t.Helper()
	key := make([]byte, pkgcrypto.SM4BlockSize)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("SM4 key gen: %v", err)
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

// --- Basic JWS / JWE ---

func TestJWSGenerateAndParseAccessToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)
	token, err := m.GenerateAccessToken("user-123", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", claims.UserID)
	}
	if claims.TokenType != "access" {
		t.Errorf("TokenType = %s, want access", claims.TokenType)
	}
}

func TestJWEGenerateAndParseAccessToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)
	token, err := m.GenerateAccessToken("user-123", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", claims.UserID)
	}
}

func TestJWSRefreshToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)
	token, err := m.GenerateRefreshToken("user-456", "", []string{"user"})
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("TokenType = %s, want refresh", claims.TokenType)
	}
}

func TestJWERefreshToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)
	token, err := m.GenerateRefreshToken("user-456", "", []string{"user"})
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("TokenType = %s, want refresh", claims.TokenType)
	}
}

// --- Expiry ---

func TestJWSExpiredToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", -1*time.Second, 7*24*time.Hour)
	token, _ := m.GenerateAccessToken("user-789", "", []string{"user"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("got %v, want ErrExpiredToken", err)
	}
}

func TestJWEExpiredToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, -1*time.Second, 7*24*time.Hour)
	token, _ := m.GenerateAccessToken("user-789", "", []string{"user"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("got %v, want ErrExpiredToken", err)
	}
}

// --- Invalid token ---

func TestJWSInvalidToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)
	_, err := m.ParseToken("invalid.token.string")
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

func TestJWEInvalidToken(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)
	_, err := m.ParseToken("invalid.token.string")
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

// --- Wrong key ---

func TestJWSWrongSignKey(t *testing.T) {
	m1 := mustNewManager(t, kexjwt.ModeJWS, "correct-key", "", 15*time.Minute, 7*24*time.Hour)
	m2 := mustNewManager(t, kexjwt.ModeJWS, "wrong-key", "", 15*time.Minute, 7*24*time.Hour)
	token, _ := m1.GenerateAccessToken("user-123", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

func TestJWEWrongEncKey(t *testing.T) {
	m1 := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)
	m2 := mustNewManager(t, kexjwt.ModeJWE, testSignKey, "BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB", 15*time.Minute, 7*24*time.Hour)
	token, _ := m1.GenerateAccessToken("user-123", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err == nil {
		t.Error("expected error with wrong encryption key")
	}
}

// --- Token format ---

func TestJWSTokenFormat(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)
	token, _ := m.GenerateAccessToken("user-123", "", []string{"admin"})
	if n := len(strings.Split(token, ".")); n != 3 {
		t.Errorf("JWS has %d parts, want 3", n)
	}
}

func TestJWETokenFormat(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)
	token, _ := m.GenerateAccessToken("user-123", "", []string{"admin"})
	if n := len(strings.Split(token, ".")); n != 5 {
		t.Errorf("JWE has %d parts, want 5", n)
	}
}

func TestTokenUniqueness(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour)
	t1, _ := m.GenerateAccessToken("user-123", "", []string{"admin"})
	t2, _ := m.GenerateAccessToken("user-123", "", []string{"admin"})
	if t1 == t2 {
		t.Error("two tokens should be different")
	}
}

// --- Manager construction errors ---

func TestNewManager_JWEInvalidEncKey(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, "not-base64!!!", 15*time.Minute, 7*24*time.Hour)
	if err == nil {
		t.Error("expected error for invalid base64 encryption key")
	}
}

func TestNewManager_JWEShortEncKey(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, "AAAA", 15*time.Minute, 7*24*time.Hour)
	if err == nil {
		t.Error("expected error for short encryption key")
	}
}

func TestNotBeforeValidation(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour)
	token, _ := m.GenerateAccessToken("user-123", "", []string{"admin"})
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.NotBefore == 0 {
		t.Error("nbf should be set")
	}
}

// --- HMAC variants ---

func TestSignAlgorithm_HS384(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("HS384"))
	token, err := m.GenerateAccessToken("user-123", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", claims.UserID)
	}
}

func TestSignAlgorithm_HS512(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("HS512"))
	token, err := m.GenerateAccessToken("user-123", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", claims.UserID)
	}
}

func TestSignAlgorithm_CrossAlgFails(t *testing.T) {
	m1 := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("HS256"))
	m2 := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("HS512"))
	token, _ := m1.GenerateAccessToken("user-123", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err == nil {
		t.Error("expected error cross-algorithm")
	}
}

// --- AES content encryption ---

func TestContentEncryption_A128GCM(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey128, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption("A128GCM"))
	token, err := m.GenerateAccessToken("user-123", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", claims.UserID)
	}
}

func TestContentEncryption_A192GCM(t *testing.T) {
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, testEncKey192, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption("A192GCM"))
	token, err := m.GenerateAccessToken("user-123", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-123" {
		t.Errorf("UserID = %s, want user-123", claims.UserID)
	}
}

func TestContentEncryption_WrongKeyLength(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, testEncKey128, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption("A256GCM"))
	if err == nil {
		t.Error("expected error for 16-byte key with A256GCM")
	}
}

func TestContentEncryption_UnsupportedAlgorithm(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, testEncKey, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption("FAKE-GCM"))
	if err == nil {
		t.Error("expected error for unsupported content encryption")
	}
}

// --- SM2 (SGD_SM3_SM2) ---

func TestSM2SignAndVerify(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm(pkgcrypto.SGD_SM3_SM2))
	token, err := m.GenerateAccessToken("user-sm2", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if n := len(strings.Split(token, ".")); n != 3 {
		t.Fatalf("SM2 JWS has %d parts, want 3", n)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-sm2" {
		t.Errorf("UserID = %s, want user-sm2", claims.UserID)
	}
}

func TestSM2RefreshToken(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm(pkgcrypto.SGD_SM3_SM2))
	token, err := m.GenerateRefreshToken("user-sm2", "", []string{"user"})
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("TokenType = %s, want refresh", claims.TokenType)
	}
}

func TestSM2ExpiredToken(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", -1*time.Second, 7*24*time.Hour, kexjwt.WithSignAlgorithm(pkgcrypto.SGD_SM3_SM2))
	token, _ := m.GenerateAccessToken("user-sm2", "", []string{"admin"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("got %v, want ErrExpiredToken", err)
	}
}

func TestSM2WrongKey(t *testing.T) {
	k1 := generateSM2KeyBase64(t)
	k2 := generateSM2KeyBase64(t)
	m1 := mustNewManager(t, kexjwt.ModeJWS, k1, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm(pkgcrypto.SGD_SM3_SM2))
	m2 := mustNewManager(t, kexjwt.ModeJWS, k2, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm(pkgcrypto.SGD_SM3_SM2))
	token, _ := m1.GenerateAccessToken("user-123", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

func TestSM2InvalidKey(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWS, "not-valid-base64!!!", "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm(pkgcrypto.SGD_SM3_SM2))
	if err == nil {
		t.Error("expected error for invalid SM2 key")
	}
}

// --- SM4-GCM (SGD_SM4_GCM) ---

func TestSM4GCMBasic(t *testing.T) {
	sm4KeyB64 := generateSM4KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, sm4KeyB64, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption(pkgcrypto.SGD_SM4_GCM))
	token, err := m.GenerateAccessToken("user-sm4", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	if n := len(strings.Split(token, ".")); n != 5 {
		t.Fatalf("SM4 JWE has %d parts, want 5", n)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-sm4" {
		t.Errorf("UserID = %s, want user-sm4", claims.UserID)
	}
}

func TestSM4GCMRefreshToken(t *testing.T) {
	sm4KeyB64 := generateSM4KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, sm4KeyB64, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption(pkgcrypto.SGD_SM4_GCM))
	token, err := m.GenerateRefreshToken("user-sm4", "", []string{"user"})
	if err != nil {
		t.Fatalf("GenerateRefreshToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.TokenType != "refresh" {
		t.Errorf("TokenType = %s, want refresh", claims.TokenType)
	}
}

func TestSM4GCMExpiredToken(t *testing.T) {
	sm4KeyB64 := generateSM4KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, sm4KeyB64, -1*time.Second, 7*24*time.Hour, kexjwt.WithContentEncryption(pkgcrypto.SGD_SM4_GCM))
	token, _ := m.GenerateAccessToken("user-sm4", "", []string{"admin"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("got %v, want ErrExpiredToken", err)
	}
}

func TestSM4GCMWrongKey(t *testing.T) {
	k1 := generateSM4KeyBase64(t)
	k2 := generateSM4KeyBase64(t)
	m1 := mustNewManager(t, kexjwt.ModeJWE, testSignKey, k1, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption(pkgcrypto.SGD_SM4_GCM))
	m2 := mustNewManager(t, kexjwt.ModeJWE, testSignKey, k2, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption(pkgcrypto.SGD_SM4_GCM))
	token, _ := m1.GenerateAccessToken("user-123", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err == nil {
		t.Error("expected error with wrong SM4 key")
	}
}

func TestSM4GCMInvalidKeySize(t *testing.T) {
	wrong := base64.RawURLEncoding.EncodeToString([]byte("short"))
	_, err := kexjwt.NewManager(kexjwt.ModeJWE, testSignKey, wrong, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption(pkgcrypto.SGD_SM4_GCM))
	if err == nil {
		t.Error("expected error for wrong SM4 key size")
	}
}

// --- SM2 + SM4 combined ---

func TestSM2SM4FullJWE(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)
	sm4KeyB64 := generateSM4KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWE, sm2KeyB64, sm4KeyB64, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm(pkgcrypto.SGD_SM3_SM2),
		kexjwt.WithContentEncryption(pkgcrypto.SGD_SM4_GCM),
	)
	token, err := m.GenerateAccessToken("user-guomi", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-guomi" {
		t.Errorf("UserID = %s, want user-guomi", claims.UserID)
	}
}

func TestSM2SM4TokenUniqueness(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)
	sm4KeyB64 := generateSM4KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWE, sm2KeyB64, sm4KeyB64, 15*time.Minute, 7*24*time.Hour,
		kexjwt.WithSignAlgorithm(pkgcrypto.SGD_SM3_SM2),
		kexjwt.WithContentEncryption(pkgcrypto.SGD_SM4_GCM),
	)
	t1, _ := m.GenerateAccessToken("user-guomi", "", []string{"admin"})
	t2, _ := m.GenerateAccessToken("user-guomi", "", []string{"admin"})
	if t1 == t2 {
		t.Error("two tokens should be different")
	}
}

// --- Legacy aliases ---

func TestSM2LegacyAliasSM2(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("SM2"))
	token, err := m.GenerateAccessToken("user-alias", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-alias" {
		t.Errorf("UserID = %s, want user-alias", claims.UserID)
	}
}

func TestSM2LegacyAliasSM2SM3(t *testing.T) {
	sm2KeyB64 := generateSM2KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, sm2KeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("SM2-SM3"))
	token, err := m.GenerateAccessToken("user-sm2sm3", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-sm2sm3" {
		t.Errorf("UserID = %s, want user-sm2sm3", claims.UserID)
	}
}

func TestSM4LegacyAliasSM4GCM(t *testing.T) {
	sm4KeyB64 := generateSM4KeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWE, testSignKey, sm4KeyB64, 15*time.Minute, 7*24*time.Hour, kexjwt.WithContentEncryption("SM4-GCM"))
	token, err := m.GenerateAccessToken("user-sm4legacy", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-sm4legacy" {
		t.Errorf("UserID = %s, want user-sm4legacy", claims.UserID)
	}
}

// --- RSA ---

func TestRS256SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("RS256"))
	token, err := m.GenerateAccessToken("user-rsa", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-rsa" {
		t.Errorf("UserID = %s, want user-rsa", claims.UserID)
	}
}

func TestRS384SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("RS384"))
	token, err := m.GenerateAccessToken("user-rsa384", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-rsa384" {
		t.Errorf("UserID = %s, want user-rsa384", claims.UserID)
	}
}

func TestRS512SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("RS512"))
	token, err := m.GenerateAccessToken("user-rsa512", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-rsa512" {
		t.Errorf("UserID = %s, want user-rsa512", claims.UserID)
	}
}

func TestPS256SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("PS256"))
	token, err := m.GenerateAccessToken("user-ps256", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ps256" {
		t.Errorf("UserID = %s, want user-ps256", claims.UserID)
	}
}

func TestPS384SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("PS384"))
	token, err := m.GenerateAccessToken("user-ps384", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ps384" {
		t.Errorf("UserID = %s, want user-ps384", claims.UserID)
	}
}

func TestPS512SignAndVerify(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("PS512"))
	token, err := m.GenerateAccessToken("user-ps512", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ps512" {
		t.Errorf("UserID = %s, want user-ps512", claims.UserID)
	}
}

// --- ECDSA ---

func TestES256SignAndVerify(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P256())
	m := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("ES256"))
	token, err := m.GenerateAccessToken("user-ec256", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ec256" {
		t.Errorf("UserID = %s, want user-ec256", claims.UserID)
	}
}

func TestES384SignAndVerify(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P384())
	m := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("ES384"))
	token, err := m.GenerateAccessToken("user-ec384", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ec384" {
		t.Errorf("UserID = %s, want user-ec384", claims.UserID)
	}
}

func TestES512SignAndVerify(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P521())
	m := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("ES512"))
	token, err := m.GenerateAccessToken("user-ec512", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ec512" {
		t.Errorf("UserID = %s, want user-ec512", claims.UserID)
	}
}

// --- Wrong key (RSA/ECDSA) ---

func TestRSAWrongKey(t *testing.T) {
	k1 := generateRSAKeyBase64(t)
	k2 := generateRSAKeyBase64(t)
	m1 := mustNewManager(t, kexjwt.ModeJWS, k1, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("RS256"))
	m2 := mustNewManager(t, kexjwt.ModeJWS, k2, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("RS256"))
	token, _ := m1.GenerateAccessToken("user-123", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

func TestECDSAWrongKey(t *testing.T) {
	k1 := generateECDSAKeyBase64(t, elliptic.P256())
	k2 := generateECDSAKeyBase64(t, elliptic.P256())
	m1 := mustNewManager(t, kexjwt.ModeJWS, k1, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("ES256"))
	m2 := mustNewManager(t, kexjwt.ModeJWS, k2, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("ES256"))
	token, _ := m1.GenerateAccessToken("user-123", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err != kexjwt.ErrInvalidToken {
		t.Errorf("got %v, want ErrInvalidToken", err)
	}
}

func TestRSAInvalidKey(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWS, "not-valid-base64!!!", "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("RS256"))
	if err == nil {
		t.Error("expected error for invalid RSA key")
	}
}

func TestECDSAInvalidKey(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWS, "not-valid-base64!!!", "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("ES256"))
	if err == nil {
		t.Error("expected error for invalid ECDSA key")
	}
}

// --- RSA/ECDSA + JWE ---

func TestRS256WithJWE(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWE, rsaKeyB64, testEncKey, 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("RS256"))
	token, err := m.GenerateAccessToken("user-rsa-jwe", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-rsa-jwe" {
		t.Errorf("UserID = %s, want user-rsa-jwe", claims.UserID)
	}
}

func TestES256WithJWE(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P256())
	m := mustNewManager(t, kexjwt.ModeJWE, ecKeyB64, testEncKey, 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("ES256"))
	token, err := m.GenerateAccessToken("user-ec-jwe", "", []string{"admin"})
	if err != nil {
		t.Fatalf("GenerateAccessToken: %v", err)
	}
	claims, err := m.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken: %v", err)
	}
	if claims.UserID != "user-ec-jwe" {
		t.Errorf("UserID = %s, want user-ec-jwe", claims.UserID)
	}
}

// --- Cross-algorithm ---

func TestRSACrossAlgFails(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)
	m1 := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("RS256"))
	m2 := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("PS256"))
	token, _ := m1.GenerateAccessToken("user-123", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err == nil {
		t.Error("expected error cross-algorithm RS256 vs PS256")
	}
}

func TestECDSACrossAlgFails(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P256())
	m1 := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("ES256"))
	m2 := mustNewManager(t, kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("HS256"))
	token, _ := m1.GenerateAccessToken("user-123", "", []string{"admin"})
	_, err := m2.ParseToken(token)
	if err == nil {
		t.Error("expected error cross-algorithm ES256 vs HS256")
	}
}

func TestRSAExpiredToken(t *testing.T) {
	rsaKeyB64 := generateRSAKeyBase64(t)
	m := mustNewManager(t, kexjwt.ModeJWS, rsaKeyB64, "", -1*time.Second, 7*24*time.Hour, kexjwt.WithSignAlgorithm("RS256"))
	token, _ := m.GenerateAccessToken("user-rsa", "", []string{"admin"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("got %v, want ErrExpiredToken", err)
	}
}

func TestECDSAExpiredToken(t *testing.T) {
	ecKeyB64 := generateECDSAKeyBase64(t, elliptic.P256())
	m := mustNewManager(t, kexjwt.ModeJWS, ecKeyB64, "", -1*time.Second, 7*24*time.Hour, kexjwt.WithSignAlgorithm("ES256"))
	token, _ := m.GenerateAccessToken("user-ec", "", []string{"admin"})
	_, err := m.ParseToken(token)
	if err != kexjwt.ErrExpiredToken {
		t.Errorf("got %v, want ErrExpiredToken", err)
	}
}

func TestUnsupportedSignAlgorithm(t *testing.T) {
	_, err := kexjwt.NewManager(kexjwt.ModeJWS, testSignKey, "", 15*time.Minute, 7*24*time.Hour, kexjwt.WithSignAlgorithm("EdDSA"))
	if err == nil {
		t.Error("expected error for unsupported sign algorithm")
	}
}
