// QuotaGate-only model: billing subscription and plan (not shared with KexCore IAM)

package model

import "time"

// Plan defines a subscription tier: quota pool, price, duration, and included models.
type Plan struct {
	ID            string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID      string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	Name          string    `gorm:"size:128;not null" json:"name"`
	Slug          string    `gorm:"size:64;uniqueIndex;not null" json:"slug"`
	Description   string    `gorm:"type:text" json:"description"`
	Type          string    `gorm:"size:32;not null;default:'quota'" json:"type"` // quota / unlimited / hybrid
	QuotaAmount   int64     `gorm:"column:quota_amount;not null;default:0" json:"quota_amount"` // tokens or smallest currency unit
	QuotaUnit     string    `gorm:"column:quota_unit;size:16;default:'token'" json:"quota_unit"`
	Price         int64     `gorm:"not null;default:0" json:"price"` // smallest currency unit
	Currency      string    `gorm:"size:8;not null;default:'CNY'" json:"currency"`
	DurationDays  int       `gorm:"column:duration_days;not null" json:"duration_days"`
	IncludedModels string   `gorm:"column:included_models;type:text" json:"included_models"` // JSON array of model IDs
	Features      string    `gorm:"type:text" json:"features"` // JSON object of feature flags
	Active        bool      `gorm:"default:true;not null" json:"active"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Plan) TableName() string { return "billing_plans" }

// Subscription links a user (or tenant) to a plan with quota tracking and renewal state.
type Subscription struct {
	ID            string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID      string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID        string     `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	PlanID        string     `gorm:"column:plan_id;size:36;index;not null" json:"plan_id"`
	Status        string     `gorm:"size:16;not null;default:'active';index" json:"status"`
	QuotaTotal    int64      `gorm:"column:quota_total;not null;default:0" json:"quota_total"`
	QuotaUsed     int64      `gorm:"column:quota_used;not null;default:0" json:"quota_used"`
	QuotaResetAt  *time.Time `gorm:"column:quota_reset_at" json:"quota_reset_at,omitempty"`
	StartedAt     time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	ExpiresAt     time.Time  `gorm:"column:expires_at;not null;index" json:"expires_at"`
	AutoRenew     bool       `gorm:"column:auto_renew;default:false;not null" json:"auto_renew"`
	PaymentMethod string     `gorm:"column:payment_method;size:32" json:"payment_method"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Subscription) TableName() string { return "billing_subscriptions" }

const (
	PlanTypeQuota     = "quota"
	PlanTypeUnlimited = "unlimited"
	PlanTypeHybrid    = "hybrid"

	SubscriptionStatusActive   = "active"
	SubscriptionStatusExpired  = "expired"
	SubscriptionStatusCancelled = "cancelled"
	SubscriptionStatusPending  = "pending"
)
