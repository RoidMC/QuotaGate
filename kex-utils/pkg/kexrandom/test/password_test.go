package kexrandom_test

import (
	"strings"
	"testing"

	"github.com/roidmc/kex-utils/pkg/kexrandom"
)

func TestPassword(t *testing.T) {
	pwd, err := kexrandom.Password(16)
	if err != nil {
		t.Fatalf("Password failed: %v", err)
	}
	if len(pwd) != 16 {
		t.Errorf("expected length 16, got %d", len(pwd))
	}
}

func TestPasswordZeroLength(t *testing.T) {
	pwd, err := kexrandom.Password(0)
	if err != nil {
		t.Fatalf("Password(0) failed: %v", err)
	}
	if pwd != "" {
		t.Errorf("expected empty string, got %q", pwd)
	}
}

func TestPasswordNegativeLength(t *testing.T) {
	pwd, err := kexrandom.Password(-1)
	if err != nil {
		t.Fatalf("Password(-1) failed: %v", err)
	}
	if pwd != "" {
		t.Errorf("expected empty string, got %q", pwd)
	}
}

func TestMustPassword(t *testing.T) {
	pwd := kexrandom.MustPassword(16)
	if len(pwd) != 16 {
		t.Errorf("expected length 16, got %d", len(pwd))
	}
}

func TestPasswordFromCharset(t *testing.T) {
	pwd, err := kexrandom.PasswordFromCharset(10, "ABC")
	if err != nil {
		t.Fatalf("PasswordFromCharset failed: %v", err)
	}
	if len(pwd) != 10 {
		t.Errorf("expected length 10, got %d", len(pwd))
	}
	for _, c := range pwd {
		if c != 'A' && c != 'B' && c != 'C' {
			t.Errorf("expected only A/B/C, got %q", c)
		}
	}
}

func TestPasswordContainsSpecialChars(t *testing.T) {
	pwd := kexrandom.MustPassword(32)

	// Verify all characters are from the expected charset
	for _, c := range pwd {
		if !strings.ContainsRune(kexrandom.PasswordChars, c) {
			t.Errorf("character %q not in allowed charset", c)
		}
	}
}

func TestSecurePassword(t *testing.T) {
	pwd, err := kexrandom.SecurePassword(16)
	if err != nil {
		t.Fatalf("SecurePassword failed: %v", err)
	}
	if len(pwd) != 16 {
		t.Errorf("expected length 16, got %d", len(pwd))
	}

	hasLower := false
	hasUpper := false
	hasDigit := false
	hasSpecial := false

	for _, c := range pwd {
		if c >= 'a' && c <= 'z' {
			hasLower = true
		} else if c >= 'A' && c <= 'Z' {
			hasUpper = true
		} else if c >= '0' && c <= '9' {
			hasDigit = true
		} else {
			hasSpecial = true
		}
	}

	if !hasLower {
		t.Error("secure password missing lowercase letter")
	}
	if !hasUpper {
		t.Error("secure password missing uppercase letter")
	}
	if !hasDigit {
		t.Error("secure password missing digit")
	}
	if !hasSpecial {
		t.Error("secure password missing special character")
	}
}

func TestSecurePasswordMinLength(t *testing.T) {
	pwd, err := kexrandom.SecurePassword(3)
	if err != nil {
		t.Fatalf("SecurePassword(3) failed: %v", err)
	}
	if pwd != "" {
		t.Errorf("expected empty string for length < 4, got %q", pwd)
	}
}

func TestSecurePasswordExactlyFour(t *testing.T) {
	pwd, err := kexrandom.SecurePassword(4)
	if err != nil {
		t.Fatalf("SecurePassword(4) failed: %v", err)
	}
	if len(pwd) != 4 {
		t.Errorf("expected length 4, got %d", len(pwd))
	}
}

func TestMustSecurePassword(t *testing.T) {
	pwd := kexrandom.MustSecurePassword(16)
	if len(pwd) != 16 {
		t.Errorf("expected length 16, got %d", len(pwd))
	}
}

func TestPasswordUniqueness(t *testing.T) {
	passwords := make(map[string]bool)
	for i := 0; i < 100; i++ {
		pwd, err := kexrandom.Password(16)
		if err != nil {
			t.Fatalf("Password failed: %v", err)
		}
		if passwords[pwd] {
			t.Fatalf("duplicate password generated: %s", pwd)
		}
		passwords[pwd] = true
	}
}

func TestSecurePasswordShuffle(t *testing.T) {
	// Run multiple times to verify shuffling works
	for i := 0; i < 10; i++ {
		pwd, err := kexrandom.SecurePassword(8)
		if err != nil {
			t.Fatalf("SecurePassword failed: %v", err)
		}
		if len(pwd) != 8 {
			t.Errorf("expected length 8, got %d", len(pwd))
		}
	}
}

func TestAPIKey(t *testing.T) {
	key, err := kexrandom.APIKey(32)
	if err != nil {
		t.Fatalf("APIKey failed: %v", err)
	}
	if len(key) != 32 {
		t.Errorf("expected length 32, got %d", len(key))
	}

	// Verify no ambiguous or URL-unsafe characters
	for _, c := range key {
		switch c {
		case '0', 'O', 'o', '1', 'l', 'I':
			t.Errorf("APIKey contains ambiguous character: %q", c)
		case '&', '=', '?', '#', '%', '+', '/', ' ':
			t.Errorf("APIKey contains URL-unsafe character: %q", c)
		}
	}
}

func TestMustAPIKey(t *testing.T) {
	key := kexrandom.MustAPIKey(32)
	if len(key) != 32 {
		t.Errorf("expected length 32, got %d", len(key))
	}
}

func TestAPIKeyCharset(t *testing.T) {
	key := kexrandom.MustAPIKey(100)
	for _, c := range key {
		if !strings.ContainsRune("ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789", c) {
			t.Errorf("character %q not in allowed API key charset", c)
		}
	}
}
