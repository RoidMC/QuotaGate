package event_test

import (
	"sync"
	"testing"
	"time"

	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/types"
)

func TestBusSubscribeAndPublish(t *testing.T) {
	bus := event.NewBus()
	defer bus.Close()

	var mu sync.Mutex
	received := make([]event.Event, 0)

	bus.SubscribeEvent(types.ActionUserRegister, func(evt event.Event) {
		mu.Lock()
		received = append(received, evt)
		mu.Unlock()
	})

	evt := event.Event{
		ID:   "evt-1",
		Type: types.ActionUserRegister,
		Data: map[string]string{"user_id": "user-123"},
	}

	bus.PublishEvent(evt)

	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if len(received) != 1 {
		t.Fatalf("expected 1 event, got %d", len(received))
	}
	if received[0].ID != "evt-1" {
		t.Errorf("expected event ID evt-1, got %s", received[0].ID)
	}
	mu.Unlock()
}

func TestBusCancelSubscription(t *testing.T) {
	bus := event.NewBus()
	defer bus.Close()

	var count int
	var mu sync.Mutex

	cancel, err := bus.SubscribeEvent(types.ActionUserLogin, func(evt event.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("SubscribeEvent failed: %v", err)
	}

	bus.PublishEvent(event.Event{ID: "1", Type: types.ActionUserLogin})
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
	mu.Unlock()

	cancel()

	bus.PublishEvent(event.Event{ID: "2", Type: types.ActionUserLogin})
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if count != 1 {
		t.Errorf("expected still 1 after cancel, got %d", count)
	}
	mu.Unlock()
}

func TestBusMultipleHandlers(t *testing.T) {
	bus := event.NewBus()
	defer bus.Close()

	var count1, count2 int
	var mu sync.Mutex

	bus.SubscribeEvent(types.ActionUserRegister, func(evt event.Event) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	bus.SubscribeEvent(types.ActionUserRegister, func(evt event.Event) {
		mu.Lock()
		count2++
		mu.Unlock()
	})

	bus.PublishEvent(event.Event{ID: "1", Type: types.ActionUserRegister})
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if count1 != 1 {
		t.Errorf("handler1: expected 1, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("handler2: expected 1, got %d", count2)
	}
	mu.Unlock()
}

func TestBusNoHandlerForEventType(t *testing.T) {
	bus := event.NewBus()
	defer bus.Close()

	bus.PublishEvent(event.Event{ID: "1", Type: types.ActionUserDelete})
	time.Sleep(50 * time.Millisecond)
}

func TestBusClose(t *testing.T) {
	bus := event.NewBus()

	var count int
	var mu sync.Mutex

	bus.SubscribeEvent(types.ActionUserRegister, func(evt event.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Close()

	bus.PublishEvent(event.Event{ID: "1", Type: types.ActionUserRegister})
	time.Sleep(50 * time.Millisecond)

	mu.Lock()
	if count != 0 {
		t.Errorf("expected 0 events after close, got %d", count)
	}
	mu.Unlock()
}

func TestBusConcurrentPublish(t *testing.T) {
	bus := event.NewBus()
	defer bus.Close()

	var count int
	var mu sync.Mutex

	bus.SubscribeEvent(types.ActionUserLogin, func(evt event.Event) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			bus.PublishEvent(event.Event{
				ID:   string(rune(n)),
				Type: types.ActionUserLogin,
			})
		}(i)
	}
	wg.Wait()

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count != 50 {
		t.Errorf("expected 50 events, got %d", count)
	}
	mu.Unlock()
}
