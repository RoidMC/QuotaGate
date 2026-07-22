package kexrandom

import (
	"crypto/rand"
	"math/big"
)

const (
	// digits 是纯数字字符集
	// digits is the numeric character set.
	digits = "0123456789"

	// alphaDigits 是大写字母和数字混合字符集
	// alphaDigits is the alphanumeric character set (uppercase letters + digits).
	alphaDigits = "ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
)

// VerifyCode 生成指定长度的纯数字验证码
// VerifyCode generates a numeric verification code of the specified length.
func VerifyCode(length int) (string, error) {
	return randomString(length, digits)
}

// MustVerifyCode 生成纯数字验证码，失败时 panic
// MustVerifyCode generates a numeric verification code, panicking on error.
func MustVerifyCode(length int) string {
	code, err := VerifyCode(length)
	if err != nil {
		panic(err)
	}
	return code
}

// VerifyCodeAlpha 生成指定长度的大写字母+数字混合验证码
// VerifyCodeAlpha generates an alphanumeric verification code (A-Z, 0-9).
func VerifyCodeAlpha(length int) (string, error) {
	return randomString(length, alphaDigits)
}

// MustVerifyCodeAlpha 生成字母数字混合验证码，失败时 panic
// MustVerifyCodeAlpha generates an alphanumeric verification code, panicking on error.
func MustVerifyCodeAlpha(length int) string {
	code, err := VerifyCodeAlpha(length)
	if err != nil {
		panic(err)
	}
	return code
}

// randomString 从指定字符集中生成指定长度的随机字符串
// randomString generates a random string of given length from the provided charset.
func randomString(length int, charset string) (string, error) {
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
