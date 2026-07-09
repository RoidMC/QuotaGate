package repository

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"time"

	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/types"
	kexrandom "github.com/roidmc/quotagate/internal/util/random"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var (
	ErrWebhookNotFound = errors.New("quotagate/repository: webhook config not found")
)

type WebhookRepository struct {
	db *gorm.DB
}

func NewWebhookRepository(db *gorm.DB) *WebhookRepository {
	return &WebhookRepository{db: db}
}

// DB returns the underlying *gorm.DB. Callers should use it only when they
// need to pass a database handle to methods that accept *gorm.DB directly.
func (r *WebhookRepository) DB() *gorm.DB {
	return r.db
}

func (r *WebhookRepository) AutoMigrate() error {
	if err := r.db.AutoMigrate(&model.WebhookConfig{}); err != nil {
		return err
	}
	if err := r.db.AutoMigrate(&model.WebhookDeliveryLog{}); err != nil {
		return err
	}
	return r.db.AutoMigrate(&model.WebhookOutbox{})
}

func (r *WebhookRepository) Create(ctx context.Context, cfg *model.WebhookConfig) error {
	return r.db.WithContext(ctx).Create(cfg).Error
}

func (r *WebhookRepository) FindByID(ctx context.Context, id string) (*model.WebhookConfig, error) {
	var cfg model.WebhookConfig
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&cfg)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrWebhookNotFound
		}
		return nil, result.Error
	}
	return &cfg, nil
}

func (r *WebhookRepository) FindByTenantID(ctx context.Context, tenantID string) ([]model.WebhookConfig, error) {
	var configs []model.WebhookConfig
	result := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID).Order("created_at DESC").Find(&configs)
	return configs, result.Error
}

func (r *WebhookRepository) FindActiveByEvent(ctx context.Context, eventType types.EventType) ([]model.WebhookConfig, error) {
	return r.findActiveByEvent(r.db.WithContext(ctx), eventType)
}

func (r *WebhookRepository) findActiveByEvent(db *gorm.DB, eventType types.EventType) ([]model.WebhookConfig, error) {
	var configs []model.WebhookConfig
	result := db.Where("active = ?", true).Find(&configs)
	if result.Error != nil {
		return nil, result.Error
	}

	var matched []model.WebhookConfig
	for _, cfg := range configs {
		if eventMatches(cfg.Events, string(eventType)) {
			matched = append(matched, cfg)
		}
	}

	return matched, nil
}

func (r *WebhookRepository) Update(ctx context.Context, cfg *model.WebhookConfig) error {
	result := r.db.WithContext(ctx).Model(&model.WebhookConfig{}).Where("id = ?", cfg.ID).Updates(map[string]interface{}{
		"name":            cfg.Name,
		"url":             cfg.URL,
		"secret":          cfg.Secret,
		"events":          cfg.Events,
		"active":          cfg.Active,
		"retry_count":     cfg.RetryCount,
		"timeout_seconds": cfg.TimeoutSeconds,
	})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

func (r *WebhookRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Delete(&model.WebhookConfig{}, "id = ?", id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

func (r *WebhookRepository) CreateDeliveryLog(ctx context.Context, log *model.WebhookDeliveryLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

func (r *WebhookRepository) ListDeliveryLogs(ctx context.Context, webhookConfigID string, limit, offset int) ([]model.WebhookDeliveryLog, error) {
	var logs []model.WebhookDeliveryLog
	result := r.db.WithContext(ctx).Where("webhook_config_id = ?", webhookConfigID).
		Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&logs)
	return logs, result.Error
}

// CreateOutboxEntries writes one outbox entry per active webhook config that
// matches eventType. The db argument may be a plain *gorm.DB or a transaction
// (*gorm.DB returned by db.Begin()/db.Transaction); all reads and writes are
// executed on it, so callers can make the outbox write atomic with their
// business transaction.
func (r *WebhookRepository) CreateOutboxEntries(db *gorm.DB, eventType types.EventType, eventID, tenantID, payload string) error {
	configs, err := r.findActiveByEvent(db, eventType)
	if err != nil {
		return err
	}
	if len(configs) == 0 {
		return nil
	}

	entries := make([]model.WebhookOutbox, 0, len(configs))
	for _, cfg := range configs {
		entries = append(entries, model.WebhookOutbox{
			ID:             kexrandom.MustUUIDString(),
			EventType:      eventType,
			EventID:        eventID,
			TenantID:       tenantID,
			WebhookID:      cfg.ID,
			URL:            cfg.URL,
			Secret:         cfg.Secret,
			Payload:        payload,
			Status:         model.OutboxStatusPending,
			MaxAttempts:    cfg.RetryCount,
			TimeoutSeconds: cfg.TimeoutSeconds,
			NextAttemptAt:  time.Now(),
		})
	}

	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "event_id"}, {Name: "webhook_id"}},
		DoNothing: true,
	}).Create(&entries).Error
}

// processingLeaseTimeout is how long a processing entry is considered "owned"
// by a worker before it's assumed crashed and can be reclaimed. Must exceed the
// longest possible dispatch (timeout * max_attempts + backoff sum).
const processingLeaseTimeout = 5 * time.Minute

func (r *WebhookRepository) ClaimPendingOutbox(ctx context.Context, limit int) ([]model.WebhookOutbox, error) {
	var entries []model.WebhookOutbox
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		now := time.Now()
		staleBefore := now.Add(-processingLeaseTimeout)

		var raw string
		if isSQLite(tx) {
			raw = `SELECT * FROM webhook_outbox
				   WHERE (status = ? AND next_attempt_at <= ?)
				      OR (status = ? AND updated_at < ?)
				   ORDER BY created_at ASC
				   LIMIT ?`
		} else {
			raw = `SELECT * FROM webhook_outbox
				   WHERE (status = ? AND next_attempt_at <= ?)
				      OR (status = ? AND updated_at < ?)
				   ORDER BY created_at ASC
				   LIMIT ?
				   FOR UPDATE SKIP LOCKED`
		}

		if err := tx.Raw(raw,
			model.OutboxStatusPending, now,
			model.OutboxStatusProcessing, staleBefore,
			limit,
		).Scan(&entries).Error; err != nil {
			return err
		}

		if len(entries) == 0 {
			return nil
		}

		ids := make([]string, len(entries))
		for i, e := range entries {
			ids[i] = e.ID
		}

		return tx.Model(&model.WebhookOutbox{}).
			Where("id IN ?", ids).
			Updates(map[string]interface{}{
				"status":     model.OutboxStatusProcessing,
				"updated_at": now,
			}).Error
	})

	return entries, err
}

func isSQLite(db *gorm.DB) bool {
	if db == nil || db.Dialector == nil {
		return false
	}
	name := db.Dialector.Name()
	return name == "sqlite" || name == "sqlite3"
}

func (r *WebhookRepository) MarkOutboxCompleted(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Model(&model.WebhookOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     model.OutboxStatusCompleted,
		"updated_at": time.Now(),
	}).Error
}

// MarkOutboxDead marks an outbox entry as dead-letter. Use this for permanent
// failures that should not be retried (e.g. unmarshalable payloads).
func (r *WebhookRepository) MarkOutboxDead(ctx context.Context, id string, lastError string) error {
	return r.db.WithContext(ctx).Model(&model.WebhookOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":     model.OutboxStatusDead,
		"last_error": lastError,
		"updated_at": time.Now(),
	}).Error
}

func (r *WebhookRepository) MarkOutboxFailed(ctx context.Context, id string, lastError string) error {
	var entry model.WebhookOutbox
	if err := r.db.WithContext(ctx).Where("id = ?", id).First(&entry).Error; err != nil {
		return err
	}

	nextAttempt := entry.Attempt + 1
	if nextAttempt >= entry.MaxAttempts {
		return r.db.WithContext(ctx).Model(&model.WebhookOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
			"status":     model.OutboxStatusDead,
			"attempt":    nextAttempt,
			"last_error": lastError,
			"updated_at": time.Now(),
		}).Error
	}

	backoff := time.Duration(math.Pow(2, float64(nextAttempt))) * time.Second

	return r.db.WithContext(ctx).Model(&model.WebhookOutbox{}).Where("id = ?", id).Updates(map[string]interface{}{
		"status":          model.OutboxStatusPending,
		"attempt":         nextAttempt,
		"next_attempt_at": time.Now().Add(backoff),
		"last_error":      lastError,
		"updated_at":      time.Now(),
	}).Error
}

func (r *WebhookRepository) CountDeadOutbox(ctx context.Context) (int64, error) {
	var count int64
	err := r.db.WithContext(ctx).Model(&model.WebhookOutbox{}).
		Where("status = ?", model.OutboxStatusDead).
		Count(&count).Error
	return count, err
}

// ListDeadOutbox returns dead-letter outbox entries ordered by creation time
// (oldest first). This is the primary query used by DLQ monitoring and replay
// interfaces.
func (r *WebhookRepository) ListDeadOutbox(ctx context.Context, limit, offset int) ([]model.WebhookOutbox, error) {
	var entries []model.WebhookOutbox
	result := r.db.WithContext(ctx).
		Where("status = ?", model.OutboxStatusDead).
		Order("created_at ASC").
		Limit(limit).Offset(offset).
		Find(&entries)
	return entries, result.Error
}

// ReplayDeadOutbox resets a dead-letter entry back to pending so the worker
// will pick it up again. attempt is reset to 0 and next_attempt_at to now.
// It returns ErrWebhookNotFound if the entry does not exist or is not dead.
func (r *WebhookRepository) ReplayDeadOutbox(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Model(&model.WebhookOutbox{}).
		Where("id = ? AND status = ?", id, model.OutboxStatusDead).
		Updates(map[string]interface{}{
			"status":          model.OutboxStatusPending,
			"attempt":         0,
			"next_attempt_at": time.Now(),
			"last_error":      "",
			"updated_at":      time.Now(),
		})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

func eventMatches(eventsJSON, eventType string) bool {
	var events []string
	if err := json.Unmarshal([]byte(eventsJSON), &events); err != nil {
		return false
	}
	for _, e := range events {
		if e == eventType || e == "*" {
			return true
		}
	}
	return false
}
