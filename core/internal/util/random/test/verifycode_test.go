package random_test

import (
	"strings"
	"testing"

	"github.com/roidmc/quotagate/internal/util/random"
)

func TestVerifyCode(t *testing.T) {
	code, err := random.VerifyCode(6)
	if err != nil {
		t.Fatalf("VerifyCode failed: %v", err)
	}
	if len(code) != 6 {
		t.Errorf("expected length 6, got %d", len(code))
	}
	for _, c := range code {
		if c < '0' || c > '9' {
			t.Errorf("expected only digits, got %q", c)
		}
	}
}

func TestVerifyCodeZeroLength(t *testing.T) {
	code, err := random.VerifyCode(0)
	if err != nil {
		t.Fatalf("VerifyCode(0) failed: %v", err)
	}
	if code != "" {
		t.Errorf("expected empty string, got %q", code)
	}
}

func TestVerifyCodeNegativeLength(t *testing.T) {
	code, err := random.VerifyCode(-1)
	if err != nil {
		t.Fatalf("VerifyCode(-1) failed: %v", err)
	}
	if code != "" {
		t.Errorf("expected empty string, got %q", code)
	}
}

func TestMustVerifyCode(t *testing.T) {
	code := random.MustVerifyCode(6)
	if len(code) != 6 {
		t.Errorf("expected length 6, got %d", len(code))
	}
}

func TestVerifyCodeAlpha(t *testing.T) {
	code, err := random.VerifyCodeAlpha(8)
	if err != nil {
		t.Fatalf("VerifyCodeAlpha failed: %v", err)
	}
	if len(code) != 8 {
		t.Errorf("expected length 8, got %d", len(code))
	}
	for _, c := range code {
		if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'Z')) {
			t.Errorf("expected only A-Z or 0-9, got %q", c)
		}
	}
}

func TestVerifyCodeAlphaZeroLength(t *testing.T) {
	code, err := random.VerifyCodeAlpha(0)
	if err != nil {
		t.Fatalf("VerifyCodeAlpha(0) failed: %v", err)
	}
	if code != "" {
		t.Errorf("expected empty string, got %q", code)
	}
}

func TestMustVerifyCodeAlpha(t *testing.T) {
	code := random.MustVerifyCodeAlpha(8)
	if len(code) != 8 {
		t.Errorf("expected length 8, got %d", len(code))
	}
}

func TestVerifyCodeUniqueness(t *testing.T) {
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code, err := random.VerifyCode(6)
		if err != nil {
			t.Fatalf("VerifyCode failed: %v", err)
		}
		if codes[code] {
			t.Fatalf("duplicate code generated: %s", code)
		}
		codes[code] = true
	}
}

func TestVerifyCodeAlphaUniqueness(t *testing.T) {
	codes := make(map[string]bool)
	for i := 0; i < 100; i++ {
		code, err := random.VerifyCodeAlpha(8)
		if err != nil {
			t.Fatalf("VerifyCodeAlpha failed: %v", err)
		}
		if codes[code] {
			t.Fatalf("duplicate code generated: %s", code)
		}
		codes[code] = true
	}
}

func TestVerifyCodeDistribution(t *testing.T) {
	const length = 1000
	code, err := random.VerifyCode(length)
	if err != nil {
		t.Fatalf("VerifyCode failed: %v", err)
	}
	count := make(map[rune]int)
	for _, c := range code {
		count[c]++
	}
	expected := float64(length) / 10.0
	for c := '0'; c <= '9'; c++ {
		actual := float64(count[c])
		diff := actual - expected
		if diff < 0 {
			diff = -diff
		}
		// Allow 30% deviation for random distribution
		if diff/expected > 0.3 {
			t.Errorf("digit %q count %d deviates too much from expected %.0f", c, count[c], expected)
		}
	}
}

func TestVerifyCodeAlphaCharset(t *testing.T) {
	code := random.MustVerifyCodeAlpha(100)
	for _, c := range code {
		if !strings.ContainsRune("ABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789", c) {
			t.Errorf("character %q not in allowed charset", c)
		}
	}
}
