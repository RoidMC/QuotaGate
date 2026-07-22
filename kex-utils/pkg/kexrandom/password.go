package kexrandom

import (
	"crypto/rand"
	"math/big"
)

const (
	// passwordLower 是小写字母字符集
	// passwordLower is the lowercase letter character set.
	passwordLower = "abcdefghijklmnopqrstuvwxyz"

	// passwordUpper 是大写字母字符集
	// passwordUpper is the uppercase letter character set.
	passwordUpper = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"

	// passwordDigits 是数字字符集
	// passwordDigits is the digit character set.
	passwordDigits = "0123456789"

	// passwordSpecial 是特殊字符集
	// passwordSpecial is the special character set.
	passwordSpecial = "!@#$%^&*()-_=+[]{}|;:,.<>?"

	// PasswordChars 是默认密码字符集（大小写字母+数字+特殊字符）
	// PasswordChars is the default password character set.
	PasswordChars = passwordLower + passwordUpper + passwordDigits + passwordSpecial
)

// Password 生成指定长度的随机密码，使用默认字符集
// Password generates a random password of the specified length using the default charset.
func Password(length int) (string, error) {
	return PasswordFromCharset(length, PasswordChars)
}

// MustPassword 生成随机密码，失败时 panic
// MustPassword generates a random password, panicking on error.
func MustPassword(length int) string {
	pwd, err := Password(length)
	if err != nil {
		panic(err)
	}
	return pwd
}

// PasswordFromCharset 从指定字符集中生成随机密码
// PasswordFromCharset generates a random password from the provided charset.
func PasswordFromCharset(length int, charset string) (string, error) {
	if length <= 0 {
		return "", nil
	}

	result := make([]byte, length)
	charsetLen := big.NewInt(int64(len(charset)))

	for i := 0; i < length; i++ {
		n, err := rand.Int(rand.Reader, charsetLen)
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}

	return string(result), nil
}

// SecurePassword 生成符合复杂度要求的随机密码
// 保证至少包含：1个小写字母、1个大写字母、1个数字、1个特殊字符
// SecurePassword generates a random password meeting complexity requirements.
// Guarantees at least: 1 lowercase, 1 uppercase, 1 digit, 1 special character.
func SecurePassword(length int) (string, error) {
	if length < 4 {
		return "", nil
	}

	// Ensure at least one of each required character type
	result := make([]byte, length)
	required := []string{passwordLower, passwordUpper, passwordDigits, passwordSpecial}

	for i, charset := range required {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			return "", err
		}
		result[i] = charset[n.Int64()]
	}

	// Fill the rest with the full charset
	fullLen := big.NewInt(int64(len(PasswordChars)))
	for i := 4; i < length; i++ {
		n, err := rand.Int(rand.Reader, fullLen)
		if err != nil {
			return "", err
		}
		result[i] = PasswordChars[n.Int64()]
	}

	// Shuffle the result to avoid predictable positions
	shuffleBytes(result)

	return string(result), nil
}

// MustSecurePassword 生成符合复杂度要求的随机密码，失败时 panic
// MustSecurePassword generates a secure password, panicking on error.
func MustSecurePassword(length int) string {
	pwd, err := SecurePassword(length)
	if err != nil {
		panic(err)
	}
	return pwd
}

// apiKeyChars 是 API 密钥字符集（URL 安全，无易混淆字符）
// apiKeyChars is the API key charset (URL-safe, no ambiguous characters).
const apiKeyChars = "ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789"

// APIKey 生成指定长度的 URL 安全随机 API 密钥
// 排除易混淆字符（0/O, 1/l/I）和 URL 特殊字符
// APIKey generates a URL-safe random API key of the specified length.
// Excludes ambiguous characters (0/O, 1/l/I) and URL special characters.
func APIKey(length int) (string, error) {
	return PasswordFromCharset(length, apiKeyChars)
}

// MustAPIKey 生成 API 密钥，失败时 panic
// MustAPIKey generates an API key, panicking on error.
func MustAPIKey(length int) string {
	key, err := APIKey(length)
	if err != nil {
		panic(err)
	}
	return key
}

// shuffleBytes 使用 Fisher-Yates 算法打乱字节顺序
// shuffleBytes shuffles the byte slice using the Fisher-Yates algorithm.
func shuffleBytes(b []byte) {
	for i := len(b) - 1; i > 0; i-- {
		n, _ := rand.Int(rand.Reader, big.NewInt(int64(i+1)))
		j := int(n.Int64())
		b[i], b[j] = b[j], b[i]
	}
}
