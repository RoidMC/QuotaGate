package crypto_test

import (
	"testing"

	pkgcrypto "github.com/roidmc/kex-utils/pkg/crypto"
)

func TestArgon2idHashAndVerify(t *testing.T) {
	hash, err := pkgcrypto.Argon2idHash("my-secret-password", pkgcrypto.DefaultArgon2idParams)
	if err != nil {
		t.Fatalf("Argon2idHash: %v", err)
	}

	ok, err := pkgcrypto.Argon2idVerify("my-secret-password", hash)
	if err != nil {
		t.Fatalf("Argon2idVerify: %v", err)
	}
	if !ok {
		t.Fatal("password should verify")
	}
}

func TestArgon2idWrongPassword(t *testing.T) {
	hash, _ := pkgcrypto.Argon2idHash("correct", pkgcrypto.DefaultArgon2idParams)
	ok, err := pkgcrypto.Argon2idVerify("wrong", hash)
	if err != nil {
		t.Fatalf("Verify: %v", err)
	}
	if ok {
		t.Fatal("wrong password should not verify")
	}
}

func TestArgon2idEmptyPassword(t *testing.T) {
	hash, _ := pkgcrypto.Argon2idHash("", pkgcrypto.DefaultArgon2idParams)
	ok, _ := pkgcrypto.Argon2idVerify("", hash)
	if !ok {
		t.Fatal("empty password should verify")
	}
}

func TestArgon2idDifferentSalts(t *testing.T) {
	h1, _ := pkgcrypto.Argon2idHash("password", pkgcrypto.DefaultArgon2idParams)
	h2, _ := pkgcrypto.Argon2idHash("password", pkgcrypto.DefaultArgon2idParams)
	if h1 == h2 {
		t.Fatal("hashes with different salts should differ")
	}
}

func TestArgon2idInvalidFormat(t *testing.T) {
	_, err := pkgcrypto.Argon2idVerify("pw", "not-a-valid-hash")
	if err == nil {
		t.Fatal("expected error for invalid format")
	}
}

func TestArgon2idWrongAlgorithm(t *testing.T) {
	_, err := pkgcrypto.Argon2idVerify("pw", "$argon2i$v=19$m=65536,t=3,p=2$salt$hash")
	if err == nil {
		t.Fatal("expected error for wrong algorithm")
	}
}

func TestArgon2idInvalidBase64(t *testing.T) {
	_, err := pkgcrypto.Argon2idVerify("pw", "$argon2id$v=19$m=65536,t=3,p=2$!!!$!!!")
	if err == nil {
		t.Fatal("expected error for invalid base64")
	}
}

func TestArgon2idCustomParams(t *testing.T) {
	p := pkgcrypto.Argon2idParams{
		Memory:      32 * 1024,
		Iterations:  1,
		Parallelism: 1,
		SaltLength:  8,
		KeyLength:   16,
	}
	hash, err := pkgcrypto.Argon2idHash("pw", p)
	if err != nil {
		t.Fatalf("Hash: %v", err)
	}
	ok, _ := pkgcrypto.Argon2idVerify("pw", hash)
	if !ok {
		t.Fatal("custom params should verify")
	}
}
