package event

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/roidmc/quotagate/pkg/kexswiftbus"
	"gorm.io/gorm"
)

// OutboxWriter persists events to a transactional outbox. The database handle
// passed to CreateOutboxEntries may be a plain *gorm.DB or a transaction; the
// implementation must execute all reads and writes on that handle so the caller
// can make outbox writes atomic with its business transaction.
//
// This interface is typically implemented by a repository in the data layer.
// Keeping it in the event package lets event publishers depend on an abstract
// contract rather than a concrete repository, which avoids coupling the
// infrastructure event bus to application-specific storage details.
type OutboxWriter interface {
	CreateOutboxEntries(db *gorm.DB, eventType EventType, eventID, tenantID, payload string) error
}

// TransactionalBus wraps an EventBus and routes event publishing to the
// transactional outbox whenever a GORM transaction is present in the context.
// If no transaction is present, it falls back to the underlying bus so that
// in-memory notifications or non-webhook handlers still work as usual.
//
// This keeps the publisher decoupled from both the event bus implementation
// (memory, Redis, RabbitMQ) and the concrete outbox storage implementation.
type TransactionalBus struct {
	bus          *EventBus
	outboxWriter OutboxWriter
}

// NewTransactionalBus creates a bus that writes outbox entries inside the
// transaction carried by the publish context, falling back to the event bus
// otherwise. The outboxWriter argument must implement OutboxWriter; in
// QuotaGate the webhook repository satisfies this interface.
func NewTransactionalBus(bus *EventBus, outboxWriter OutboxWriter) *TransactionalBus {
	return &TransactionalBus{bus: bus, outboxWriter: outboxWriter}
}

// PublishEvent publishes the event. If ctx carries a transaction (see WithTx),
// the event is serialized and written to the transactional outbox in the same
// transaction. Otherwise it is forwarded to the underlying EventBus for
// immediate delivery to subscribers.
func (b *TransactionalBus) PublishEvent(ctx context.Context, evt Event) error {
	if tx, ok := TxFromContext(ctx); ok {
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("event: failed to marshal event for outbox: %w", err)
		}
		return b.outboxWriter.CreateOutboxEntries(tx, evt.Type, evt.ID, evt.Subject, string(payload))
	}

	b.bus.PublishEvent(evt)
	return nil
}

// PublishEventSync is the synchronous equivalent of PublishEvent. It writes to
// the outbox when a transaction is present; otherwise it forwards to the
// underlying EventBus.PublishEventSync.
func (b *TransactionalBus) PublishEventSync(ctx context.Context, evt Event) error {
	if tx, ok := TxFromContext(ctx); ok {
		payload, err := json.Marshal(evt)
		if err != nil {
			return fmt.Errorf("event: failed to marshal event for outbox: %w", err)
		}
		return b.outboxWriter.CreateOutboxEntries(tx, evt.Type, evt.ID, evt.Subject, string(payload))
	}

	return b.bus.PublishEventSync(evt)
}

// SubscribeEvent delegates to the underlying EventBus.
func (b *TransactionalBus) SubscribeEvent(eventType EventType, handler EventHandler) (kexswiftbus.CancelFunc, error) {
	return b.bus.SubscribeEvent(eventType, handler)
}

// Bus exposes the underlying EventBus for direct access when needed.
func (b *TransactionalBus) Bus() *EventBus {
	return b.bus
}
