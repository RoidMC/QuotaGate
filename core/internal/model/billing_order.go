// QuotaGate-only model: billing order (not shared with KexCore IAM)

package model

import "time"

// Order groups one or more billing operations into a single business unit.
// For API gateway usage, an order typically maps to one upstream request or
// one chat completion call that may involve multiple model/provider attempts.
type Order struct {
	ID            string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID      string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID        string     `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	RequestID     string     `gorm:"column:request_id;size:64;uniqueIndex;not null" json:"request_id"`
	WALID         *int64     `gorm:"column:wal_id;index" json:"wal_id,omitempty"`
	ChannelID     string     `gorm:"column:channel_id;size:36;index" json:"channel_id"`
	ModelID       string     `gorm:"column:model_id;size:64;index" json:"model_id"`
	PromptTokens  int64      `gorm:"column:prompt_tokens;not null;default:0" json:"prompt_tokens"`
	CompletionTokens int64   `gorm:"column:completion_tokens;not null;default:0" json:"completion_tokens"`
	TotalTokens   int64      `gorm:"column:total_tokens;not null;default:0" json:"total_tokens"`
	Amount        int64      `gorm:"not null;default:0" json:"amount"` // final billed amount in smallest unit
	Currency      string     `gorm:"size:8;not null;default:'CNY'" json:"currency"`
	Status        string     `gorm:"size:16;not null;default:'pending';index" json:"status"`
	SettledAt     *time.Time `gorm:"column:settled_at" json:"settled_at,omitempty"`
	ErrorMessage  string     `gorm:"column:error_message;type:text" json:"error_message"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Order) TableName() string { return "billing_orders" }

const (
	OrderStatusPending   = "pending"
	OrderStatusSettled   = "settled"
	OrderStatusRefunded  = "refunded"
	OrderStatusFailed    = "failed"
	OrderStatusCancelled = "cancelled"
)
