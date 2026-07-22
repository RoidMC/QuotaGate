package kexswiftdb_test

import (
	"context"
	"testing"
	"time"

	"github.com/roidmc/kex-utils/pkg/kexswiftdb"
)

func newTestStore(t *testing.T) kexswiftdb.Store {
	t.Helper()
	s, err := kexswiftdb.NewInMemoryBadgerStore()
	if err != nil {
		t.Fatalf("NewInMemoryBadgerStore: %v", err)
	}
	return s
}

func TestBadgerStore_SetGet(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	err := s.Set(ctx, "qrcode", "key1", []byte("value1"), 0)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, err := s.Get(ctx, "qrcode", "key1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if string(val) != "value1" {
		t.Fatalf("Get: got %q, want %q", string(val), "value1")
	}
}

func TestBadgerStore_GetNotFound(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	_, err := s.Get(ctx, "qrcode", "nonexistent")
	if err != kexswiftdb.ErrKeyNotFound {
		t.Fatalf("Get: got err %v, want ErrKeyNotFound", err)
	}
}

func TestBadgerStore_Delete(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	s.Set(ctx, "qrcode", "key1", []byte("value1"), 0)
	s.Delete(ctx, "qrcode", "key1")

	_, err := s.Get(ctx, "qrcode", "key1")
	if err != kexswiftdb.ErrKeyNotFound {
		t.Fatalf("Get after Delete: got err %v, want ErrKeyNotFound", err)
	}
}

func TestBadgerStore_Exists(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	exists, err := s.Exists(ctx, "qrcode", "key1")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if exists {
		t.Fatal("Exists: got true, want false")
	}

	s.Set(ctx, "qrcode", "key1", []byte("value1"), 0)
	exists, err = s.Exists(ctx, "qrcode", "key1")
	if err != nil {
		t.Fatalf("Exists: %v", err)
	}
	if !exists {
		t.Fatal("Exists: got false, want true")
	}
}

func TestBadgerStore_TTL(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	err := s.Set(ctx, "qrcode", "key1", []byte("value1"), 2*time.Second)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, err := s.Get(ctx, "qrcode", "key1")
	if err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}
	if string(val) != "value1" {
		t.Fatalf("Get: got %q, want %q", string(val), "value1")
	}

	time.Sleep(3 * time.Second)

	_, err = s.Get(ctx, "qrcode", "key1")
	if err != kexswiftdb.ErrKeyNotFound {
		t.Fatalf("Get after expiry: got err %v, want ErrKeyNotFound", err)
	}

	exists, _ := s.Exists(ctx, "qrcode", "key1")
	if exists {
		t.Fatal("Exists after expiry: got true, want false")
	}
}

func TestBadgerStore_Increment(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	val, err := s.Increment(ctx, "ratelimit", "counter1", time.Minute)
	if err != nil {
		t.Fatalf("Increment: %v", err)
	}
	if val != 1 {
		t.Fatalf("Increment: got %d, want 1", val)
	}

	val, err = s.Increment(ctx, "ratelimit", "counter1", time.Minute)
	if err != nil {
		t.Fatalf("Increment: %v", err)
	}
	if val != 2 {
		t.Fatalf("Increment: got %d, want 2", val)
	}

	val, err = s.Increment(ctx, "ratelimit", "counter1", time.Minute)
	if err != nil {
		t.Fatalf("Increment: %v", err)
	}
	if val != 3 {
		t.Fatalf("Increment: got %d, want 3", val)
	}
}

func TestBadgerStore_IncrementResetAfterExpiry(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	val, _ := s.Increment(ctx, "ratelimit", "counter1", 2*time.Second)
	if val != 1 {
		t.Fatalf("Increment: got %d, want 1", val)
	}

	time.Sleep(3 * time.Second)

	val, _ = s.Increment(ctx, "ratelimit", "counter1", time.Minute)
	if val != 1 {
		t.Fatalf("Increment after expiry: got %d, want 1", val)
	}
}

func TestBadgerStore_NamespaceIsolation(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	s.Set(ctx, "qrcode", "same-key", []byte("qr-value"), 0)
	s.Set(ctx, "token:blacklist", "same-key", []byte("token-value"), 0)

	val1, _ := s.Get(ctx, "qrcode", "same-key")
	val2, _ := s.Get(ctx, "token:blacklist", "same-key")

	if string(val1) != "qr-value" {
		t.Fatalf("QRCode ns: got %q, want %q", string(val1), "qr-value")
	}
	if string(val2) != "token-value" {
		t.Fatalf("TokenBlacklist ns: got %q, want %q", string(val2), "token-value")
	}

	s.Delete(ctx, "qrcode", "same-key")

	_, err := s.Get(ctx, "qrcode", "same-key")
	if err != kexswiftdb.ErrKeyNotFound {
		t.Fatal("QRCode key should be deleted")
	}

	val2, err = s.Get(ctx, "token:blacklist", "same-key")
	if err != nil {
		t.Fatal("TokenBlacklist key should still exist")
	}
	if string(val2) != "token-value" {
		t.Fatalf("TokenBlacklist ns: got %q, want %q", string(val2), "token-value")
	}
}

func TestBadgerStore_Keys(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	s.Set(ctx, "qrcode", "abc-1", []byte("v1"), 0)
	s.Set(ctx, "qrcode", "abc-2", []byte("v2"), 0)
	s.Set(ctx, "qrcode", "xyz-1", []byte("v3"), 0)
	s.Set(ctx, "token:blacklist", "abc-1", []byte("v4"), 0)

	keys, err := s.Keys(ctx, "qrcode", "abc")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("Keys: got %d keys, want 2", len(keys))
	}

	allKeys, err := s.Keys(ctx, "qrcode", "")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(allKeys) != 3 {
		t.Fatalf("Keys: got %d keys, want 3", len(allKeys))
	}
}

func TestBadgerStore_Stats(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	stats := s.Stats(ctx)
	if len(stats) != 0 {
		t.Fatalf("Stats on empty store: got %d entries, want 0", len(stats))
	}

	s.Set(ctx, "qrcode", "key1", []byte("v1"), 0)
	s.Set(ctx, "qrcode", "key2", []byte("v2"), 0)
	s.Set(ctx, "ratelimit", "counter1", []byte("1"), time.Minute)

	stats = s.Stats(ctx)

	nsMap := make(map[string]int)
	for _, st := range stats {
		nsMap[st.Namespace] = st.KeyCount
	}
	if nsMap["qrcode"] != 2 {
		t.Fatalf("Stats qrcode: got %d, want 2", nsMap["qrcode"])
	}
	if nsMap["ratelimit"] != 1 {
		t.Fatalf("Stats ratelimit: got %d, want 1", nsMap["ratelimit"])
	}
}

func TestBadgerStore_Close(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	s.Set(ctx, "qrcode", "key1", []byte("value1"), 0)
	s.Close()

	err := s.Set(ctx, "qrcode", "key2", []byte("value2"), 0)
	if err != kexswiftdb.ErrStoreClosed {
		t.Fatalf("Set after close: got err %v, want ErrStoreClosed", err)
	}

	_, err = s.Get(ctx, "qrcode", "key1")
	if err != kexswiftdb.ErrStoreClosed {
		t.Fatalf("Get after close: got err %v, want ErrStoreClosed", err)
	}
}

func TestBadgerStore_Overwrite(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	s.Set(ctx, "qrcode", "key1", []byte("value1"), 0)
	s.Set(ctx, "qrcode", "key1", []byte("value2"), 0)

	val, _ := s.Get(ctx, "qrcode", "key1")
	if string(val) != "value2" {
		t.Fatalf("Get: got %q, want %q", string(val), "value2")
	}
}

func TestParseFormatInt64(t *testing.T) {
	original := int64(42)
	data := kexswiftdb.FormatInt64(original)
	parsed, err := kexswiftdb.ParseInt64(data)
	if err != nil {
		t.Fatalf("ParseInt64: %v", err)
	}
	if parsed != original {
		t.Fatalf("ParseInt64: got %d, want %d", parsed, original)
	}
}

func TestBadgerStore_DeleteByPrefix(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	s.Set(ctx, "session", "user-1:device-a", []byte("s1"), 0)
	s.Set(ctx, "session", "user-1:device-b", []byte("s2"), 0)
	s.Set(ctx, "session", "user-1:device-c", []byte("s3"), 0)
	s.Set(ctx, "session", "user-2:device-a", []byte("s4"), 0)
	s.Set(ctx, "qrcode", "user-1:code", []byte("qr"), 0)

	count, err := s.DeleteByPrefix(ctx, "session", "user-1:")
	if err != nil {
		t.Fatalf("DeleteByPrefix: %v", err)
	}
	if count != 3 {
		t.Fatalf("DeleteByPrefix: got %d, want 3", count)
	}

	keys, _ := s.Keys(ctx, "session", "")
	if len(keys) != 1 {
		t.Fatalf("Keys after DeleteByPrefix: got %d, want 1", len(keys))
	}

	exists, _ := s.Exists(ctx, "session", "user-2:device-a")
	if !exists {
		t.Fatal("user-2 session should still exist")
	}

	exists, _ = s.Exists(ctx, "qrcode", "user-1:code")
	if !exists {
		t.Fatal("qrcode key should not be affected")
	}
}

func TestBadgerStore_DeleteByPrefix_EntireNamespace(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	s.Set(ctx, "ratelimit", "a", []byte("1"), 0)
	s.Set(ctx, "ratelimit", "b", []byte("2"), 0)
	s.Set(ctx, "qrcode", "c", []byte("3"), 0)

	count, err := s.DeleteByPrefix(ctx, "ratelimit", "")
	if err != nil {
		t.Fatalf("DeleteByPrefix: %v", err)
	}
	if count != 2 {
		t.Fatalf("DeleteByPrefix: got %d, want 2", count)
	}

	keys, _ := s.Keys(ctx, "ratelimit", "")
	if len(keys) != 0 {
		t.Fatalf("Keys after DeleteByPrefix: got %d, want 0", len(keys))
	}

	exists, _ := s.Exists(ctx, "qrcode", "c")
	if !exists {
		t.Fatal("qrcode key should not be affected")
	}
}

func TestBadgerStore_DeleteByPrefix_NoMatch(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	s.Set(ctx, "session", "user-1:device-a", []byte("s1"), 0)

	count, err := s.DeleteByPrefix(ctx, "session", "user-99:")
	if err != nil {
		t.Fatalf("DeleteByPrefix: %v", err)
	}
	if count != 0 {
		t.Fatalf("DeleteByPrefix: got %d, want 0", count)
	}
}

func TestBadgerStore_CompareAndDelete(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	// Not present -> false, no error.
	deleted, err := s.CompareAndDelete(ctx, "qrcode", "missing", []byte("x"))
	if err != nil {
		t.Fatalf("CompareAndDelete on missing: %v", err)
	}
	if deleted {
		t.Fatal("CompareAndDelete on missing: got true, want false")
	}

	if err := s.Set(ctx, "qrcode", "k", []byte("v1"), 0); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Wrong expected -> false, key untouched.
	deleted, err = s.CompareAndDelete(ctx, "qrcode", "k", []byte("wrong"))
	if err != nil {
		t.Fatalf("CompareAndDelete wrong expected: %v", err)
	}
	if deleted {
		t.Fatal("CompareAndDelete wrong expected: got true, want false")
	}
	val, err := s.Get(ctx, "qrcode", "k")
	if err != nil || string(val) != "v1" {
		t.Fatalf("key should survive wrong-expected delete: val=%q err=%v", string(val), err)
	}

	// Correct expected -> true, key gone.
	deleted, err = s.CompareAndDelete(ctx, "qrcode", "k", []byte("v1"))
	if err != nil {
		t.Fatalf("CompareAndDelete correct expected: %v", err)
	}
	if !deleted {
		t.Fatal("CompareAndDelete correct expected: got false, want true")
	}
	if _, err := s.Get(ctx, "qrcode", "k"); err != kexswiftdb.ErrKeyNotFound {
		t.Fatalf("key should be gone after correct delete: err=%v", err)
	}
}

func TestBadgerStore_ConsumeJSON(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()
	ctx := context.Background()

	type entry struct {
		ID string `json:"id"`
	}
	mustSet := func(id string) {
		if err := kexswiftdb.SetJSON(ctx, s, "qrcode", id, entry{ID: id}, 0); err != nil {
			t.Fatalf("SetJSON: %v", err)
		}
	}

	// Absent -> (nil, nil).
	got, err := kexswiftdb.ConsumeJSON[entry](ctx, s, "qrcode", "absent", func(c *entry) (bool, error) {
		return true, nil
	})
	if err != nil || got != nil {
		t.Fatalf("ConsumeJSON absent: got=%v err=%v, want nil,nil", got, err)
	}

	mustSet("a")

	// First consume returns the value and removes the key.
	got, err = kexswiftdb.ConsumeJSON[entry](ctx, s, "qrcode", "a", func(c *entry) (bool, error) {
		return true, nil
	})
	if err != nil || got == nil || got.ID != "a" {
		t.Fatalf("ConsumeJSON first: got=%v err=%v", got, err)
	}
	if _, err := s.Get(ctx, "qrcode", "a"); err != kexswiftdb.ErrKeyNotFound {
		t.Fatalf("key should be gone after consume: err=%v", err)
	}

	// Second consume misses.
	got, err = kexswiftdb.ConsumeJSON[entry](ctx, s, "qrcode", "a", func(c *entry) (bool, error) {
		return true, nil
	})
	if err != nil || got != nil {
		t.Fatalf("ConsumeJSON second: got=%v err=%v, want nil,nil", got, err)
	}

	// Declined -> key survives.
	mustSet("b")
	got, err = kexswiftdb.ConsumeJSON[entry](ctx, s, "qrcode", "b", func(c *entry) (bool, error) {
		return false, nil
	})
	if err != nil || got != nil {
		t.Fatalf("ConsumeJSON declined: got=%v err=%v, want nil,nil", got, err)
	}
	if _, err := s.Get(ctx, "qrcode", "b"); err != nil {
		t.Fatalf("declined key should survive: err=%v", err)
	}
}
