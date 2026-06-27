package kexswiftbus_test

import (
	"sync"
	"testing"
	"time"

	"github.com/roidmc/quotagate/pkg/kexswiftbus"
)

func TestBusSubscribeAndPublish(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	defer bus.Close()

	var mu sync.Mutex
	var received []string

	bus.Subscribe("user.created", func(msg *kexswiftbus.Message) {
		mu.Lock()
		received = append(received, msg.Topic)
		mu.Unlock()
	})

	time.Sleep(50 * time.Millisecond)

	bus.Publish(kexswiftbus.NewMessage("user.created", map[string]string{"id": "123"}))

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(received) != 1 {
		t.Fatalf("expected 1 message, got %d", len(received))
	}
	if received[0] != "user.created" {
		t.Errorf("expected topic user.created, got %s", received[0])
	}
	mu.Unlock()
}

func TestBusWildcard(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	defer bus.Close()

	var mu sync.Mutex
	var received []string

	bus.Subscribe(kexswiftbus.Wildcard, func(msg *kexswiftbus.Message) {
		mu.Lock()
		received = append(received, msg.Topic)
		mu.Unlock()
	})

	time.Sleep(50 * time.Millisecond)

	bus.Publish(kexswiftbus.NewMessage("order.placed", nil))
	bus.Publish(kexswiftbus.NewMessage("user.login", nil))

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if len(received) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(received))
	}
	mu.Unlock()
}

func TestBusMultipleHandlers(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	defer bus.Close()

	var count1, count2 int
	var mu sync.Mutex

	bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {
		mu.Lock()
		count2++
		mu.Unlock()
	})

	time.Sleep(50 * time.Millisecond)

	bus.Publish(kexswiftbus.NewMessage("test.event", nil))

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count1 != 1 {
		t.Errorf("handler1: expected 1, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("handler2: expected 1, got %d", count2)
	}
	mu.Unlock()
}

func TestBusCancelSubscription(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	defer bus.Close()

	var count int
	var mu sync.Mutex

	cancel, err := bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {
		mu.Lock()
		count++
		mu.Unlock()
	})
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	bus.Publish(kexswiftbus.NewMessage("test.event", nil))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count != 1 {
		t.Fatalf("expected 1, got %d", count)
	}
	mu.Unlock()

	cancel()

	bus.Publish(kexswiftbus.NewMessage("test.event", nil))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count != 1 {
		t.Errorf("expected still 1 after cancel, got %d", count)
	}
	mu.Unlock()
}

func TestBusCancelIdempotent(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	defer bus.Close()

	cancel, _ := bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {})

	cancel()
	cancel()
}

func TestBusClose(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)

	var count int
	var mu sync.Mutex

	bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	bus.Close()

	bus.Publish(kexswiftbus.NewMessage("test.event", nil))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count != 0 {
		t.Errorf("expected 0 events after close, got %d", count)
	}
	mu.Unlock()
}

func TestBusConcurrentPublish(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	defer bus.Close()

	var count int
	var mu sync.Mutex

	bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {
		mu.Lock()
		count++
		mu.Unlock()
	})

	time.Sleep(50 * time.Millisecond)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bus.Publish(kexswiftbus.NewMessage("test.event", nil))
		}()
	}
	wg.Wait()
	time.Sleep(200 * time.Millisecond)

	mu.Lock()
	if count != 50 {
		t.Errorf("expected 50 events, got %d", count)
	}
	mu.Unlock()
}

func TestBusPublishSync(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	defer bus.Close()

	var received bool
	var mu sync.Mutex

	bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {
		mu.Lock()
		received = true
		mu.Unlock()
	})

	time.Sleep(50 * time.Millisecond)

	if err := bus.PublishSync(kexswiftbus.NewMessage("test.event", nil)); err != nil {
		t.Fatalf("PublishSync failed: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if !received {
		t.Error("expected message to be received")
	}
	mu.Unlock()
}

func TestBusPublishAfterClose(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	bus.Close()

	if bus.Publish(kexswiftbus.NewMessage("test.event", nil)) {
		t.Error("expected Publish to return false after close")
	}

	if err := bus.PublishSync(kexswiftbus.NewMessage("test.event", nil)); err == nil {
		t.Error("expected PublishSync to return error after close")
	}
}

func TestBusSubscribeAfterClose(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	bus.Close()

	_, err := bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {})
	if err == nil {
		t.Error("expected Subscribe to return error after close")
	}
}

func TestBusCancelOneHandlerKeepsOther(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	bus := kexswiftbus.NewBus(store)
	defer bus.Close()

	var count1, count2 int
	var mu sync.Mutex

	cancel1, _ := bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {
		mu.Lock()
		count1++
		mu.Unlock()
	})
	bus.Subscribe("test.event", func(msg *kexswiftbus.Message) {
		mu.Lock()
		count2++
		mu.Unlock()
	})

	time.Sleep(50 * time.Millisecond)

	bus.Publish(kexswiftbus.NewMessage("test.event", nil))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count1 != 1 || count2 != 1 {
		t.Errorf("expected both 1, got %d and %d", count1, count2)
	}
	mu.Unlock()

	cancel1()
	bus.Publish(kexswiftbus.NewMessage("test.event", nil))
	time.Sleep(100 * time.Millisecond)

	mu.Lock()
	if count1 != 1 {
		t.Errorf("handler1 expected still 1 after cancel, got %d", count1)
	}
	if count2 != 2 {
		t.Errorf("handler2 expected 2, got %d", count2)
	}
	mu.Unlock()
}
