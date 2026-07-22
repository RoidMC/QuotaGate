// Package jwt provides QuotaGate API Gateway token lifecycle management.
//
// Architecture:
//
//	pkg/crypto/     Gateway's cryptographic primitives with ProviderRegistry
//	                dispatch (mirrors kexcore-oidc + kexswiftdb/bus pattern).
//	                Interfaces: SignProvider, VerifyProvider, ContentEncryptProvider.
//	                Built-in gmsm providers registered in init(); HSM/KMS overrides
//	                via init() (last registration wins).
//
//	pkg/jwt/        Gateway token layer (this package).
//	                Business logic: QuotaGate Claims, access/refresh issuance,
//	                iss/aud/exp/nbf validation.
//	                SM2 → pkg/crypto.Signer (through ProviderRegistry).
//	                SM4 → pkg/crypto.EncryptJWE/DecryptJWE (through ProviderRegistry).
//	                HMAC/RSA/ECDSA → jwx directly.
//
// Algorithm naming follows GM/T 0125.1:
//
//	SGD_SM3_SM2   SM2+SM3 digital signature
//	SGD_SM4_GCM   SM4-GCM content encryption
package jwt

import (
	"context"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	gmsm "github.com/emmansun/gmsm/sm2"
	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwe"
	"github.com/lestrrat-go/jwx/v4/jws"

	pkgcrypto "github.com/roidmc/kex-utils/pkg/crypto"
	"github.com/roidmc/kex-utils/pkg/kexrandom"
)

var (
	ErrInvalidToken    = errors.New("quotagate/jwt: invalid token")
	ErrExpiredToken    = errors.New("quotagate/jwt: token expired")
	ErrInvalidIssuer   = errors.New("quotagate/jwt: invalid issuer")
	ErrInvalidAudience = errors.New("quotagate/jwt: invalid audience")
)

// Mode specifies the token protection mode.
type Mode string

const (
	ModeJWS Mode = "jws" // JWS (signed only)
	ModeJWE Mode = "jwe" // JWE (nested: signed then encrypted)
)

// Claims is the QuotaGate JWT payload schema.
type Claims struct {
	Issuer    string                 `json:"iss,omitempty"`
	Subject   string                 `json:"sub,omitempty"`
	Audience  string                 `json:"aud,omitempty"`
	ExpiresAt int64                  `json:"exp"`
	NotBefore int64                  `json:"nbf"`
	IssuedAt  int64                  `json:"iat"`
	JWTID     string                 `json:"jti,omitempty"`
	UserID    string                 `json:"uid"`
	TenantID  string                 `json:"tid,omitempty"`
	Roles     []string               `json:"roles,omitempty"`
	TokenType string                 `json:"type"` // "access" or "refresh"
	Extra     map[string]interface{} `json:"extra,omitempty"`
}

func (c *Claims) isValid() bool {
	now := time.Now().Unix()
	if now > c.ExpiresAt {
		return false
	}
	if c.NotBefore > 0 && now < c.NotBefore {
		return false
	}
	return true
}

// JWTManager manages token issuance and validation.
//
// Cryptography:
//   - SM2:  delegated to pkg/crypto (SM2SignCompact / SM2VerifyCompact)
//   - SM4:  delegated to pkg/crypto (SM4GCMEncryptJWE / SM4GCMDecryptJWE)
//   - HMAC/RSA/ECDSA: jwx directly
type JWTManager struct {
	mode              Mode
	hmacKey           []byte
	stdSigner         interface{}     // *rsa.PrivateKey or *ecdsa.PrivateKey
	sm2Priv           *gmsm.PrivateKey
	sm2Pub            *ecdsa.PublicKey
	encKey            []byte
	signAlgorithm     string
	contentEncryption string
	issuer            string
	audience          string
	accessExpiry      time.Duration
	refreshExpiry     time.Duration
}

// ManagerOption configures JWTManager.
type ManagerOption func(*JWTManager)

// WithIssuer sets the expected "iss" claim.
func WithIssuer(issuer string) ManagerOption {
	return func(m *JWTManager) { m.issuer = issuer }
}

// WithAudience sets the expected "aud" claim.
func WithAudience(audience string) ManagerOption {
	return func(m *JWTManager) { m.audience = audience }
}

// WithSignAlgorithm sets the JWA signature algorithm.
// Supported: HS256/384/512, RS256/384/512, PS256/384/512, ES256/384/512,
// SGD_SM3_SM2 (and legacy "SM2"/"SM2-SM3" aliases). Default: HS256.
func WithSignAlgorithm(alg string) ManagerOption {
	return func(m *JWTManager) { m.signAlgorithm = alg }
}

// WithContentEncryption sets the JWE content encryption algorithm.
// Supported: A128GCM, A192GCM, A256GCM, SGD_SM4_GCM (and legacy
// "SM4-GCM"/"SM4" aliases). Default: A256GCM. Ignored in ModeJWS.
func WithContentEncryption(alg string) ManagerOption {
	return func(m *JWTManager) { m.contentEncryption = alg }
}

// --- algorithm helpers ---

func isHMACAlgorithm(alg string) bool {
	switch alg {
	case "HS256", "HS384", "HS512":
		return true
	}
	return false
}

func isRSAAlgorithm(alg string) bool {
	switch alg {
	case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512":
		return true
	}
	return false
}

func isECDSAAlgorithm(alg string) bool {
	switch alg {
	case "ES256", "ES384", "ES512":
		return true
	}
	return false
}

// isSM2Algorithm accepts standard SGD_SM3_SM2 and legacy "SM2"/"SM2-SM3".
func isSM2Algorithm(alg string) bool {
	return alg == pkgcrypto.SGD_SM3_SM2 ||
		strings.EqualFold(alg, "SM2") ||
		strings.EqualFold(alg, "SM2-SM3")
}

// isSM4Encryption accepts standard SGD_SM4_GCM and legacy "SM4-GCM"/"SM4".
func isSM4Encryption(alg string) bool {
	return alg == pkgcrypto.SGD_SM4_GCM ||
		strings.EqualFold(alg, "SM4-GCM") ||
		strings.EqualFold(alg, "SM4")
}

func isStandardContentEncryption(alg string) bool {
	switch alg {
	case "A128GCM", "A192GCM", "A256GCM":
		return true
	}
	return false
}

// --- construction ---

func (m *JWTManager) resolveSignAlgorithm() jwa.SignatureAlgorithm {
	if alg, ok := jwa.LookupSignatureAlgorithm(m.signAlgorithm); ok {
		return alg
	}
	return jwa.HS256()
}

func (m *JWTManager) resolveContentEncryption() jwa.ContentEncryptionAlgorithm {
	if alg, ok := jwa.LookupContentEncryptionAlgorithm(m.contentEncryption); ok {
		return alg
	}
	return jwa.A256GCM()
}

// NewManager creates a JWTManager.
//
// signKey: raw string (HMAC) or base64url-encoded DER (RSA/ECDSA) or
// base64url-encoded raw bytes (SM2).
// encKey: base64url-encoded symmetric key (JWE mode only).
func NewManager(mode Mode, signKey, encKey string, accessExpiry, refreshExpiry time.Duration, opts ...ManagerOption) (*JWTManager, error) {
	m := &JWTManager{
		mode:              mode,
		signAlgorithm:     "HS256",
		contentEncryption: "A256GCM",
		accessExpiry:      accessExpiry,
		refreshExpiry:     refreshExpiry,
	}
	for _, opt := range opts {
		opt(m)
	}
	if err := m.initSignKey(signKey); err != nil {
		return nil, err
	}
	if mode == ModeJWE {
		if err := m.initEncKey(encKey); err != nil {
			return nil, err
		}
	}
	return m, nil
}

func (m *JWTManager) initSignKey(signKey string) error {
	if isSM2Algorithm(m.signAlgorithm) {
		return m.initSM2Key(signKey)
	}
	if isRSAAlgorithm(m.signAlgorithm) {
		return m.initRSAKey(signKey)
	}
	if isECDSAAlgorithm(m.signAlgorithm) {
		return m.initECDSAKey(signKey)
	}
	if isHMACAlgorithm(m.signAlgorithm) {
		m.hmacKey = []byte(signKey)
		return nil
	}
	return fmt.Errorf("quotagate/jwt: unsupported sign algorithm: %s", m.signAlgorithm)
}

func (m *JWTManager) initSM2Key(signKey string) error {
	raw, err := base64.RawURLEncoding.DecodeString(signKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: sign_key must be base64url-encoded SM2 private key: %w", err)
	}
	priv, err := gmsm.NewPrivateKey(raw)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: invalid SM2 private key: %w", err)
	}
	m.sm2Priv = priv
	m.sm2Pub = &priv.PublicKey
	return nil
}

func (m *JWTManager) initRSAKey(signKey string) error {
	keyBytes, err := base64.RawURLEncoding.DecodeString(signKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: sign_key must be base64url-encoded: %w", err)
	}
	key, err := x509.ParsePKCS8PrivateKey(keyBytes)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: invalid RSA private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("quotagate/jwt: sign_key is not an RSA private key")
	}
	m.stdSigner = rsaKey
	return nil
}

func (m *JWTManager) initECDSAKey(signKey string) error {
	keyBytes, err := base64.RawURLEncoding.DecodeString(signKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: sign_key must be base64url-encoded: %w", err)
	}
	key, err := x509.ParsePKCS8PrivateKey(keyBytes)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: invalid ECDSA private key: %w", err)
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return fmt.Errorf("quotagate/jwt: sign_key is not an ECDSA private key")
	}
	m.stdSigner = ecKey
	return nil
}

func (m *JWTManager) initEncKey(encKey string) error {
	if isSM4Encryption(m.contentEncryption) {
		return m.initEncKeyLen(encKey, pkgcrypto.SM4BlockSize)
	}
	return m.initAESKey(encKey)
}

func (m *JWTManager) initEncKeyLen(encKey string, expected int) error {
	key, err := base64.RawURLEncoding.DecodeString(encKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: encryption_key must be base64url-encoded: %w", err)
	}
	if len(key) != expected {
		return fmt.Errorf("quotagate/jwt: encryption_key must be %d bytes, got %d", expected, len(key))
	}
	m.encKey = key
	return nil
}

func (m *JWTManager) initAESKey(encKey string) error {
	key, err := base64.RawURLEncoding.DecodeString(encKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: encryption_key must be base64url-encoded: %w", err)
	}
	var expected int
	switch m.contentEncryption {
	case "A128GCM":
		expected = 16
	case "A192GCM":
		expected = 24
	case "A256GCM":
		expected = 32
	default:
		return fmt.Errorf("quotagate/jwt: unsupported content encryption: %s", m.contentEncryption)
	}
	if len(key) != expected {
		return fmt.Errorf("quotagate/jwt: encryption_key must be %d bytes for %s, got %d", expected, m.contentEncryption, len(key))
	}
	m.encKey = key
	return nil
}

// --- Token generation ---

// GenerateAccessToken issues a short-lived access token.
func (m *JWTManager) GenerateAccessToken(userID, tenantID string, roles []string) (string, error) {
	return m.generateToken(userID, tenantID, roles, "access", m.accessExpiry)
}

// GenerateRefreshToken issues a long-lived refresh token.
func (m *JWTManager) GenerateRefreshToken(userID, tenantID string, roles []string) (string, error) {
	return m.generateToken(userID, tenantID, roles, "refresh", m.refreshExpiry)
}

func (m *JWTManager) generateToken(userID, tenantID string, roles []string, tokenType string, expiry time.Duration) (string, error) {
	now := time.Now()
	jti, err := kexrandom.NewUUIDString()
	if err != nil {
		return "", fmt.Errorf("quotagate/jwt: generate jti failed: %w", err)
	}

	claims := Claims{
		Issuer:    m.issuer,
		Subject:   userID,
		Audience:  m.audience,
		ExpiresAt: now.Add(expiry).Unix(),
		NotBefore: now.Unix(),
		IssuedAt:  now.Unix(),
		JWTID:     jti,
		UserID:    userID,
		TenantID:  tenantID,
		Roles:     roles,
		TokenType: tokenType,
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	if m.mode == ModeJWE {
		return m.signThenEncrypt(payload)
	}
	return m.sign(payload)
}

// sign signs the payload. SM2 → pkg/crypto.Signer (through ProviderRegistry);
// HMAC/RSA/ECDSA → jwx.
func (m *JWTManager) sign(payload []byte) (string, error) {
	if isSM2Algorithm(m.signAlgorithm) {
		signer, err := pkgcrypto.NewSigner(pkgcrypto.SGD_SM3_SM2, m.sm2Priv, "")
		if err != nil {
			return "", fmt.Errorf("quotagate/jwt: create SM2 signer: %w", err)
		}
		signer.SetTokenType("JWT")
		return signer.Sign(payload)
	}
	signed, err := jws.Sign(payload, jws.WithKey(m.resolveSignAlgorithm(), m.signingKey()))
	if err != nil {
		return "", err
	}
	return string(signed), nil
}

// signThenEncrypt creates nested JWE: sign → encrypt.
// SM4 → pkg/crypto.EncryptJWE (through ProviderRegistry); AES → jwx.
func (m *JWTManager) signThenEncrypt(payload []byte) (string, error) {
	signed, err := m.sign(payload)
	if err != nil {
		return "", fmt.Errorf("quotagate/jwt: nested JWS sign failed: %w", err)
	}

	if isSM4Encryption(m.contentEncryption) {
		return pkgcrypto.EncryptJWE([]byte(signed), m.encKey)
	}

	encrypted, err := jwe.Encrypt(
		[]byte(signed),
		jwe.WithKey(jwa.DIRECT(), m.encKey),
		jwe.WithContentEncryption(m.resolveContentEncryption()),
	)
	if err != nil {
		return "", err
	}
	return string(encrypted), nil
}

func (m *JWTManager) signingKey() interface{} {
	if m.sm2Priv != nil {
		return nil
	}
	if m.stdSigner != nil {
		return m.stdSigner
	}
	return m.hmacKey
}

func (m *JWTManager) verificationKey() interface{} {
	if m.sm2Priv != nil {
		return nil
	}
	if m.stdSigner != nil {
		if rk, ok := m.stdSigner.(*rsa.PrivateKey); ok {
			return &rk.PublicKey
		}
		if ek, ok := m.stdSigner.(*ecdsa.PrivateKey); ok {
			return &ek.PublicKey
		}
		return nil
	}
	return m.hmacKey
}

// --- Token parsing ---

// ParseToken decodes and validates a JWS or JWE token.
func (m *JWTManager) ParseToken(tokenString string) (*Claims, error) {
	if m.mode == ModeJWE {
		return m.decryptThenParse(tokenString)
	}
	return m.parseAndVerify(tokenString)
}

// parseAndVerify parses JWS, verifies signature, validates claims.
func (m *JWTManager) parseAndVerify(tokenString string) (*Claims, error) {
	if isSM2Algorithm(m.signAlgorithm) {
		return m.verifySM2JWS(tokenString)
	}

	payload, err := jws.Verify(
		[]byte(tokenString),
		jws.WithKey(m.resolveSignAlgorithm(), m.verificationKey()),
	)
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if err := m.validateClaims(&claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

// verifySM2JWS verifies SM2 JWS via pkg/crypto ProviderRegistry.
func (m *JWTManager) verifySM2JWS(tokenString string) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	signingInput := []byte(parts[0] + "." + parts[1])
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	verifier, ok := pkgcrypto.DefaultRegistry.GetVerifier(pkgcrypto.SGD_SM3_SM2)
	if !ok {
		return nil, fmt.Errorf("quotagate/jwt: no SM2 verifier registered")
	}
	if err := verifier.Verify(context.Background(), signingInput, signature, m.sm2Pub); err != nil {
		return nil, ErrInvalidToken
	}

	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return nil, ErrInvalidToken
	}

	var claims Claims
	if err := json.Unmarshal(payload, &claims); err != nil {
		return nil, ErrInvalidToken
	}
	if err := m.validateClaims(&claims); err != nil {
		return nil, err
	}
	return &claims, nil
}

// decryptThenParse decrypts JWE then parses nested JWS.
// SM4 → pkg/crypto.DecryptJWE (through ProviderRegistry); AES → jwx.
func (m *JWTManager) decryptThenParse(tokenString string) (*Claims, error) {
	if isSM4Encryption(m.contentEncryption) {
		inner, err := pkgcrypto.DecryptJWE(tokenString, m.encKey)
		if err != nil {
			return nil, ErrInvalidToken
		}
		return m.parseAndVerify(string(inner))
	}

	payload, err := jwe.Decrypt([]byte(tokenString), jwe.WithKey(jwa.DIRECT(), m.encKey))
	if err != nil {
		return nil, ErrInvalidToken
	}
	return m.parseAndVerify(string(payload))
}

func (m *JWTManager) validateClaims(claims *Claims) error {
	if !claims.isValid() {
		return ErrExpiredToken
	}
	if m.issuer != "" && claims.Issuer != m.issuer {
		return ErrInvalidIssuer
	}
	if m.audience != "" && claims.Audience != m.audience {
		return ErrInvalidAudience
	}
	return nil
}
