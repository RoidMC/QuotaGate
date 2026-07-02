package worker_test

import (
	"context"
	"testing"

	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/repository"
	"github.com/roidmc/quotagate/internal/worker"
)

func TestOutboxHandlerCreatesEntriesForActiveWebhooks(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	handler := worker.NewOutboxHandler(repo)

	cfg := &model.WebhookConfig{
		ID:             "wh-1",
		TenantID:       "tenant-1",
		Name:           "test hook",
		URL:            "https://example.com/webhook",
		Secret:         "secret",
		Events:         `["user.register"]`,
		Active:         true,
		RetryCount:     3,
		TimeoutSeconds: 5,
	}
	if err := repo.Create(context.Background(), cfg); err != nil {
		t.Fatalf("failed to create webhook config: %v", err)
	}

	evt := event.Event{
		ID:      "evt-1",
		Type:    "user.register",
		Subject: "tenant-1",
		Data:    map[string]string{"email": "a@example.com"},
	}
	handler(evt)

	var count int64
	if err := db.Model(&model.WebhookOutbox{}).Where("event_id = ?", evt.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count outbox entries: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 outbox entry, got %d", count)
	}
}

func TestOutboxHandlerSkipsInactiveWebhooks(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	handler := worker.NewOutboxHandler(repo)

	cfg := &model.WebhookConfig{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "inactive hook",
		URL:      "https://example.com/webhook",
		Events:   `["user.register"]`,
		Active:   false,
	}
	if err := repo.Create(context.Background(), cfg); err != nil {
		t.Fatalf("failed to create webhook config: %v", err)
	}

	evt := event.Event{ID: "evt-2", Type: "user.register", Subject: "tenant-1"}
	handler(evt)

	var count int64
	db.Model(&model.WebhookOutbox{}).Where("event_id = ?", evt.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 outbox entries for inactive webhook, got %d", count)
	}
}

func TestOutboxHandlerDeduplicatesDuplicateEvents(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	handler := worker.NewOutboxHandler(repo)

	cfg := &model.WebhookConfig{
		ID:             "wh-1",
		TenantID:       "tenant-1",
		Name:           "test hook",
		URL:            "https://example.com/webhook",
		Secret:         "secret",
		Events:         `["user.register"]`,
		Active:         true,
		RetryCount:     3,
		TimeoutSeconds: 5,
	}
	if err := repo.Create(context.Background(), cfg); err != nil {
		t.Fatalf("failed to create webhook config: %v", err)
	}

	evt := event.Event{
		ID:      "evt-dedup",
		Type:    "user.register",
		Subject: "tenant-1",
		Data:    map[string]string{"email": "a@example.com"},
	}
	handler(evt)
	handler(evt) // duplicate publish should be idempotent

	var count int64
	if err := db.Model(&model.WebhookOutbox{}).Where("event_id = ?", evt.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count outbox entries: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 outbox entry after duplicate publish, got %d", count)
	}
}

func TestOutboxHandlerSkipsUnmatchedEvents(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	handler := worker.NewOutboxHandler(repo)

	cfg := &model.WebhookConfig{
		ID:             "wh-1",
		TenantID:       "tenant-1",
		Name:           "login hook",
		URL:            "https://example.com/webhook",
		Secret:         "",
		Events:         `["user.login"]`,
		Active:         true,
		RetryCount:     3,
		TimeoutSeconds: 5,
	}
	if err := repo.Create(context.Background(), cfg); err != nil {
		t.Fatalf("failed to create webhook config: %v", err)
	}

	evt := event.Event{ID: "evt-3", Type: "user.logout", Subject: "tenant-1"}
	handler(evt)

	var count int64
	db.Model(&model.WebhookOutbox{}).Where("event_id = ?", evt.ID).Count(&count)
	if count != 0 {
		t.Errorf("expected 0 outbox entries for unmatched event, got %d", count)
	}
}
