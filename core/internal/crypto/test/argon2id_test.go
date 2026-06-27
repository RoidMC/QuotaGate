package crypto_test

import (
	"strings"
	"testing"

	"github.com/roidmc/quotagate/internal/crypto"
)

func TestArgon2idHash(t *testing.T) {
	password := "testPassword123!"

	params := crypto.Argon2idParams{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hash, err := crypto.Argon2idHash(password, params)
	if err != nil {
		t.Fatalf("Argon2idHash failed: %v", err)
	}

	if !strings.HasPrefix(hash, "$argon2id$") {
		t.Errorf("hash should start with $argon2id$, got: %s", hash)
	}

	parts := strings.Split(hash, "$")
	if len(parts) != 6 {
		t.Errorf("hash should have 6 parts, got %d", len(parts))
	}
}

func TestArgon2idVerify(t *testing.T) {
	password := "testPassword123!"

	hash, err := crypto.Argon2idHash(password, crypto.DefaultArgon2idParams)
	if err != nil {
		t.Fatalf("Argon2idHash failed: %v", err)
	}

	valid, err := crypto.Argon2idVerify(password, hash)
	if err != nil {
		t.Fatalf("Argon2idVerify failed: %v", err)
	}

	if !valid {
		t.Error("password should be valid")
	}
}

func TestArgon2idVerifyWrongPassword(t *testing.T) {
	password := "correctPassword"
	wrongPassword := "wrongPassword"

	hash, _ := crypto.Argon2idHash(password, crypto.DefaultArgon2idParams)

	valid, err := crypto.Argon2idVerify(wrongPassword, hash)
	if err != nil {
		t.Fatalf("Argon2idVerify failed: %v", err)
	}

	if valid {
		t.Error("wrong password should not be valid")
	}
}

func TestArgon2idDifferentHashesForSamePassword(t *testing.T) {
	password := "samePassword123"

	hash1, _ := crypto.Argon2idHash(password, crypto.DefaultArgon2idParams)
	hash2, _ := crypto.Argon2idHash(password, crypto.DefaultArgon2idParams)

	if hash1 == hash2 {
		t.Error("different hashes should be generated for same password due to random salt")
	}

	valid1, _ := crypto.Argon2idVerify(password, hash1)
	valid2, _ := crypto.Argon2idVerify(password, hash2)

	if !valid1 || !valid2 {
		t.Error("both hashes should verify the same password")
	}
}

func TestArgon2idInvalidHashFormat(t *testing.T) {
	password := "testPassword"

	invalidHashes := []string{
		"invalidhash",
		"$argon2id$invalid",
		"$argon2id$v=19$m=65536,t=3,p=2$salt",
		"$argon2i$v=19$m=65536,t=3,p=2$c2FsdA$hash",
	}

	for _, invalidHash := range invalidHashes {
		_, err := crypto.Argon2idVerify(password, invalidHash)
		if err == nil {
			t.Errorf("expected error for invalid hash: %s", invalidHash)
		}
	}
}

func TestArgon2idMustHash(t *testing.T) {
	password := "testPassword123"

	hash := crypto.MustArgon2idHash(password)

	if hash == "" {
		t.Error("MustArgon2idHash returned empty string")
	}

	valid, _ := crypto.Argon2idVerify(password, hash)
	if !valid {
		t.Error("hashed password should verify")
	}
}

func TestArgon2idEmptyPassword(t *testing.T) {
	password := ""

	hash, err := crypto.Argon2idHash(password, crypto.DefaultArgon2idParams)
	if err != nil {
		t.Fatalf("Argon2idHash failed for empty password: %v", err)
	}

	valid, err := crypto.Argon2idVerify(password, hash)
	if err != nil {
		t.Fatalf("Argon2idVerify failed: %v", err)
	}

	if !valid {
		t.Error("empty password should verify")
	}
}

func TestArgon2idLongPassword(t *testing.T) {
	password := strings.Repeat("a", 1000)

	hash, err := crypto.Argon2idHash(password, crypto.DefaultArgon2idParams)
	if err != nil {
		t.Fatalf("Argon2idHash failed for long password: %v", err)
	}

	valid, err := crypto.Argon2idVerify(password, hash)
	if err != nil {
		t.Fatalf("Argon2idVerify failed: %v", err)
	}

	if !valid {
		t.Error("long password should verify")
	}
}

func TestArgon2idUnicodePassword(t *testing.T) {
	passwords := []string{
		"密码测试123",
		"パスワード",
		"🔐🔒🗝️",
		"café",
	}

	for _, password := range passwords {
		hash, err := crypto.Argon2idHash(password, crypto.DefaultArgon2idParams)
		if err != nil {
			t.Fatalf("Argon2idHash failed for unicode password '%s': %v", password, err)
		}

		valid, err := crypto.Argon2idVerify(password, hash)
		if err != nil {
			t.Fatalf("Argon2idVerify failed: %v", err)
		}

		if !valid {
			t.Errorf("unicode password '%s' should verify", password)
		}
	}
}

func TestArgon2idDefaultParams(t *testing.T) {
	if crypto.DefaultArgon2idParams.Memory != 64*1024 {
		t.Errorf("expected default memory 65536, got %d", crypto.DefaultArgon2idParams.Memory)
	}
	if crypto.DefaultArgon2idParams.Iterations != 3 {
		t.Errorf("expected default iterations 3, got %d", crypto.DefaultArgon2idParams.Iterations)
	}
	if crypto.DefaultArgon2idParams.Parallelism != 2 {
		t.Errorf("expected default parallelism 2, got %d", crypto.DefaultArgon2idParams.Parallelism)
	}
	if crypto.DefaultArgon2idParams.SaltLength != 16 {
		t.Errorf("expected default salt length 16, got %d", crypto.DefaultArgon2idParams.SaltLength)
	}
	if crypto.DefaultArgon2idParams.KeyLength != 32 {
		t.Errorf("expected default key length 32, got %d", crypto.DefaultArgon2idParams.KeyLength)
	}
}
