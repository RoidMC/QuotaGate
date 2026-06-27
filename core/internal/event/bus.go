package event

import (
	"fmt"
	"log/slog"

	"github.com/roidmc/quotagate/pkg/kexswiftbus"
)

type EventBus struct {
	*kexswiftbus.Bus
}

// NewBus creates a new instance of EventBus
// Todo: We need to support customizing providers based on YAML configuration in the future
func NewBus() *EventBus {
	return &EventBus{
		Bus: kexswiftbus.NewBus(kexswiftbus.NewMemoryStore()),
	}
}

func (b *EventBus) SubscribeEvent(eventType EventType, handler EventHandler) (kexswiftbus.CancelFunc, error) {
	return b.Bus.Subscribe(string(eventType), func(msg *kexswiftbus.Message) {
		if evt, ok := msg.Payload.(Event); ok {
			handler(evt)
		} else {
			slog.Warn("[quotagate/event] unexpected payload type", "package", "event", "type", fmt.Sprintf("%T", msg.Payload), "topic", string(eventType))
		}
	})
}

func (b *EventBus) PublishEvent(event Event) {
	msg := kexswiftbus.NewMessage(string(event.Type), event)
	msg.ID = event.ID
	msg.Timestamp = event.Timestamp
	if !b.Bus.Publish(msg) {
		slog.Warn("[quotagate/event] message dropped", "package", "event", "topic", event.Type, "event_id", event.ID)
	}
}

func (b *EventBus) PublishEventSync(event Event) error {
	msg := kexswiftbus.NewMessage(string(event.Type), event)
	msg.ID = event.ID
	msg.Timestamp = event.Timestamp
	return b.Bus.PublishSync(msg)
}
