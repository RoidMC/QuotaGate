package kexvalidator

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// Validator 是通用验证器结构体
// Validator is a generic validator struct.
type Validator struct {
	MinLength    int
	MaxLength    int
	Regex        *regexp.Regexp
	AllowEmpty   bool
	SpecialChars string
}

// Validate 执行通用验证
// Validate performs generic validation.
func (v *Validator) Validate(input string) error {
	if input == "" {
		if v.AllowEmpty {
			return nil
		}
		return fmt.Errorf("kex-utils/validator: input is empty")
	}

	length := len([]rune(input))
	if v.MinLength > 0 && length < v.MinLength {
		return fmt.Errorf("kex-utils/validator: input length %d is less than minimum %d", length, v.MinLength)
	}
	if v.MaxLength > 0 && length > v.MaxLength {
		return fmt.Errorf("kex-utils/validator: input length %d exceeds maximum %d", length, v.MaxLength)
	}

	if v.Regex != nil && !v.Regex.MatchString(input) {
		return fmt.Errorf("kex-utils/validator: input does not match required pattern")
	}

	if v.SpecialChars != "" {
		for _, c := range input {
			if !strings.ContainsRune(v.SpecialChars, c) && !unicode.IsLetter(c) && !unicode.IsDigit(c) {
				return fmt.Errorf("kex-utils/validator: input contains invalid character: %q", c)
			}
		}
	}

	return nil
}

// EmailValidator 是邮箱验证器
// EmailValidator is an email validator with blacklist/whitelist support.
type EmailValidator struct {
	Regex     *regexp.Regexp
	Blacklist []string
	Whitelist []string
}

// NewEmailValidator 创建邮箱验证器
// NewEmailValidator creates an email validator with optional blacklist/whitelist.
func NewEmailValidator(blacklist, whitelist []string) *EmailValidator {
	return &EmailValidator{
		Regex:     regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`),
		Blacklist: blacklist,
		Whitelist: whitelist,
	}
}

// Validate 验证邮箱
// Validate validates email format and domain restrictions.
func (ev *EmailValidator) Validate(email string) error {
	if !ev.Regex.MatchString(email) {
		return fmt.Errorf("kex-utils/validator: invalid email format")
	}

	// Extract domain
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return fmt.Errorf("kex-utils/validator: invalid email format")
	}
	domain := parts[1]

	// Check whitelist
	if len(ev.Whitelist) > 0 {
		allowed := false
		for _, d := range ev.Whitelist {
			if strings.EqualFold(domain, d) {
				allowed = true
				break
			}
		}
		if !allowed {
			return fmt.Errorf("kex-utils/validator: email domain %s is not in whitelist", domain)
		}
	}

	// Check blacklist
	for _, d := range ev.Blacklist {
		if strings.EqualFold(domain, d) {
			return fmt.Errorf("kex-utils/validator: email domain %s is in blacklist", domain)
		}
	}

	return nil
}

// UsernameValidator 是用户名验证器
// UsernameValidator is a username validator with configurable rules.
type UsernameValidator struct {
	MinLength     int
	MaxLength     int
	Regex         *regexp.Regexp
	AllowEmpty    bool
	ReservedNames []string
}

// NewUsernameValidator 创建用户名验证器
// NewUsernameValidator creates a username kexvalidator.
func NewUsernameValidator(minLen, maxLen int, reserved []string) *UsernameValidator {
	return &UsernameValidator{
		MinLength:     minLen,
		MaxLength:     maxLen,
		Regex:         regexp.MustCompile(`^[a-zA-Z0-9_]+$`),
		AllowEmpty:    false,
		ReservedNames: reserved,
	}
}

// Validate 验证用户名
// Validate validates username.
func (uv *UsernameValidator) Validate(username string) error {
	if !uv.AllowEmpty && username == "" {
		return fmt.Errorf("kex-utils/validator: username is empty")
	}

	length := len([]rune(username))
	if uv.MinLength > 0 && length < uv.MinLength {
		return fmt.Errorf("kex-utils/validator: username length %d is less than minimum %d", length, uv.MinLength)
	}
	if uv.MaxLength > 0 && length > uv.MaxLength {
		return fmt.Errorf("kex-utils/validator: username length %d exceeds maximum %d", length, uv.MaxLength)
	}

	if uv.Regex != nil && !uv.Regex.MatchString(username) {
		return fmt.Errorf("kex-utils/validator: username contains invalid characters")
	}

	for _, reserved := range uv.ReservedNames {
		if strings.EqualFold(username, reserved) {
			return fmt.Errorf("kex-utils/validator: username %s is reserved", username)
		}
	}

	return nil
}

// PasswordValidator 是密码验证器
// PasswordValidator is a password validator with configurable complexity rules.
type PasswordValidator struct {
	MinLength       int
	MaxLength       int
	MinUppercase    int
	MinLowercase    int
	MinDigits       int
	MinSpecialChars int
	SpecialChars    string
	AllowEmpty      bool
}

// NewPasswordValidator 创建密码验证器
// NewPasswordValidator creates a password validator.
func NewPasswordValidator(minLen, maxLen int) *PasswordValidator {
	return &PasswordValidator{
		MinLength:       minLen,
		MaxLength:       maxLen,
		MinUppercase:    1,
		MinLowercase:    1,
		MinDigits:       1,
		MinSpecialChars: 0,
		SpecialChars:    "!@#$%^&*()-_=+[]{}|;:,.<>?",
		AllowEmpty:      false,
	}
}

// Validate 验证密码复杂度
// Validate validates password complexity.
func (pv *PasswordValidator) Validate(password string) error {
	if !pv.AllowEmpty && password == "" {
		return fmt.Errorf("kex-utils/validator: password is empty")
	}

	length := len([]rune(password))
	if pv.MinLength > 0 && length < pv.MinLength {
		return fmt.Errorf("kex-utils/validator: password length %d is less than minimum %d", length, pv.MinLength)
	}
	if pv.MaxLength > 0 && length > pv.MaxLength {
		return fmt.Errorf("kex-utils/validator: password length %d exceeds maximum %d", length, pv.MaxLength)
	}

	upperCount := 0
	lowerCount := 0
	digitCount := 0
	specialCount := 0

	for _, c := range password {
		if unicode.IsUpper(c) {
			upperCount++
		} else if unicode.IsLower(c) {
			lowerCount++
		} else if unicode.IsDigit(c) {
			digitCount++
		} else if strings.ContainsRune(pv.SpecialChars, c) {
			specialCount++
		} else {
			return fmt.Errorf("kex-utils/validator: password contains invalid character: %q", c)
		}
	}

	if upperCount < pv.MinUppercase {
		return fmt.Errorf("kex-utils/validator: password must contain at least %d uppercase letter(s)", pv.MinUppercase)
	}
	if lowerCount < pv.MinLowercase {
		return fmt.Errorf("kex-utils/validator: password must contain at least %d lowercase letter(s)", pv.MinLowercase)
	}
	if digitCount < pv.MinDigits {
		return fmt.Errorf("kex-utils/validator: password must contain at least %d digit(s)", pv.MinDigits)
	}
	if specialCount < pv.MinSpecialChars {
		return fmt.Errorf("kex-utils/validator: password must contain at least %d special character(s)", pv.MinSpecialChars)
	}

	return nil
}

// ValidateWithRegex 使用自定义正则验证
// ValidateWithRegex validates input with a custom regex pattern.
func ValidateWithRegex(input string, pattern string) error {
	re, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("kex-utils/validator: invalid regex pattern: %w", err)
	}
	if !re.MatchString(input) {
		return fmt.Errorf("kex-utils/validator: input does not match pattern: %s", pattern)
	}
	return nil
}
