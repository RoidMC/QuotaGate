package kexswiftbus_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/roidmc/quotagate/pkg/kexswiftbus"
)

func TestRedisStore_ConcurrentPublish(t *testing.T) {
	store := newRedisTestStore(t)
	defer store.Close()

	ctx := context.Background()
	ch, err := store.Subscribe(ctx, "test.redis.concurrent")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	// Wait for subscription to be fully established before publishing.
	time.Sleep(200 * time.Millisecond)

	// Send a warm-up message and wait for it to confirm the channel is ready.
	store.Publish(ctx, kexswiftbus.NewMessage("test.redis.concurrent", -1))
	select {
	case <-ch:
	case <-time.After(500 * time.Millisecond):
		t.Fatal("warm-up message not received")
	}

	const goroutines = 50
	const messagesPerGoroutine = 20
	var published int64

	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			for j := 0; j < messagesPerGoroutine; j++ {
				msg := kexswiftbus.NewMessage("test.redis.concurrent", id*messagesPerGoroutine+j)
				if err := store.Publish(ctx, msg); err != nil {
					t.Errorf("Publish: %v", err)
					return
				}
				atomic.AddInt64(&published, 1)
			}
		}(i)
	}
	wg.Wait()

	received := 1 // warm-up already counted
	done := time.After(2 * time.Second)
loop:
	for {
		select {
		case <-ch:
			received++
			if received == goroutines*messagesPerGoroutine+1 {
				break loop
			}
		case <-done:
			break loop
		}
	}

	if received != int(published)+1 {
		t.Fatalf("expected %d messages, got %d", published+1, received)
	}
}

func TestRedisStore_ConcurrentSubscribeUnsubscribe(t *testing.T) {
	store := newRedisTestStore(t)
	defer store.Close()

	ctx := context.Background()

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func(id int) {
			defer wg.Done()
			topic := "test.redis.subunsub"
			ch, err := store.Subscribe(ctx, topic)
			if err != nil {
				t.Errorf("Subscribe: %v", err)
				return
			}
			time.Sleep(10 * time.Millisecond)
			if err := store.Unsubscribe(ctx, topic, ch); err != nil {
				t.Errorf("Unsubscribe: %v", err)
			}
		}(i)
	}

	wg.Wait()
}

func TestRedisStore_TopicIsolation(t *testing.T) {
	store := newRedisTestStore(t)
	defer store.Close()

	ctx := context.Background()

	ch1, err := store.Subscribe(ctx, "topic.a")
	if err != nil {
		t.Fatalf("Subscribe topic.a: %v", err)
	}
	ch2, err := store.Subscribe(ctx, "topic.b")
	if err != nil {
		t.Fatalf("Subscribe topic.b: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	store.Publish(ctx, kexswiftbus.NewMessage("topic.a", "a"))
	store.Publish(ctx, kexswiftbus.NewMessage("topic.b", "b"))

	var gotA, gotB bool
	timeout := time.After(500 * time.Millisecond)
	for !gotA || !gotB {
		select {
		case m := <-ch1:
			if m.Topic != "topic.a" {
				t.Fatalf("expected topic.a, got %s", m.Topic)
			}
			gotA = true
		case m := <-ch2:
			if m.Topic != "topic.b" {
				t.Fatalf("expected topic.b, got %s", m.Topic)
			}
			gotB = true
		case <-timeout:
			t.Fatalf("timeout waiting for messages, gotA=%v gotB=%v", gotA, gotB)
		}
	}
}

func TestRedisStore_SubscribeAfterClose(t *testing.T) {
	store := newRedisTestStore(t)

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	ctx := context.Background()
	_, err := store.Subscribe(ctx, "test.redis.closed")
	if err == nil {
		t.Fatal("expected error subscribing to closed store")
	}
}

func TestRedisStore_CloseIdempotent(t *testing.T) {
	store := newRedisTestStore(t)

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}
	if err := store.Close(); err != nil {
		t.Fatalf("Close second time: %v", err)
	}
}

func TestRedisStore_GoroutineCleanupOnClose(t *testing.T) {
	store := newRedisTestStore(t)

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		_, err := store.Subscribe(ctx, "test.redis.cleanup")
		if err != nil {
			t.Fatalf("Subscribe: %v", err)
		}
	}

	time.Sleep(100 * time.Millisecond)

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	// Close waits for all consume goroutines to exit.
	// If goroutines leaked, this test would hang.
}

func TestRedisStore_LargePayload(t *testing.T) {
	store := newRedisTestStore(t)
	defer store.Close()

	ctx := context.Background()
	ch, err := store.Subscribe(ctx, "test.redis.large")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	large := make([]byte, 1024*1024) // 1 MiB
	for i := range large {
		large[i] = byte(i % 256)
	}

	store.Publish(ctx, kexswiftbus.NewMessage("test.redis.large", string(large)))

	select {
	case m := <-ch:
		if m.Payload != string(large) {
			t.Fatal("large payload mismatch")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for large payload")
	}
}
