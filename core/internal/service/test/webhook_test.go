package service_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/repository"
	"github.com/roidmc/quotagate/internal/service"
	"gorm.io/gorm"
)

func setupWebhookServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	sqlDB, _ := db.DB()
	sqlDB.SetMaxOpenConns(1)

	repo := repository.NewWebhookRepository(db)
	if err := repo.AutoMigrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestWebhookServiceRotateSecret(t *testing.T) {
	db := setupWebhookServiceTestDB(t)
	repo := repository.NewWebhookRepository(db)
	svc := service.NewWebhookService(repo, event.NewBus())

	created, err := svc.Create(context.Background(), &service.CreateWebhookRequest{
		TenantID: "tenant-1",
		Name:     "Test Hook",
		URL:      "https://example.com/webhook",
		Secret:   "old-secret",
		Events:   []string{"user.register"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	newSecret, err := svc.RotateSecret(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("RotateSecret failed: %v", err)
	}
	if newSecret == "" {
		t.Fatal("expected non-empty new secret")
	}
	if newSecret == "old-secret" {
		t.Fatal("expected new secret to differ from old secret")
	}

	cfg, err := repo.FindByID(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("FindByID failed: %v", err)
	}
	if cfg.Secret != newSecret {
		t.Errorf("expected stored secret to match rotated secret, got %q", cfg.Secret)
	}
}

func TestWebhookServiceRotateSecretNotFound(t *testing.T) {
	db := setupWebhookServiceTestDB(t)
	repo := repository.NewWebhookRepository(db)
	svc := service.NewWebhookService(repo, event.NewBus())

	_, err := svc.RotateSecret(context.Background(), "nonexistent")
	if err != service.ErrWebhookConfigNotFound {
		t.Errorf("expected ErrWebhookConfigNotFound, got %v", err)
	}
}

func TestWebhookServiceDeliveryLogIncludesEventID(t *testing.T) {
	db := setupWebhookServiceTestDB(t)
	repo := repository.NewWebhookRepository(db)
	svc := service.NewWebhookService(repo, event.NewBus())

	created, err := svc.Create(context.Background(), &service.CreateWebhookRequest{
		TenantID: "tenant-1",
		Name:     "Test Hook",
		URL:      "https://example.com/webhook",
		Events:   []string{"user.register"},
	})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	// Create a delivery log directly through repository to verify EventID persistence.
	log := &model.WebhookDeliveryLog{
		ID:              "log-1",
		WebhookConfigID: created.ID,
		EventID:         "evt-123",
		EventType:       "user.register",
		RequestURL:      created.URL,
		Success:         true,
	}
	if err := repo.CreateDeliveryLog(context.Background(), log); err != nil {
		t.Fatalf("CreateDeliveryLog failed: %v", err)
	}

	logs, err := svc.GetDeliveryLogs(context.Background(), created.ID, 10, 0)
	if err != nil {
		t.Fatalf("GetDeliveryLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].EventID != "evt-123" {
		t.Errorf("expected EventID evt-123, got %s", logs[0].EventID)
	}
}
