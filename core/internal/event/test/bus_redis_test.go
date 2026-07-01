package event_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/types"
	"github.com/roidmc/quotagate/pkg/kexswiftbus"
)

const (
	redisHost = "localhost"
	redisPort = 6379
)

func newRedisTestClient(t *testing.T) *redis.Client {
	t.Helper()

	rdb := redis.NewClient(&redis.Options{
		Addr: fmt.Sprintf("%s:%d", redisHost, redisPort),
	})

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		t.Fatalf("Redis ping: %v", err)
	}

	return rdb
}

func TestRedisBus_SubscribeAndPublish(t *testing.T) {
	rdb := newRedisTestClient(t)
	defer rdb.Close()

	bus, err := event.NewRedisBus(rdb, kexswiftbus.WithRedisPrefix("test.eventbus"))
	if err != nil {
		t.Fatalf("NewRedisBus: %v", err)
	}
	defer bus.Close()

	var mu sync.Mutex
	var received []event.Event

	bus.SubscribeEvent(types.ActionUserRegister, func(evt event.Event) {
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	evt := event.Event{
		ID:   "evt-1",
		Type: types.ActionUserRegister,
		Data: map[string]string{"user_id": "user-123"},
	}
	bus.PublishEvent(evt)

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].ID != "evt-1" {
		t.Errorf("expected event ID evt-1, got %s", received[0].ID)
	}
	if received[0].Type != types.ActionUserRegister {
		t.Errorf("expected event type %s, got %s", types.ActionUserRegister, received[0].Type)
	}
	mu.Unlock()
}

func TestRedisBus_MultipleSubscribers(t *testing.T) {
	rdb := newRedisTestClient(t)
	defer rdb.Close()

	bus, err := event.NewRedisBus(rdb, kexswiftbus.WithRedisPrefix("test.eventbus.multi"))
	if err != nil {
		t.Fatalf("NewRedisBus: %v", err)
	}
	defer bus.Close()

	var count1, count2 int
	var mu sync.Mutex

	bus.SubscribeEvent(types.ActionUserLogin, func(evt event.Event) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	bus.SubscribeEvent(types.ActionUserLogin, func(evt event.Event) {
		mu.Lock()
		count2++
		mu.Unlock()
	})

	time.Sleep(100 * time.Millisecond)

	bus.PublishEvent(event.Event{ID: "1", Type: types.ActionUserLogin})

	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if count1 != 1 {
		t.Errorf("handler1: expected 1, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("handler2: expected 1, got %d", count2)
	}
	mu.Unlock()
}

func TestRedisBus_NewRedisBus_NilClient(t *testing.T) {
	_, err := event.NewRedisBus(nil)
	if err == nil {
		t.Fatal("expected error for nil client")
	}
}
