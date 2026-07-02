package repository_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/repository"
	"gorm.io/gorm"
)

func setupWebhookTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	repo := repository.NewWebhookRepository(db)
	if err := repo.AutoMigrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func TestWebhookRepositoryCreate(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)

	cfg := &model.WebhookConfig{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "Test Webhook",
		URL:      "https://example.com/webhook",
		Secret:   "secret-123",
		Events:   `["user.registered","user.login"]`,
		Active:   true,
	}

	err := repo.Create(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
}

func TestWebhookRepositoryFindByID(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)

	cfg := &model.WebhookConfig{
		ID:       "wh-1",
		TenantID: "tenant-1",
		Name:     "Test Webhook",
		URL:      "https://example.com/webhook",
		Events:   `["user.registered"]`,
		Active:   true,
	}
	repo.Create(context.Background(), cfg)

	t.Run("found", func(t *testing.T) {
		result, err := repo.FindByID(context.Background(), "wh-1")
		if err != nil {
			t.Fatalf("FindByID failed: %v", err)
		}
		if result.Name != "Test Webhook" {
			t.Errorf("expected name 'Test Webhook', got %s", result.Name)
		}
		if result.URL != "https://example.com/webhook" {
			t.Errorf("expected URL 'https://example.com/webhook', got %s", result.URL)
		}
	})

	t.Run("not found", func(t *testing.T) {
		_, err := repo.FindByID(context.Background(), "nonexistent")
		if err != repository.ErrWebhookNotFound {
			t.Errorf("expected ErrWebhookNotFound, got %v", err)
		}
	})
}

func TestWebhookRepositoryFindByTenantID(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)

	configs := []*model.WebhookConfig{
		{ID: "wh-1", TenantID: "tenant-1", Name: "WH1", URL: "https://a.com", Events: `["user.registered"]`, Active: true},
		{ID: "wh-2", TenantID: "tenant-1", Name: "WH2", URL: "https://b.com", Events: `["user.login"]`, Active: true},
		{ID: "wh-3", TenantID: "tenant-2", Name: "WH3", URL: "https://c.com", Events: `["user.registered"]`, Active: true},
	}
	for _, cfg := range configs {
		repo.Create(context.Background(), cfg)
	}

	results, err := repo.FindByTenantID(context.Background(), "tenant-1")
	if err != nil {
		t.Fatalf("FindByTenantID failed: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 webhooks for tenant-1, got %d", len(results))
	}

	results2, err := repo.FindByTenantID(context.Background(), "tenant-2")
	if err != nil {
		t.Fatalf("FindByTenantID failed: %v", err)
	}
	if len(results2) != 1 {
		t.Fatalf("expected 1 webhook for tenant-2, got %d", len(results2))
	}

	results3, err := repo.FindByTenantID(context.Background(), "tenant-3")
	if err != nil {
		t.Fatalf("FindByTenantID failed: %v", err)
	}
	if len(results3) != 0 {
		t.Fatalf("expected 0 webhooks for tenant-3, got %d", len(results3))
	}
}

func TestWebhookRepositoryFindActiveByEvent(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)

	configs := []*model.WebhookConfig{
		{ID: "wh-1", TenantID: "t1", Name: "WH1", URL: "https://a.com", Events: `["user.registered"]`, Active: true},
		{ID: "wh-2", TenantID: "t1", Name: "WH2", URL: "https://b.com", Events: `["user.registered","user.login"]`, Active: true},
		{ID: "wh-3", TenantID: "t1", Name: "WH3", URL: "https://c.com", Events: `["user.login"]`, Active: true},
		{ID: "wh-4", TenantID: "t1", Name: "WH4", URL: "https://d.com", Events: `["user.registered"]`, Active: false},
		{ID: "wh-5", TenantID: "t1", Name: "WH5", URL: "https://e.com", Events: `["*"]`, Active: true},
	}
	for _, cfg := range configs {
		repo.Create(context.Background(), cfg)
	}

	t.Run("match user.registered", func(t *testing.T) {
		results, err := repo.FindActiveByEvent(context.Background(), "user.registered")
		if err != nil {
			t.Fatalf("FindActiveByEvent failed: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 webhooks for user.registered (2 matching + 1 wildcard), got %d", len(results))
		}
	})

	t.Run("match user.login", func(t *testing.T) {
		results, err := repo.FindActiveByEvent(context.Background(), "user.login")
		if err != nil {
			t.Fatalf("FindActiveByEvent failed: %v", err)
		}
		if len(results) != 3 {
			t.Fatalf("expected 3 webhooks for user.login, got %d", len(results))
		}
	})

	t.Run("no match", func(t *testing.T) {
		results, err := repo.FindActiveByEvent(context.Background(), "user.deleted")
		if err != nil {
			t.Fatalf("FindActiveByEvent failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("expected 1 webhook (wildcard) for user.deleted, got %d", len(results))
		}
	})
}

func TestWebhookRepositoryUpdate(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)

	cfg := &model.WebhookConfig{
		ID:       "wh-1",
		TenantID: "t1",
		Name:     "Original",
		URL:      "https://original.com",
		Events:   `["user.registered"]`,
		Active:   true,
	}
	repo.Create(context.Background(), cfg)

	cfg.Name = "Updated"
	cfg.URL = "https://updated.com"
	cfg.Active = false
	err := repo.Update(context.Background(), cfg)
	if err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	result, _ := repo.FindByID(context.Background(), "wh-1")
	if result.Name != "Updated" {
		t.Errorf("expected name 'Updated', got %s", result.Name)
	}
	if result.URL != "https://updated.com" {
		t.Errorf("expected URL 'https://updated.com', got %s", result.URL)
	}
	if result.Active {
		t.Error("expected Active to be false")
	}
}

func TestWebhookRepositoryUpdateNotFound(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)

	cfg := &model.WebhookConfig{
		ID:       "nonexistent",
		TenantID: "t1",
		Name:     "Ghost",
		URL:      "https://ghost.com",
		Events:   `["user.registered"]`,
		Active:   true,
	}
	err := repo.Update(context.Background(), cfg)
	if err != repository.ErrWebhookNotFound {
		t.Errorf("expected ErrWebhookNotFound, got %v", err)
	}
}

func TestWebhookRepositoryDelete(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)

	cfg := &model.WebhookConfig{
		ID:       "wh-1",
		TenantID: "t1",
		Name:     "To Delete",
		URL:      "https://delete.com",
		Events:   `["user.registered"]`,
		Active:   true,
	}
	repo.Create(context.Background(), cfg)

	err := repo.Delete(context.Background(), "wh-1")
	if err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	_, err = repo.FindByID(context.Background(), "wh-1")
	if err != repository.ErrWebhookNotFound {
		t.Errorf("expected ErrWebhookNotFound after delete, got %v", err)
	}
}

func TestWebhookRepositoryDeleteNotFound(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)

	err := repo.Delete(context.Background(), "nonexistent")
	if err != repository.ErrWebhookNotFound {
		t.Errorf("expected ErrWebhookNotFound, got %v", err)
	}
}

func TestWebhookRepositoryDeliveryLog(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)
	repo.AutoMigrate()

	log := &model.WebhookDeliveryLog{
		ID:              "log-1",
		WebhookConfigID: "wh-1",
		EventID:         "evt-1",
		EventType:       "user.registered",
		RequestURL:      "https://example.com/webhook",
		RequestBody:     `{"type":"user.registered"}`,
		ResponseStatus:  200,
		ResponseBody:    "ok",
		DurationMs:      42,
		Success:         true,
		Attempt:         1,
	}

	err := repo.CreateDeliveryLog(context.Background(), log)
	if err != nil {
		t.Fatalf("CreateDeliveryLog failed: %v", err)
	}

	logs, err := repo.ListDeliveryLogs(context.Background(), "wh-1", 10, 0)
	if err != nil {
		t.Fatalf("ListDeliveryLogs failed: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].ID != "log-1" {
		t.Errorf("expected log ID log-1, got %s", logs[0].ID)
	}
	if !logs[0].Success {
		t.Error("expected success to be true")
	}
}

func TestWebhookRepositoryDeliveryLogPagination(t *testing.T) {
	db := setupWebhookTestDB(t)
	repo := repository.NewWebhookRepository(db)
	repo.AutoMigrate()

	for i := 0; i < 5; i++ {
		log := &model.WebhookDeliveryLog{
			ID:              string(rune('a' + i)),
			WebhookConfigID: "wh-1",
			EventID:         fmt.Sprintf("evt-%d", i),
			EventType:       "test",
			RequestURL:      "https://example.com",
			Success:         true,
		}
		repo.CreateDeliveryLog(context.Background(), log)
	}

	logs, err := repo.ListDeliveryLogs(context.Background(), "wh-1", 2, 0)
	if err != nil {
		t.Fatalf("ListDeliveryLogs failed: %v", err)
	}
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}
}
