package kexswiftdb_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/roidmc/quotagate/pkg/kexswiftdb"
)

const (
	redisHost = "localhost"
	redisPort = 6379
)

func newRedisTestStore(t *testing.T) kexswiftdb.Store {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort),
		DB:   0,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis ping: %v", err)
	}

	store, err := kexswiftdb.NewRedisStore(rdb)
	if err != nil {
		t.Fatalf("NewRedisStore: %v", err)
	}
	return store
}

func cleanupNamespace(t *testing.T, s kexswiftdb.Store, prefix kexswiftdb.Prefix) {
	t.Helper()
	ctx := context.Background()
	keys, err := s.Keys(ctx, prefix, "")
	if err != nil {
		t.Logf("cleanup Keys: %v", err)
		return
	}
	for _, k := range keys {
		_ = s.Delete(ctx, prefix, k)
	}
}

func TestRedisStore_Ping(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()

	ctx := context.Background()
	if err := s.Ping(ctx); err != nil {
		t.Fatalf("Ping: %v", err)
	}
}

func TestRedisStore_SetGet(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

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

func TestRedisStore_GetNotFound(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()

	_, err := s.Get(ctx, "qrcode", "nonexistent-key-test")
	if err != kexswiftdb.ErrKeyNotFound {
		t.Fatalf("Get: got err %v, want ErrKeyNotFound", err)
	}
}

func TestRedisStore_Delete(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	s.Set(ctx, "qrcode", "key1", []byte("value1"), 0)
	s.Delete(ctx, "qrcode", "key1")

	_, err := s.Get(ctx, "qrcode", "key1")
	if err != kexswiftdb.ErrKeyNotFound {
		t.Fatalf("Get after Delete: got err %v, want ErrKeyNotFound", err)
	}
}

func TestRedisStore_Exists(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

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

func TestRedisStore_TTL(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	err := s.Set(ctx, "qrcode", "ttl-key", []byte("value1"), 3*time.Second)
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	val, err := s.Get(ctx, "qrcode", "ttl-key")
	if err != nil {
		t.Fatalf("Get before expiry: %v", err)
	}
	if string(val) != "value1" {
		t.Fatalf("Get: got %q, want %q", string(val), "value1")
	}

	time.Sleep(4 * time.Second)

	_, err = s.Get(ctx, "qrcode", "ttl-key")
	if err != kexswiftdb.ErrKeyNotFound {
		t.Fatalf("Get after expiry: got err %v, want ErrKeyNotFound", err)
	}
}

func TestRedisStore_Increment(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "ratelimit")

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

func TestRedisStore_IncrementNoTTL(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "ratelimit")

	val, err := s.Increment(ctx, "ratelimit", "counter-no-ttl", 0)
	if err != nil {
		t.Fatalf("Increment (no ttl): %v", err)
	}
	if val != 1 {
		t.Fatalf("Increment: got %d, want 1", val)
	}

	data, err := s.Get(ctx, "ratelimit", "counter-no-ttl")
	if err != nil {
		t.Fatalf("Get after Increment (no ttl): %v", err)
	}
	if string(data) != "1" {
		t.Fatalf("Get: got %q, want %q", string(data), "1")
	}
}

func TestRedisStore_SetNX(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	created, err := s.SetNX(ctx, "qrcode", "setnx-key", []byte("value1"), time.Minute)
	if err != nil {
		t.Fatalf("SetNX (first): %v", err)
	}
	if !created {
		t.Fatal("SetNX (first): got false, want true")
	}

	created, err = s.SetNX(ctx, "qrcode", "setnx-key", []byte("value2"), time.Minute)
	if err != nil {
		t.Fatalf("SetNX (second): %v", err)
	}
	if created {
		t.Fatal("SetNX (second): got true, want false")
	}

	val, _ := s.Get(ctx, "qrcode", "setnx-key")
	if string(val) != "value1" {
		t.Fatalf("Get: got %q, want %q", string(val), "value1")
	}
}

func TestRedisStore_CompareAndSwap(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	s.Set(ctx, "qrcode", "cas-key", []byte("old"), time.Minute)

	swapped, err := s.CompareAndSwap(ctx, "qrcode", "cas-key", []byte("old"), []byte("new"), time.Minute)
	if err != nil {
		t.Fatalf("CompareAndSwap: %v", err)
	}
	if !swapped {
		t.Fatal("CompareAndSwap: got false, want true")
	}

	val, _ := s.Get(ctx, "qrcode", "cas-key")
	if string(val) != "new" {
		t.Fatalf("Get after CAS: got %q, want %q", string(val), "new")
	}
}

func TestRedisStore_CompareAndSwap_WrongOldValue(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	s.Set(ctx, "qrcode", "cas-key2", []byte("old"), time.Minute)

	swapped, err := s.CompareAndSwap(ctx, "qrcode", "cas-key2", []byte("wrong"), []byte("new"), time.Minute)
	if err != nil {
		t.Fatalf("CompareAndSwap: %v", err)
	}
	if swapped {
		t.Fatal("CompareAndSwap: got true, want false")
	}

	val, _ := s.Get(ctx, "qrcode", "cas-key2")
	if string(val) != "old" {
		t.Fatalf("Get after failed CAS: got %q, want %q", string(val), "old")
	}
}

func TestRedisStore_CompareAndSwap_NilOldValue(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	swapped, err := s.CompareAndSwap(ctx, "qrcode", "cas-new-key", nil, []byte("created"), time.Minute)
	if err != nil {
		t.Fatalf("CompareAndSwap (nil old): %v", err)
	}
	if !swapped {
		t.Fatal("CompareAndSwap (nil old): got false, want true")
	}

	val, _ := s.Get(ctx, "qrcode", "cas-new-key")
	if string(val) != "created" {
		t.Fatalf("Get: got %q, want %q", string(val), "created")
	}
}

func TestRedisStore_CompareAndSwap_TTLPreserved(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	s.Set(ctx, "qrcode", "cas-ttl-key", []byte("v1"), 5*time.Second)

	swapped, err := s.CompareAndSwap(ctx, "qrcode", "cas-ttl-key", []byte("v1"), []byte("v2"), 5*time.Second)
	if err != nil {
		t.Fatalf("CompareAndSwap: %v", err)
	}
	if !swapped {
		t.Fatal("CompareAndSwap: got false, want true")
	}

	val, _ := s.Get(ctx, "qrcode", "cas-ttl-key")
	if string(val) != "v2" {
		t.Fatalf("Get after CAS: got %q, want %q", string(val), "v2")
	}

	time.Sleep(6 * time.Second)

	_, err = s.Get(ctx, "qrcode", "cas-ttl-key")
	if err != kexswiftdb.ErrKeyNotFound {
		t.Fatalf("Get after TTL expiry: got err %v, want ErrKeyNotFound", err)
	}
}

func TestRedisStore_NamespaceIsolation(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")
	defer cleanupNamespace(t, s, "token:blacklist")

	s.Set(ctx, "qrcode", "same-key", []byte("qr-value"), time.Minute)
	s.Set(ctx, "token:blacklist", "same-key", []byte("token-value"), time.Minute)

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

func TestRedisStore_Keys(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")
	defer cleanupNamespace(t, s, "token:blacklist")

	s.Set(ctx, "qrcode", "abc-1", []byte("v1"), time.Minute)
	s.Set(ctx, "qrcode", "abc-2", []byte("v2"), time.Minute)
	s.Set(ctx, "qrcode", "xyz-1", []byte("v3"), time.Minute)
	s.Set(ctx, "token:blacklist", "abc-1", []byte("v4"), time.Minute)

	keys, err := s.Keys(ctx, "qrcode", "abc")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(keys) != 2 {
		t.Fatalf("Keys (prefix abc): got %d keys, want 2", len(keys))
	}

	allKeys, err := s.Keys(ctx, "qrcode", "")
	if err != nil {
		t.Fatalf("Keys: %v", err)
	}
	if len(allKeys) != 3 {
		t.Fatalf("Keys (all): got %d keys, want 3", len(allKeys))
	}
}

func TestRedisStore_Stats(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")
	defer cleanupNamespace(t, s, "ratelimit")

	s.Set(ctx, "qrcode", "key1", []byte("v1"), time.Minute)
	s.Set(ctx, "qrcode", "key2", []byte("v2"), time.Minute)
	s.Set(ctx, "ratelimit", "counter1", []byte("1"), time.Minute)

	stats := s.Stats(ctx)

	nsMap := make(map[string]int)
	for _, st := range stats {
		nsMap[st.Namespace] = st.KeyCount
	}
	if nsMap["qrcode"] < 2 {
		t.Fatalf("Stats qrcode: got %d, want >= 2", nsMap["qrcode"])
	}
	if nsMap["ratelimit"] < 1 {
		t.Fatalf("Stats ratelimit: got %d, want >= 1", nsMap["ratelimit"])
	}
}

func TestRedisStore_Overwrite(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	s.Set(ctx, "qrcode", "key1", []byte("value1"), 0)
	s.Set(ctx, "qrcode", "key1", []byte("value2"), 0)

	val, _ := s.Get(ctx, "qrcode", "key1")
	if string(val) != "value2" {
		t.Fatalf("Get: got %q, want %q", string(val), "value2")
	}
}

func TestRedisStore_MutateJSON(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	type entry struct {
		Status string `json:"status"`
		Count  int    `json:"count"`
	}

	err := kexswiftdb.SetJSON(ctx, s, "qrcode", "mutate-key", entry{Status: "pending", Count: 0}, time.Minute)
	if err != nil {
		t.Fatalf("SetJSON: %v", err)
	}

	result, err := kexswiftdb.MutateJSON[entry](ctx, s, "qrcode", "mutate-key", func(current *entry) (entry, bool, time.Duration, error) {
		if current == nil {
			return entry{}, false, 0, fmt.Errorf("unexpected nil")
		}
		current.Count++
		current.Status = "updated"
		return *current, true, time.Minute, nil
	})
	if err != nil {
		t.Fatalf("MutateJSON: %v", err)
	}
	if result.Count != 1 {
		t.Fatalf("MutateJSON Count: got %d, want 1", result.Count)
	}
	if result.Status != "updated" {
		t.Fatalf("MutateJSON Status: got %q, want %q", result.Status, "updated")
	}
}

func TestRedisStore_MutateJSON_CreateNew(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "qrcode")

	type entry struct {
		Value string `json:"value"`
	}

	result, err := kexswiftdb.MutateJSON[entry](ctx, s, "qrcode", "new-mutate-key", func(current *entry) (entry, bool, time.Duration, error) {
		if current != nil {
			return entry{}, false, 0, fmt.Errorf("expected nil current")
		}
		return entry{Value: "created"}, true, time.Minute, nil
	})
	if err != nil {
		t.Fatalf("MutateJSON (create): %v", err)
	}
	if result.Value != "created" {
		t.Fatalf("MutateJSON Value: got %q, want %q", result.Value, "created")
	}
}

func TestRedisStore_Close(t *testing.T) {
	s := newRedisTestStore(t)

	err := s.Close()
	if err != nil {
		t.Fatalf("Close: %v", err)
	}
}

func TestRedisStore_NewRedisStore_NilClient(t *testing.T) {
	_, err := kexswiftdb.NewRedisStore(nil)
	if err != kexswiftdb.ErrNilClient {
		t.Fatalf("NewRedisStore(nil): got err %v, want ErrNilClient", err)
	}
}

func TestRedisStore_DeleteByPrefix(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "session")
	defer cleanupNamespace(t, s, "qrcode")

	s.Set(ctx, "session", "user-1:device-a", []byte("s1"), time.Minute)
	s.Set(ctx, "session", "user-1:device-b", []byte("s2"), time.Minute)
	s.Set(ctx, "session", "user-1:device-c", []byte("s3"), time.Minute)
	s.Set(ctx, "session", "user-2:device-a", []byte("s4"), time.Minute)
	s.Set(ctx, "qrcode", "user-1:code", []byte("qr"), time.Minute)

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

func TestRedisStore_DeleteByPrefix_EntireNamespace(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "ratelimit")
	defer cleanupNamespace(t, s, "qrcode")

	s.Set(ctx, "ratelimit", "a", []byte("1"), time.Minute)
	s.Set(ctx, "ratelimit", "b", []byte("2"), time.Minute)
	s.Set(ctx, "qrcode", "c", []byte("3"), time.Minute)

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

func TestRedisStore_DeleteByPrefix_NoMatch(t *testing.T) {
	s := newRedisTestStore(t)
	defer s.Close()
	ctx := context.Background()
	defer cleanupNamespace(t, s, "session")

	s.Set(ctx, "session", "user-1:device-a", []byte("s1"), time.Minute)

	count, err := s.DeleteByPrefix(ctx, "session", "user-99:")
	if err != nil {
		t.Fatalf("DeleteByPrefix: %v", err)
	}
	if count != 0 {
		t.Fatalf("DeleteByPrefix: got %d, want 0", count)
	}
}
