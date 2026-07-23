package event

import (
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/redis/go-redis/v9"
	"github.com/roidmc/kex-utils/pkg/kexswiftbus"
)

type EventBus struct {
	*kexswiftbus.Bus
}

// BusOption configures a new EventBus via functional options.
type BusOption func(*EventBus)

// WithStore replaces the default MemoryStore with a custom Store (e.g., RedisStore).
func WithStore(store kexswiftbus.Store) BusOption {
	return func(b *EventBus) {
		b.Bus = kexswiftbus.NewBus(store)
	}
}

// NewBus creates a new EventBus. By default it uses an in-memory Store.
// Use WithStore to configure a Redis-backed Store or a custom Store.
func NewBus(opts ...BusOption) *EventBus {
	bus := &EventBus{
		Bus: kexswiftbus.NewBus(kexswiftbus.NewMemoryStore()),
	}

	for _, opt := range opts {
		opt(bus)
	}

	return bus
}

// NewRedisBus creates a new EventBus backed by Redis Pub/Sub for cross-instance
// event delivery. The redis.Client is NOT closed when the bus is closed.
func NewRedisBus(client *redis.Client, opts ...kexswiftbus.RedisStoreOption) (*EventBus, error) {
	store, err := kexswiftbus.NewRedisStore(client, opts...)
	if err != nil {
		return nil, fmt.Errorf("event: failed to create redis store: %w", err)
	}
	return &EventBus{
		Bus: kexswiftbus.NewBus(store),
	}, nil
}

func (b *EventBus) SubscribeEvent(eventType EventType, handler EventHandler) (kexswiftbus.CancelFunc, error) {
	return b.Bus.Subscribe(string(eventType), func(msg *kexswiftbus.Message) {
		evt, ok := decodePayload(msg.Payload)
		if !ok {
			slog.Warn("[quotagate/event] unexpected payload type", "package", "event", "type", fmt.Sprintf("%T", msg.Payload), "topic", string(eventType))
			return
		}
		handler(evt)
	})
}

func (b *EventBus) PublishEvent(event Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("[quotagate/event] failed to marshal event", "package", "event", "topic", event.Type, "event_id", event.ID, "error", err)
		return
	}

	msg := kexswiftbus.NewMessage(string(event.Type), string(payload))
	msg.ID = event.ID
	msg.Timestamp = event.Timestamp
	if !b.Bus.Publish(msg) {
		slog.Warn("[quotagate/event] message dropped", "package", "event", "topic", event.Type, "event_id", event.ID)
	}
}

func (b *EventBus) PublishEventSync(event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("event: failed to marshal event: %w", err)
	}

	msg := kexswiftbus.NewMessage(string(event.Type), string(payload))
	msg.ID = event.ID
	msg.Timestamp = event.Timestamp
	return b.Bus.PublishSync(msg)
}

func decodePayload(payload interface{}) (Event, bool) {
	switch v := payload.(type) {
	case Event:
		return v, true
	case string:
		var evt Event
		if err := json.Unmarshal([]byte(v), &evt); err != nil {
			return Event{}, false
		}
		return evt, true
	default:
		return Event{}, false
	}
}
