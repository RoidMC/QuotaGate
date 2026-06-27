package validator_test

import (
	"regexp"
	"testing"

	"github.com/roidmc/quotagate/internal/util/validator"
)

func TestEmailValidator(t *testing.T) {
	tests := []struct {
		email   string
		wantErr bool
	}{
		{"test@example.com", false},
		{"user.name+tag@example.co.uk", false},
		{"invalid", true},
		{"@example.com", true},
		{"test@", true},
		{"", true},
	}

	ev := validator.NewEmailValidator(nil, nil)
	for _, tt := range tests {
		t.Run(tt.email, func(t *testing.T) {
			err := ev.Validate(tt.email)
			if (err != nil) != tt.wantErr {
				t.Errorf("EmailValidator.Validate(%q) error = %v, wantErr %v", tt.email, err, tt.wantErr)
			}
		})
	}
}

func TestEmailValidatorBlacklist(t *testing.T) {
	ev := validator.NewEmailValidator(
		[]string{"baddomain.com", "spam.org"},
		nil,
	)

	if err := ev.Validate("user@baddomain.com"); err == nil {
		t.Error("expected error for blacklisted domain")
	}

	if err := ev.Validate("user@example.com"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestEmailValidatorWhitelist(t *testing.T) {
	ev := validator.NewEmailValidator(
		nil,
		[]string{"example.com", "company.org"},
	)

	if err := ev.Validate("user@example.com"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := ev.Validate("user@other.com"); err == nil {
		t.Error("expected error for non-whitelisted domain")
	}
}

func TestUsernameValidator(t *testing.T) {
	tests := []struct {
		username string
		wantErr  bool
	}{
		{"user123", false},
		{"user_name", false},
		{"ab", true},
		{"user@name", true},
		{"user name", true},
		{"verylongusernamethatexceeds", true},
		{"", true},
	}

	uv := validator.NewUsernameValidator(3, 16, nil)
	for _, tt := range tests {
		t.Run(tt.username, func(t *testing.T) {
			err := uv.Validate(tt.username)
			if (err != nil) != tt.wantErr {
				t.Errorf("UsernameValidator.Validate(%q) error = %v, wantErr %v", tt.username, err, tt.wantErr)
			}
		})
	}
}

func TestUsernameValidatorReserved(t *testing.T) {
	uv := validator.NewUsernameValidator(3, 16, []string{"admin", "root", "system"})

	if err := uv.Validate("admin"); err == nil {
		t.Error("expected error for reserved username")
	}

	if err := uv.Validate("Admin"); err == nil {
		t.Error("expected error for reserved username (case insensitive)")
	}

	if err := uv.Validate("user123"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPasswordValidator(t *testing.T) {
	tests := []struct {
		password string
		wantErr  bool
	}{
		{"Password123", false},
		{"Short1", true},
		{"password123", true},
		{"PASSWORD123", true},
		{"Password", true},
		{"", true},
	}

	pv := validator.NewPasswordValidator(8, 64)
	for _, tt := range tests {
		t.Run(tt.password, func(t *testing.T) {
			err := pv.Validate(tt.password)
			if (err != nil) != tt.wantErr {
				t.Errorf("PasswordValidator.Validate(%q) error = %v, wantErr %v", tt.password, err, tt.wantErr)
			}
		})
	}
}

func TestPasswordValidatorCustomComplexity(t *testing.T) {
	pv := validator.NewPasswordValidator(8, 64)
	pv.MinUppercase = 2
	pv.MinLowercase = 2
	pv.MinDigits = 2
	pv.MinSpecialChars = 1

	if err := pv.Validate("AAbb12!!"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := pv.Validate("Aabb12!!"); err == nil {
		t.Error("expected error for insufficient uppercase")
	}

	if err := pv.Validate("AAbb12aa"); err == nil {
		t.Error("expected error for missing special char")
	}
}

func TestPasswordValidatorSpecialChars(t *testing.T) {
	pv := validator.NewPasswordValidator(8, 64)
	pv.MinSpecialChars = 1

	if err := pv.Validate("Password1!"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := pv.Validate("Password1@"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestPasswordValidatorInvalidChar(t *testing.T) {
	pv := validator.NewPasswordValidator(8, 64)

	if err := pv.Validate("Password1`"); err == nil {
		t.Error("expected error for invalid character")
	}
}

func TestPasswordValidatorMaxLength(t *testing.T) {
	pv := validator.NewPasswordValidator(8, 10)

	if err := pv.Validate("Password12"); err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := pv.Validate("Password123"); err == nil {
		t.Error("expected error for exceeding max length")
	}
}

func TestValidateWithRegex(t *testing.T) {
	tests := []struct {
		input   string
		pattern string
		wantErr bool
	}{
		{"hello123", `^[a-z0-9]+$`, false},
		{"HELLO", `^[a-z0-9]+$`, true},
		{"test", `^[a-z]+$`, false},
		{"test123", `^[a-z]+$`, true},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			err := validator.ValidateWithRegex(tt.input, tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateWithRegex(%q, %q) error = %v, wantErr %v",
					tt.input, tt.pattern, err, tt.wantErr)
			}
		})
	}
}

func TestValidator(t *testing.T) {
	v := &validator.Validator{
		MinLength:  3,
		MaxLength:  10,
		AllowEmpty: false,
	}

	if err := v.Validate("abc"); err != nil {
		t.Errorf("Validator.Validate(abc) error = %v", err)
	}

	if err := v.Validate("ab"); err == nil {
		t.Error("Validator.Validate(ab) should error")
	}

	if err := v.Validate("verylongstring"); err == nil {
		t.Error("Validator.Validate(verylongstring) should error")
	}

	if err := v.Validate(""); err == nil {
		t.Error("Validator.Validate(empty) should error")
	}
}

func TestValidatorAllowEmpty(t *testing.T) {
	v := &validator.Validator{
		MinLength:  3,
		MaxLength:  10,
		AllowEmpty: true,
	}

	if err := v.Validate(""); err != nil {
		t.Errorf("Validator.Validate(empty) with AllowEmpty=true error = %v", err)
	}
}

func TestValidatorWithRegex(t *testing.T) {
	v := &validator.Validator{
		MinLength: 3,
		MaxLength: 10,
		Regex:     regexp.MustCompile(`^[a-z]+$`),
	}

	if err := v.Validate("abc"); err != nil {
		t.Errorf("Validator.Validate(abc) error = %v", err)
	}

	if err := v.Validate("abc123"); err == nil {
		t.Error("Validator.Validate(abc123) should error for regex mismatch")
	}
}

func TestValidatorSpecialChars(t *testing.T) {
	v := &validator.Validator{
		MinLength:    3,
		MaxLength:    10,
		SpecialChars: "!@#",
	}

	if err := v.Validate("abc"); err != nil {
		t.Errorf("Validator.Validate(abc) error = %v", err)
	}

	if err := v.Validate("ab!"); err != nil {
		t.Errorf("Validator.Validate(ab!) error = %v", err)
	}

	if err := v.Validate("ab$"); err == nil {
		t.Error("Validator.Validate(ab$) should error for invalid special char")
	}
}
