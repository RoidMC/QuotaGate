package kexswiftbus_test

import (
	"context"
	"testing"
	"time"

	"github.com/roidmc/kex-utils/pkg/kexswiftbus"
)

func TestMemoryStorePublishSubscribe(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	ch, err := store.Subscribe(ctx, "test.topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	msg := kexswiftbus.NewMessage("test.topic", "hello")
	if err := store.Publish(ctx, msg); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case received := <-ch:
		if received.Topic != "test.topic" {
			t.Errorf("expected topic test.topic, got %s", received.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestMemoryStoreWildcard(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	ch, err := store.Subscribe(ctx, "*")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	msg := kexswiftbus.NewMessage("any.topic", "data")
	if err := store.Publish(ctx, msg); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case received := <-ch:
		if received.Topic != "any.topic" {
			t.Errorf("expected topic any.topic, got %s", received.Topic)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for message")
	}
}

func TestMemoryStoreUnsubscribe(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	ch, err := store.Subscribe(ctx, "test.topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := store.Unsubscribe(ctx, "test.topic", ch); err != nil {
		t.Fatalf("Unsubscribe: %v", err)
	}

	msg := kexswiftbus.NewMessage("test.topic", "hello")
	_ = store.Publish(ctx, msg)

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected no message after unsubscribe")
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestMemoryStoreClose(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()

	ctx := context.Background()
	ch, err := store.Subscribe(ctx, "test.topic")
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := store.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected channel to be closed")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for channel close")
	}

	_, err = store.Subscribe(ctx, "test.topic")
	if err != kexswiftbus.ErrStoreClosed {
		t.Errorf("expected ErrStoreClosed, got %v", err)
	}
}

func TestMemoryStoreMultipleSubscribers(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	ch1, _ := store.Subscribe(ctx, "test.topic")
	ch2, _ := store.Subscribe(ctx, "test.topic")

	msg := kexswiftbus.NewMessage("test.topic", "hello")
	if err := store.Publish(ctx, msg); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	for i, ch := range []<-chan *kexswiftbus.Message{ch1, ch2} {
		select {
		case received := <-ch:
			if received.Topic != "test.topic" {
				t.Errorf("subscriber %d: expected topic test.topic, got %s", i, received.Topic)
			}
		case <-time.After(time.Second):
			t.Fatalf("subscriber %d: timeout waiting for message", i)
		}
	}
}

func TestMemoryStoreNoSubscriber(t *testing.T) {
	store := kexswiftbus.NewMemoryStore()
	defer store.Close()

	ctx := context.Background()
	msg := kexswiftbus.NewMessage("no.subscriber", "hello")
	if err := store.Publish(ctx, msg); err != nil {
		t.Fatalf("Publish with no subscriber should not error: %v", err)
	}
}
