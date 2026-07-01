// QuotaGate-only model: billing WAL (not shared with KexCore IAM)

package model

import "time"

// WALRow represents a billing write-ahead log row.
type WALRow struct {
	ID             int64      `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestID      string     `json:"request_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	UserID         int        `json:"user_id" gorm:"index;not null"`
	SubscriptionID *int64     `json:"subscription_id,omitempty" gorm:"index"`
	PreConsumed    int64      `json:"pre_consumed" gorm:"not null"`
	ActualConsumed int64      `json:"actual_consumed" gorm:"default:0"`
	Status         string     `json:"status" gorm:"type:varchar(32);not null;default:'pending'"`
	BillingSource  string     `json:"billing_source" gorm:"type:varchar(32);not null"`
	PaymentMethod  string     `json:"payment_method" gorm:"type:varchar(32)"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
	SettledAt      *time.Time `json:"settled_at,omitempty"`
	RefundedAt     *time.Time `json:"refunded_at,omitempty"`
	ErrorMessage   string     `json:"error_message" gorm:"type:text"`
}

func (WALRow) TableName() string { return "billing_wal" }
