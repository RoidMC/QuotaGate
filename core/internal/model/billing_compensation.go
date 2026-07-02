// QuotaGate-only model: billing compensation (not shared with KexCore IAM)

package model

import "time"

// CompensationRow represents a billing compensation (retry/refund) task.
type CompensationRow struct {
	ID           int64     `json:"id" gorm:"primaryKey;autoIncrement"`
	WALID        int64     `json:"wal_id" gorm:"uniqueIndex;not null"`
	Action       string    `json:"action" gorm:"type:varchar(32);not null"`
	Status       string    `json:"status" gorm:"type:varchar(32);not null;default:'pending'"`
	RetryCount   int       `json:"retry_count" gorm:"default:0"`
	MaxRetry     int       `json:"max_retry" gorm:"default:3"`
	NextRetryAt  int64     `json:"next_retry_at" gorm:"index"`
	ErrorMessage string    `json:"error_message" gorm:"type:text"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

func (CompensationRow) TableName() string { return "billing_compensation" }

const (
	CompensationStatusPending    = "pending"
	CompensationStatusProcessing = "processing"
	CompensationStatusCompleted  = "completed"
	CompensationStatusFailed     = "failed"
)
