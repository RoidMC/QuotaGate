package event_test

import (
	"context"
	"testing"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/repository"
	"gorm.io/gorm"
)

func setupTransactionalBusTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	repo := repository.NewWebhookRepository(db)
	if err := repo.AutoMigrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestTransactionalBus_WritesOutboxInsideTransaction(t *testing.T) {
	db := setupTransactionalBusTestDB(t)
	repo := repository.NewWebhookRepository(db)
	bus := event.NewTransactionalBus(event.NewBus(), repo)

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

	evt := event.Event{ID: "evt-1", Type: "user.register", Subject: "tenant-1", Data: map[string]string{"x": "y"}}

	// Simulate a business transaction that publishes an event.
	err := db.Transaction(func(tx *gorm.DB) error {
		return bus.PublishEvent(event.WithTx(context.Background(), tx), evt)
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}

	var count int64
	if err := db.Model(&model.WebhookOutbox{}).Where("event_id = ?", evt.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count outbox: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 outbox entry, got %d", count)
	}
}

func TestTransactionalBus_RollsBackOutboxWhenTransactionRollsBack(t *testing.T) {
	db := setupTransactionalBusTestDB(t)
	repo := repository.NewWebhookRepository(db)
	bus := event.NewTransactionalBus(event.NewBus(), repo)

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

	evt := event.Event{ID: "evt-rollback", Type: "user.register", Subject: "tenant-1"}

	err := db.Transaction(func(tx *gorm.DB) error {
		if err := bus.PublishEvent(event.WithTx(context.Background(), tx), evt); err != nil {
			return err
		}
		return context.Canceled // force rollback
	})
	if err == nil {
		t.Fatal("expected transaction error")
	}

	var count int64
	if err := db.Model(&model.WebhookOutbox{}).Where("event_id = ?", evt.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count outbox: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 outbox entries after rollback, got %d", count)
	}
}

func TestTransactionalBus_FallsBackToBusWithoutTransaction(t *testing.T) {
	db := setupTransactionalBusTestDB(t)
	repo := repository.NewWebhookRepository(db)
	underlying := event.NewBus()
	bus := event.NewTransactionalBus(underlying, repo)

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

	received := make(chan event.Event, 1)
	cancel, err := bus.SubscribeEvent("user.register", func(evt event.Event) {
		received <- evt
	})
	if err != nil {
		t.Fatalf("subscribe failed: %v", err)
	}
	defer cancel()

	evt := event.Event{ID: "evt-bus", Type: "user.register", Subject: "tenant-1"}
	if err := bus.PublishEventSync(context.Background(), evt); err != nil {
		t.Fatalf("publish failed: %v", err)
	}

	select {
	case got := <-received:
		if got.ID != evt.ID {
			t.Errorf("expected event id %s, got %s", evt.ID, got.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("expected event to be delivered via underlying bus")
	}

	// Without a transaction the outbox must remain empty.
	var count int64
	if err := db.Model(&model.WebhookOutbox{}).Where("event_id = ?", evt.ID).Count(&count).Error; err != nil {
		t.Fatalf("failed to count outbox: %v", err)
	}
	if count != 0 {
		t.Errorf("expected 0 outbox entries without transaction, got %d", count)
	}
}
