package random_test

import (
	"regexp"
	"testing"

	"github.com/roidmc/quotagate/internal/util/random"
)

var uuidV7Pattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-7[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)

func TestNewUUID(t *testing.T) {
	uuid, err := random.NewUUID()
	if err != nil {
		t.Fatalf("NewUUID failed: %v", err)
	}

	var zero [16]byte
	if uuid == zero {
		t.Error("UUID should not be all zeros")
	}

	if uuid[6]&0xF0 != 0x70 {
		t.Errorf("UUID version should be 7, got %x", uuid[6]>>4)
	}

	if uuid[8]&0xC0 != 0x80 {
		t.Errorf("UUID variant should be RFC 4122, got %x", uuid[8]>>6)
	}
}

func TestNewUUIDUniqueness(t *testing.T) {
	seen := make(map[[16]byte]bool)
	for i := 0; i < 1000; i++ {
		uuid, err := random.NewUUID()
		if err != nil {
			t.Fatalf("NewUUID failed: %v", err)
		}
		if seen[uuid] {
			t.Error("duplicate UUID generated")
		}
		seen[uuid] = true
	}
}

func TestNewUUIDTimeOrdered(t *testing.T) {
	prev, err := random.NewUUID()
	if err != nil {
		t.Fatalf("NewUUID failed: %v", err)
	}

	for i := 0; i < 100; i++ {
		curr, err := random.NewUUID()
		if err != nil {
			t.Fatalf("NewUUID failed: %v", err)
		}

		for j := 0; j < 6; j++ {
			if prev[j] < curr[j] {
				break
			}
			if prev[j] > curr[j] {
				t.Error("UUIDs should be time-ordered (non-decreasing timestamp)")
				break
			}
		}

		prev = curr
	}
}

func TestMustUUID(t *testing.T) {
	uuid := random.MustUUID()
	if uuid[6]&0xF0 != 0x70 {
		t.Errorf("UUID version should be 7, got %x", uuid[6]>>4)
	}
}

func TestNewUUIDString(t *testing.T) {
	s, err := random.NewUUIDString()
	if err != nil {
		t.Fatalf("NewUUIDString failed: %v", err)
	}

	if !uuidV7Pattern.MatchString(s) {
		t.Errorf("UUID string format invalid: %s", s)
	}
}

func TestMustUUIDString(t *testing.T) {
	s := random.MustUUIDString()
	if !uuidV7Pattern.MatchString(s) {
		t.Errorf("UUID string format invalid: %s", s)
	}
}

func TestFormatUUID(t *testing.T) {
	uuid := [16]byte{
		0x01, 0x23, 0x45, 0x67, 0x89, 0xab, 0x7c, 0xde,
		0x80, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde,
	}
	formatted := random.FormatUUID(uuid)
	expected := "01234567-89ab-7cde-8012-3456789abcde"
	if formatted != expected {
		t.Errorf("FormatUUID got %s, want %s", formatted, expected)
	}
}

func TestParseUUID(t *testing.T) {
	original := "01234567-89ab-7cde-8012-3456789abcde"
	uuid, err := random.ParseUUID(original)
	if err != nil {
		t.Fatalf("ParseUUID failed: %v", err)
	}

	result := random.FormatUUID(uuid)
	if result != original {
		t.Errorf("round-trip failed: got %s, want %s", result, original)
	}
}

func TestParseUUIDWithoutDashes(t *testing.T) {
	s := "0123456789ab7cde80123456789abcde"
	uuid, err := random.ParseUUID(s)
	if err != nil {
		t.Fatalf("ParseUUID failed: %v", err)
	}

	result := random.FormatUUID(uuid)
	expected := "01234567-89ab-7cde-8012-3456789abcde"
	if result != expected {
		t.Errorf("ParseUUID without dashes got %s, want %s", result, expected)
	}
}

func TestParseUUIDInvalid(t *testing.T) {
	_, err := random.ParseUUID("not-a-uuid")
	if err == nil {
		t.Error("expected error for invalid UUID")
	}

	_, err = random.ParseUUID("01234567-89ab-7cde-8012-3456789abcdeff")
	if err == nil {
		t.Error("expected error for too long UUID")
	}
}

func TestUUIDRoundTrip(t *testing.T) {
	for i := 0; i < 100; i++ {
		original, err := random.NewUUID()
		if err != nil {
			t.Fatalf("NewUUID failed: %v", err)
		}

		formatted := random.FormatUUID(original)
		parsed, err := random.ParseUUID(formatted)
		if err != nil {
			t.Fatalf("ParseUUID failed: %v", err)
		}

		if original != parsed {
			t.Errorf("round-trip failed: original=%x, parsed=%x", original, parsed)
		}
	}
}
