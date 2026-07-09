package worker

import (
	"encoding/json"
	"log/slog"

	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/repository"
)

// NewOutboxHandler creates an EventHandler that serializes events to JSON
// and writes them to the webhook outbox table. One outbox entry is created
// per matching active webhook config. The WebhookWorker then picks up
// pending entries and dispatches them via HTTP.
//
// Usage:
//
//	bus.Subscribe(event.Wildcard, worker.NewOutboxHandler(webhookRepo))
func NewOutboxHandler(repo *repository.WebhookRepository) event.EventHandler {
	return func(evt event.Event) {
		payload, err := json.Marshal(evt)
		if err != nil {
			slog.Error("quotagate/worker: marshal event for outbox failed", "error", err)
			return
		}

		// event.EventHandler does not carry a transaction; use the repository's
		// default database connection. For transactional publishing use
		// event.TransactionalBus instead.
		if err := repo.CreateOutboxEntries(repo.DB(), evt.Type, evt.ID, evt.Subject, string(payload)); err != nil {
			slog.Error("quotagate/worker: create outbox entries failed", "error", err)
		}
	}
}
