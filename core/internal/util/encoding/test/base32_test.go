package encoding_test

import (
	"bytes"
	"testing"

	"github.com/roidmc/quotagate/internal/util/encoding"
)

func TestBase32EncodeDecode(t *testing.T) {
	original := []byte("hello world")
	encoded := encoding.Base32Encode(original)
	decoded, err := encoding.Base32Decode(encoded)
	if err != nil {
		t.Fatalf("Base32Decode failed: %v", err)
	}
	if !bytes.Equal(original, decoded) {
		t.Errorf("expected %q, got %q", original, decoded)
	}
}

func TestBase32DecodeWithPadding(t *testing.T) {
	encoded := encoding.Base32Encode([]byte("test"))
	decoded, err := encoding.Base32Decode(encoded)
	if err != nil {
		t.Fatalf("Base32Decode with padding failed: %v", err)
	}
	if string(decoded) != "test" {
		t.Errorf("expected 'test', got %q", decoded)
	}
}

func TestBase32DecodeLowercase(t *testing.T) {
	encoded := encoding.Base32Encode([]byte("hello"))
	decoded, err := encoding.Base32Decode(encoded)
	if err != nil {
		t.Fatalf("Base32Decode lowercase failed: %v", err)
	}
	if string(decoded) != "hello" {
		t.Errorf("expected 'hello', got %q", decoded)
	}
}

func TestBase32RoundTrip(t *testing.T) {
	tests := []string{
		"",
		"a",
		"ab",
		"abc",
		"abcd",
		"abcde",
		"abcdef",
		"abcdefg",
		"abcdefgh",
		"hello world",
		"The quick brown fox jumps over the lazy dog",
	}

	for _, tc := range tests {
		encoded := encoding.Base32Encode([]byte(tc))
		decoded, err := encoding.Base32Decode(encoded)
		if err != nil {
			t.Fatalf("Base32Decode failed for %q: %v", tc, err)
		}
		if string(decoded) != tc {
			t.Errorf("round-trip failed for %q: got %q", tc, decoded)
		}
	}
}

func TestBase32HexEncodeDecode(t *testing.T) {
	original := []byte("hello world")
	encoded := encoding.Base32HexEncode(original)
	decoded, err := encoding.Base32HexDecode(encoded)
	if err != nil {
		t.Fatalf("Base32HexDecode failed: %v", err)
	}
	if !bytes.Equal(original, decoded) {
		t.Errorf("expected %q, got %q", original, decoded)
	}
}

func TestBase32HexRoundTrip(t *testing.T) {
	tests := []string{
		"",
		"a",
		"hello world",
		"The quick brown fox jumps over the lazy dog",
	}

	for _, tc := range tests {
		encoded := encoding.Base32HexEncode([]byte(tc))
		decoded, err := encoding.Base32HexDecode(encoded)
		if err != nil {
			t.Fatalf("Base32HexDecode failed for %q: %v", tc, err)
		}
		if string(decoded) != tc {
			t.Errorf("hex round-trip failed for %q: got %q", tc, decoded)
		}
	}
}

func TestBase32DecodeInvalidInput(t *testing.T) {
	_, err := encoding.Base32Decode("!@#$%^&*")
	if err == nil {
		t.Error("expected error for invalid base32 input")
	}
}

func TestBase32HexDecodeInvalidInput(t *testing.T) {
	_, err := encoding.Base32HexDecode("!@#$%^&*")
	if err == nil {
		t.Error("expected error for invalid base32hex input")
	}
}

func TestBase32EncodeEmpty(t *testing.T) {
	encoded := encoding.Base32Encode([]byte{})
	if encoded != "" {
		t.Errorf("expected empty string, got %q", encoded)
	}
	decoded, err := encoding.Base32Decode(encoded)
	if err != nil {
		t.Fatalf("Base32Decode empty failed: %v", err)
	}
	if len(decoded) != 0 {
		t.Errorf("expected empty decoded, got %q", decoded)
	}
}

func TestBase32HexEncodeEmpty(t *testing.T) {
	encoded := encoding.Base32HexEncode([]byte{})
	if encoded != "" {
		t.Errorf("expected empty string, got %q", encoded)
	}
	decoded, err := encoding.Base32HexDecode(encoded)
	if err != nil {
		t.Fatalf("Base32HexDecode empty failed: %v", err)
	}
	if len(decoded) != 0 {
		t.Errorf("expected empty decoded, got %q", decoded)
	}
}

func TestBase32DecodeWithExplicitPadding(t *testing.T) {
	encoded := encoding.Base32Encode([]byte("a"))
	decoded, err := encoding.Base32Decode(encoded)
	if err != nil {
		t.Fatalf("Base32Decode explicit padding failed: %v", err)
	}
	if string(decoded) != "a" {
		t.Errorf("expected 'a', got %q", decoded)
	}
}

func TestBase32DecodeMixedCase(t *testing.T) {
	encoded := encoding.Base32Encode([]byte("test"))
	decoded, err := encoding.Base32Decode(encoded)
	if err != nil {
		t.Fatalf("Base32Decode mixed case failed: %v", err)
	}
	if string(decoded) != "test" {
		t.Errorf("expected 'test', got %q", decoded)
	}
}
