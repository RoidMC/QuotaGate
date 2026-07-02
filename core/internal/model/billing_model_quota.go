// QuotaGate-only model: per-model quota within a subscription (not shared with KexCore IAM)

package model

import "time"

// ModelQuota tracks per-model quota inside a subscription.
// Without this, a plan including multiple models (gpt-4, claude, gemini) can only
// enforce an aggregate quota via Subscription.QuotaTotal/QuotaUsed — operators cannot
// cap "gpt-4: 1M tokens/month, claude: 500K tokens/month" independently.
//
// Rows are created when a subscription is bound to a plan whose IncludedModels
// declares per-model quotas (encoded as JSON). Absent rows mean the model is
// either not included or falls back to the subscription's aggregate pool.
type ModelQuota struct {
	ID             string `gorm:"primaryKey;size:36" json:"id"`
	TenantID       string `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	SubscriptionID string `gorm:"column:subscription_id;size:36;index;not null" json:"subscription_id"`
	UserID         string `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	ModelID        string `gorm:"column:model_id;size:64;index;not null" json:"model_id"`
	QuotaTotal     int64  `gorm:"column:quota_total;not null;default:0" json:"quota_total"` // 0 = unlimited for this model
	QuotaUsed      int64  `gorm:"column:quota_used;not null;default:0" json:"quota_used"`
	// Reset tracking (mirrors subscription reset schedule; may be overridden per model)
	LastResetTime *time.Time `gorm:"column:last_reset_time;index" json:"last_reset_time,omitempty"`
	NextResetTime *time.Time `gorm:"column:next_reset_time;index" json:"next_reset_time,omitempty"`
	Status        string     `gorm:"size:16;not null;default:'active';index" json:"status"` // active / cancelled
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ModelQuota) TableName() string { return "billing_model_quotas" }

const (
	ModelQuotaStatusActive    = "active"
	ModelQuotaStatusCancelled = "cancelled" // logical deletion; never physically dropped
)
