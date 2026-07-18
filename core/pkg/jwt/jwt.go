package jwt

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/emmansun/gmsm/sm2"
	"github.com/lestrrat-go/jwx/v4/jwa"
	"github.com/lestrrat-go/jwx/v4/jwe"
	"github.com/lestrrat-go/jwx/v4/jws"
	kexcrypto "github.com/roidmc/quotagate/internal/crypto"
	kexrandom "github.com/roidmc/quotagate/internal/util/random"
)

var (
	ErrInvalidToken    = errors.New("quotagate/jwt: invalid token")
	ErrExpiredToken    = errors.New("quotagate/jwt: token expired")
	ErrInvalidIssuer   = errors.New("quotagate/jwt: invalid issuer")
	ErrInvalidAudience = errors.New("quotagate/jwt: invalid audience")
)

type Mode string

const (
	ModeJWS Mode = "jws"
	ModeJWE Mode = "jwe"
)

type Claims struct {
	Issuer    string                 `json:"iss,omitempty"`
	Subject   string                 `json:"sub,omitempty"`
	Audience  string                 `json:"aud,omitempty"`
	ExpiresAt int64                  `json:"exp"`
	NotBefore int64                  `json:"nbf"`
	IssuedAt  int64                  `json:"iat"`
	JWTID     string                 `json:"jti,omitempty"`
	UserID    string                 `json:"uid"`
	Role      string                 `json:"role"`
	TenantID  string                 `json:"tid,omitempty"`
	Roles     []string               `json:"roles,omitempty"`
	TokenType string                 `json:"type"`
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

type sm2KeyPair struct {
	privateKey *sm2.PrivateKey
	publicKey  *ecdsa.PublicKey
}

type JWTManager struct {
	mode              Mode
	hmacKey           []byte
	signer            crypto.Signer
	sm2Keys           *sm2KeyPair
	encKey            []byte
	signAlgorithm     string
	contentEncryption string
	issuer            string
	audience          string
	accessExpiry      time.Duration
	refreshExpiry     time.Duration
}

type ManagerOption func(*JWTManager)

func WithIssuer(issuer string) ManagerOption {
	return func(m *JWTManager) {
		m.issuer = issuer
	}
}

func WithAudience(audience string) ManagerOption {
	return func(m *JWTManager) {
		m.audience = audience
	}
}

func WithSignAlgorithm(alg string) ManagerOption {
	return func(m *JWTManager) {
		m.signAlgorithm = alg
	}
}

func WithContentEncryption(alg string) ManagerOption {
	return func(m *JWTManager) {
		m.contentEncryption = alg
	}
}

func isHMACAlgorithm(alg string) bool {
	switch alg {
	case "HS256", "HS384", "HS512":
		return true
	default:
		return false
	}
}

func isRSAAlgorithm(alg string) bool {
	switch alg {
	case "RS256", "RS384", "RS512", "PS256", "PS384", "PS512":
		return true
	default:
		return false
	}
}

func isECDSAAlgorithm(alg string) bool {
	switch alg {
	case "ES256", "ES384", "ES512":
		return true
	default:
		return false
	}
}

func isSM2Algorithm(alg string) bool {
	return strings.EqualFold(alg, "SM2") || strings.EqualFold(alg, "SM2-SM3")
}

func isSM4Encryption(alg string) bool {
	return strings.EqualFold(alg, "SM4-GCM") || strings.EqualFold(alg, "SM4")
}

func isStandardContentEncryption(alg string) bool {
	switch alg {
	case "A128GCM", "A192GCM", "A256GCM":
		return true
	default:
		return false
	}
}

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

func (m *JWTManager) signingKey() interface{} {
	if m.sm2Keys != nil {
		return nil
	}
	if m.signer != nil {
		return m.signer
	}
	return m.hmacKey
}

func (m *JWTManager) verificationKey() interface{} {
	if m.sm2Keys != nil {
		return nil
	}
	if m.signer != nil {
		return m.signer.Public()
	}
	return m.hmacKey
}

func NewManager(mode Mode, signKey string, encKey string, accessExpiry, refreshExpiry time.Duration, opts ...ManagerOption) (*JWTManager, error) {
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
	return fmt.Errorf("quotagate/jwt: unsupported sign algorithm: %s (supported: HS256, HS384, HS512, RS256, RS384, RS512, PS256, PS384, PS512, ES256, ES384, ES512, SM2)", m.signAlgorithm)
}

func (m *JWTManager) initSM2Key(signKey string) error {
	privKeyBytes, err := base64.RawURLEncoding.DecodeString(signKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: sign_key must be base64url-encoded SM2 private key: %w", err)
	}
	privKey, err := kexcrypto.SM2NewPrivateKey(privKeyBytes)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: invalid SM2 private key: %w", err)
	}
	m.sm2Keys = &sm2KeyPair{
		privateKey: privKey,
		publicKey:  &privKey.PublicKey,
	}
	return nil
}

func (m *JWTManager) initRSAKey(signKey string) error {
	keyBytes, err := base64.RawURLEncoding.DecodeString(signKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: sign_key must be base64url-encoded RSA private key: %w", err)
	}
	key, err := x509.ParsePKCS8PrivateKey(keyBytes)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: invalid RSA private key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return fmt.Errorf("quotagate/jwt: sign_key is not an RSA private key")
	}
	m.signer = rsaKey
	return nil
}

func (m *JWTManager) initECDSAKey(signKey string) error {
	keyBytes, err := base64.RawURLEncoding.DecodeString(signKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: sign_key must be base64url-encoded ECDSA private key: %w", err)
	}
	key, err := x509.ParsePKCS8PrivateKey(keyBytes)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: invalid ECDSA private key: %w", err)
	}
	ecKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return fmt.Errorf("quotagate/jwt: sign_key is not an ECDSA private key")
	}
	m.signer = ecKey
	return nil
}

func (m *JWTManager) initEncKey(encKey string) error {
	if isSM4Encryption(m.contentEncryption) {
		return m.initSM4Key(encKey)
	}
	if isStandardContentEncryption(m.contentEncryption) {
		return m.initAESKey(encKey)
	}
	return fmt.Errorf("quotagate/jwt: unsupported content encryption algorithm: %s (supported: A128GCM, A192GCM, A256GCM, SM4-GCM)", m.contentEncryption)
}

func (m *JWTManager) initSM4Key(encKey string) error {
	key, err := base64.RawURLEncoding.DecodeString(encKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: encryption_key must be base64url-encoded key: %w", err)
	}
	if len(key) != kexcrypto.SM4BlockSize {
		return fmt.Errorf("quotagate/jwt: encryption_key must be exactly %d bytes for SM4-GCM, got %d", kexcrypto.SM4BlockSize, len(key))
	}
	m.encKey = key
	return nil
}

func (m *JWTManager) initAESKey(encKey string) error {
	key, err := base64.RawURLEncoding.DecodeString(encKey)
	if err != nil {
		return fmt.Errorf("quotagate/jwt: encryption_key must be base64url-encoded key: %w", err)
	}
	switch m.contentEncryption {
	case "A128GCM":
		if len(key) != 16 {
			return fmt.Errorf("quotagate/jwt: encryption_key must be exactly 16 bytes for A128GCM, got %d", len(key))
		}
	case "A192GCM":
		if len(key) != 24 {
			return fmt.Errorf("quotagate/jwt: encryption_key must be exactly 24 bytes for A192GCM, got %d", len(key))
		}
	case "A256GCM":
		if len(key) != 32 {
			return fmt.Errorf("quotagate/jwt: encryption_key must be exactly 32 bytes for A256GCM, got %d", len(key))
		}
	}
	m.encKey = key
	return nil
}

func (m *JWTManager) generateToken(userID, role, tenantID string, roles []string, tokenType string, expiry time.Duration) (string, error) {
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
		Role:      role,
		TenantID:  tenantID,
		Roles:     roles,
		TokenType: tokenType,
	}

	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}

	if m.mode == ModeJWE {
		return m.generateJWE(payload)
	}
	return m.generateJWS(payload)
}

func (m *JWTManager) generateJWS(payload []byte) (string, error) {
	if isSM2Algorithm(m.signAlgorithm) {
		return m.generateSM2JWS(payload)
	}
	signed, err := jws.Sign(payload, jws.WithKey(m.resolveSignAlgorithm(), m.signingKey()))
	if err != nil {
		return "", err
	}
	return string(signed), nil
}

func (m *JWTManager) generateSM2JWS(payload []byte) (string, error) {
	header := map[string]interface{}{
		"alg": "SM2-SM3",
		"typ": "JWT",
	}
	headerJSON, err := json.Marshal(header)
	if err != nil {
		return "", err
	}

	headerB64 := base64.RawURLEncoding.EncodeToString(headerJSON)
	payloadB64 := base64.RawURLEncoding.EncodeToString(payload)
	signingInput := headerB64 + "." + payloadB64

	hash := kexcrypto.SM3HashString(signingInput)
	signature, err := kexcrypto.SM2Sign(m.sm2Keys.privateKey, hash)
	if err != nil {
		return "", fmt.Errorf("quotagate/jwt: SM2 sign failed: %w", err)
	}

	signatureB64 := base64.RawURLEncoding.EncodeToString(signature)
	return headerB64 + "." + payloadB64 + "." + signatureB64, nil
}

func (m *JWTManager) generateJWE(payload []byte) (string, error) {
	signed, err := m.generateJWS(payload)
	if err != nil {
		return "", fmt.Errorf("quotagate/jwt: nested JWS sign failed: %w", err)
	}

	if isSM4Encryption(m.contentEncryption) {
		return m.generateSM4JWE([]byte(signed))
	}
	encrypted, err := jwe.Encrypt([]byte(signed), jwe.WithKey(jwa.DIRECT(), m.encKey), jwe.WithContentEncryption(m.resolveContentEncryption()))
	if err != nil {
		return "", err
	}
	return string(encrypted), nil
}

func (m *JWTManager) generateSM4JWE(payload []byte) (string, error) {
	protectedHeader := map[string]interface{}{
		"alg": "dir",
		"enc": "SM4-GCM",
		"typ": "JWT",
		"cty": "JWT",
	}
	protectedJSON, err := json.Marshal(protectedHeader)
	if err != nil {
		return "", err
	}

	protectedB64 := base64.RawURLEncoding.EncodeToString(protectedJSON)
	encryptedKey := ""

	iv := make([]byte, 12)
	if _, err := rand.Read(iv); err != nil {
		return "", fmt.Errorf("quotagate/jwt: generate IV failed: %w", err)
	}

	aad := []byte(protectedB64)
	sealed, err := kexcrypto.SM4EncryptGCMWithNonce(m.encKey, iv, payload, aad)
	if err != nil {
		return "", fmt.Errorf("quotagate/jwt: SM4-GCM encrypt failed: %w", err)
	}

	tagSize := len(sealed) - len(payload)
	ciphertext := sealed[:len(sealed)-tagSize]
	tag := sealed[len(sealed)-tagSize:]

	ivB64 := base64.RawURLEncoding.EncodeToString(iv)
	ciphertextB64 := base64.RawURLEncoding.EncodeToString(ciphertext)
	tagB64 := base64.RawURLEncoding.EncodeToString(tag)

	return protectedB64 + "." + encryptedKey + "." + ivB64 + "." + ciphertextB64 + "." + tagB64, nil
}

func (m *JWTManager) GenerateAccessToken(userID, role, tenantID string, roles []string) (string, error) {
	return m.generateToken(userID, role, tenantID, roles, "access", m.accessExpiry)
}

func (m *JWTManager) GenerateRefreshToken(userID, role, tenantID string, roles []string) (string, error) {
	return m.generateToken(userID, role, tenantID, roles, "refresh", m.refreshExpiry)
}

func (m *JWTManager) ParseToken(tokenString string) (*Claims, error) {
	if m.mode == ModeJWE {
		return m.parseJWE(tokenString)
	}
	return m.parseJWS(tokenString)
}

func (m *JWTManager) parseJWS(tokenString string) (*Claims, error) {
	if isSM2Algorithm(m.signAlgorithm) {
		return m.parseSM2JWS(tokenString)
	}

	payload, err := jws.Verify([]byte(tokenString), jws.WithKey(m.resolveSignAlgorithm(), m.verificationKey()))
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

func (m *JWTManager) parseSM2JWS(tokenString string) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 3 {
		return nil, ErrInvalidToken
	}

	signingInput := parts[0] + "." + parts[1]
	signature, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	hash := kexcrypto.SM3HashString(signingInput)
	if !kexcrypto.SM2Verify(m.sm2Keys.publicKey, hash, signature) {
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

func (m *JWTManager) parseJWE(tokenString string) (*Claims, error) {
	if isSM4Encryption(m.contentEncryption) {
		return m.parseSM4JWE(tokenString)
	}

	payload, err := jwe.Decrypt([]byte(tokenString), jwe.WithKey(jwa.DIRECT(), m.encKey))
	if err != nil {
		return nil, ErrInvalidToken
	}

	return m.parseJWS(string(payload))
}

func (m *JWTManager) parseSM4JWE(tokenString string) (*Claims, error) {
	parts := strings.Split(tokenString, ".")
	if len(parts) != 5 {
		return nil, ErrInvalidToken
	}

	if parts[4] == "" {
		return nil, ErrInvalidToken
	}

	iv, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return nil, ErrInvalidToken
	}

	ciphertext, err := base64.RawURLEncoding.DecodeString(parts[3])
	if err != nil {
		return nil, ErrInvalidToken
	}

	tag, err := base64.RawURLEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, ErrInvalidToken
	}

	aad := []byte(parts[0])
	sealed := make([]byte, len(ciphertext)+len(tag))
	copy(sealed, ciphertext)
	copy(sealed[len(ciphertext):], tag)

	payload, err := kexcrypto.SM4DecryptGCMWithNonce(m.encKey, iv, sealed, aad)
	if err != nil {
		return nil, ErrInvalidToken
	}

	return m.parseJWS(string(payload))
}
