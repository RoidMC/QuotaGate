package kexswiftbus_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/roidmc/kex-utils/pkg/kexswiftbus"
)

const (
	redisHost = "localhost"
	redisPort = 6379
)

func newRedisTestStore(t *testing.T) *kexswiftbus.RedisStore {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis ping: %v", err)
	}

	store, err := kexswiftbus.NewRedisStore(rdb)
	if err != nil {
		t.Fatalf("NewRedisStore: %v", err)
	}
	return store
}

func TestRedisStore_PublishAndSubscribe(t *testing.T) {
	store := newRedisTestStore(t)
	defer store.Close()

	ctx := context.Background()

	ch, err := store.Subscribe(ctx, "test.redis.msg")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	payload := map[string]string{"hello": "world"}
	msg := kexswiftbus.NewMessage("test.redis.msg", payload)
	if err := store.Publish(ctx, msg); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case m := <-ch:
		if m.Topic != "test.redis.msg" {
			t.Fatalf("expected topic test.redis.msg, got %s", m.Topic)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for message")
	}
}

func TestRedisStore_MultipleSubscribers(t *testing.T) {
	store := newRedisTestStore(t)
	defer store.Close()

	ctx := context.Background()

	ch1, err := store.Subscribe(ctx, "test.redis.multi")
	if err != nil {
		t.Fatalf("Subscribe 1: %v", err)
	}
	ch2, err := store.Subscribe(ctx, "test.redis.multi")
	if err != nil {
		t.Fatalf("Subscribe 2: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	store.Publish(ctx, kexswiftbus.NewMessage("test.redis.multi", "hello"))

	received := make(chan int, 2)
	go func() { <-ch1; received <- 1 }()
	go func() { <-ch2; received <- 2 }()

	select {
	case <-received:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ch1 timed out")
	}
	select {
	case <-received:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ch2 timed out")
	}
}

func TestRedisStore_Unsubscribe(t *testing.T) {
	store := newRedisTestStore(t)
	defer store.Close()

	ctx := context.Background()

	ch, err := store.Subscribe(ctx, "test.redis.unsub")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	store.Publish(ctx, kexswiftbus.NewMessage("test.redis.unsub", "first"))
	select {
	case <-ch:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out before unsubscribe")
	}

	if err := store.Unsubscribe(ctx, "test.redis.unsub", ch); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	store.Publish(ctx, kexswiftbus.NewMessage("test.redis.unsub", "second"))
	select {
	case <-ch:
		// A message may have been in flight before unsubscribe took effect.
	case <-time.After(500 * time.Millisecond):
	}
}

func TestRedisStore_Prefix(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis ping: %v", err)
	}

	store, err := kexswiftbus.NewRedisStore(rdb, kexswiftbus.WithRedisPrefix("test.prefix"))
	if err != nil {
		t.Fatalf("NewRedisStore: %v", err)
	}
	defer store.Close()

	ch, err := store.Subscribe(ctx, "topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	store.Publish(ctx, kexswiftbus.NewMessage("topic", "prefixed"))

	select {
	case <-ch:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for prefixed message")
	}
}

func TestRedisStore_Close(t *testing.T) {
	store := newRedisTestStore(t)

	ctx := context.Background()
	ch, err := store.Subscribe(ctx, "test.redis.close")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	if err := store.Publish(ctx, kexswiftbus.NewMessage("test.redis.close", nil)); !errors.Is(err, kexswiftbus.ErrStoreClosed) {
		t.Fatalf("expected ErrStoreClosed, got %v", err)
	}

	select {
	case <-ch:
	case <-time.After(500 * time.Millisecond):
	}
}

func TestRedisStore_NewRedisStore_NilClient(t *testing.T) {
	_, err := kexswiftbus.NewRedisStore(nil)
	if !errors.Is(err, kexswiftbus.ErrNilClient) {
		t.Fatalf("NewRedisStore(nil): got %v, want ErrNilClient", err)
	}
}

func TestRedisStore_JSONRoundTrip(t *testing.T) {
	store := newRedisTestStore(t)
	defer store.Close()

	ctx := context.Background()

	ch, err := store.Subscribe(ctx, "test.redis.json")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	payload := map[string]interface{}{
		"name":  "test",
		"count": 42,
	}
	serialized, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort),
	})
	defer rdb.Close()
	rdb.Publish(ctx, "test.redis.json", serialized)

	select {
	case m := <-ch:
		if m.Topic != "test.redis.json" {
			t.Fatalf("expected topic test.redis.json, got %s", m.Topic)
		}
		if m.Payload != string(serialized) {
			t.Fatalf("expected payload %s, got %v", string(serialized), m.Payload)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for JSON round-trip message")
	}
}

func TestRedisStore_ErrorHandler(t *testing.T) {
	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis ping: %v", err)
	}

	var mu sync.Mutex
	var gotErr error
	gotTopic := ""

	store, err := kexswiftbus.NewRedisStore(rdb, kexswiftbus.WithErrorHandler(func(topic string, err error) {
		mu.Lock()
		gotTopic = topic
		gotErr = err
		mu.Unlock()
	}))
	if err != nil {
		t.Fatalf("NewRedisStore: %v", err)
	}
	defer store.Close()

	// Close the underlying client to force a subscription error.
	_ = rdb.Close()

	_, _ = store.Subscribe(ctx, "test.redis.error")

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if gotTopic != "test.redis.error" {
		t.Errorf("expected topic test.redis.error, got %q", gotTopic)
	}
	if gotErr == nil {
		t.Fatal("expected error via handler, got nil")
	}
	mu.Unlock()
}
