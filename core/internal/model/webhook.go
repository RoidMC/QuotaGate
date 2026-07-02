package model

import (
	"time"

	"github.com/roidmc/quotagate/internal/types"
)

type WebhookConfig struct {
	ID             string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID       string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	Name           string    `gorm:"size:128;not null" json:"name"`
	URL            string    `gorm:"size:512;not null" json:"url"`
	Secret         string    `gorm:"size:128;not null;default:''" json:"-"`
	Events         string    `gorm:"type:text;not null" json:"events"`
	Active         bool      `gorm:"not null" json:"active"`
	RetryCount     int       `gorm:"column:retry_count;default:3;not null" json:"retry_count"`
	TimeoutSeconds int       `gorm:"column:timeout_seconds;default:10;not null" json:"timeout_seconds"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (WebhookConfig) TableName() string {
	return "webhook_configs"
}

type WebhookDeliveryLog struct {
	ID              string          `gorm:"primaryKey;size:36" json:"id"`
	WebhookConfigID string          `gorm:"column:webhook_config_id;size:36;index;not null" json:"webhook_config_id"`
	EventID         string          `gorm:"column:event_id;size:36;index;not null" json:"event_id"`
	EventType       types.EventType `gorm:"column:event_type;size:64;not null;index" json:"event_type"`
	RequestURL      string          `gorm:"column:request_url;size:512;not null" json:"request_url"`
	RequestBody     string          `gorm:"column:request_body;type:text" json:"request_body"`
	ResponseStatus  int             `gorm:"column:response_status" json:"response_status"`
	ResponseBody    string          `gorm:"column:response_body;type:text" json:"response_body"`
	DurationMs      int64           `gorm:"column:duration_ms" json:"duration_ms"`
	Success         bool            `gorm:"not null;index" json:"success"`
	Attempt         int             `gorm:"default:1;not null" json:"attempt"`
	Error           string          `gorm:"type:text" json:"error"`
	CreatedAt       time.Time       `gorm:"autoCreateTime;index" json:"created_at"`
}

func (WebhookDeliveryLog) TableName() string {
	return "webhook_delivery_logs"
}

const (
	OutboxStatusPending    = "pending"
	OutboxStatusProcessing = "processing"
	OutboxStatusCompleted  = "completed"
	OutboxStatusFailed     = "failed"
	OutboxStatusDead       = "dead"
)

// WebhookOutbox implements the transactional outbox pattern. When an event
// is published, a handler writes one outbox entry per matching webhook config
// in the same transaction. A background worker then claims pending entries
// with FOR UPDATE SKIP LOCKED and dispatches them via HTTP.
type WebhookOutbox struct {
	ID             string          `gorm:"primaryKey;size:36" json:"id"`
	EventType      types.EventType `gorm:"column:event_type;size:64;not null;index" json:"event_type"`
	EventID        string          `gorm:"column:event_id;size:36;not null;uniqueIndex:idx_webhook_outbox_event_webhook" json:"event_id"`
	TenantID       string          `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	WebhookID      string          `gorm:"column:webhook_id;size:36;not null;uniqueIndex:idx_webhook_outbox_event_webhook" json:"webhook_id"`
	URL            string          `gorm:"column:url;size:512;not null" json:"url"`
	Secret         string          `gorm:"column:secret;size:128;not null;default:''" json:"-"`
	Payload        string          `gorm:"type:text;not null" json:"payload"`
	Status         string          `gorm:"size:16;not null;default:pending;index" json:"status"`
	Attempt        int             `gorm:"default:0;not null" json:"attempt"`
	MaxAttempts    int             `gorm:"column:max_attempts;default:3;not null" json:"max_attempts"`
	TimeoutSeconds int             `gorm:"column:timeout_seconds;default:10;not null" json:"timeout_seconds"`
	NextAttemptAt  time.Time       `gorm:"column:next_attempt_at;index;not null" json:"next_attempt_at"`
	LastError      string          `gorm:"column:last_error;type:text" json:"last_error"`
	CreatedAt      time.Time       `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt      time.Time       `gorm:"autoUpdateTime" json:"updated_at"`
}

func (WebhookOutbox) TableName() string {
	return "webhook_outbox"
}
