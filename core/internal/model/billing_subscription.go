// QuotaGate-only model: billing subscription and plan (not shared with KexCore IAM)

package model

import "time"

// Plan defines a subscription tier: quota pool, price, duration, and included models.
type Plan struct {
	ID          string `gorm:"primaryKey;size:36" json:"id"`
	TenantID    string `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	Name        string `gorm:"size:128;not null" json:"name"`
	Slug        string `gorm:"size:64;uniqueIndex;not null" json:"slug"`
	Description string `gorm:"type:text" json:"description"`
	Type        string `gorm:"size:32;not null;default:'quota'" json:"type"`               // quota / unlimited / hybrid
	QuotaAmount int64  `gorm:"column:quota_amount;not null;default:0" json:"quota_amount"` // tokens or smallest currency unit
	QuotaUnit   string `gorm:"column:quota_unit;size:16;default:'token'" json:"quota_unit"`
	Price       int64  `gorm:"not null;default:0" json:"price"` // smallest currency unit
	Currency    string `gorm:"size:8;not null;default:'CNY'" json:"currency"`
	// Duration: unit + value (year/month/day/hour) or custom seconds
	DurationUnit  string `gorm:"column:duration_unit;size:16;not null;default:'month'" json:"duration_unit"`
	DurationValue int    `gorm:"column:duration_value;not null;default:1" json:"duration_value"`
	CustomSeconds int64  `gorm:"column:custom_seconds;not null;default:0" json:"custom_seconds"` // used when DurationUnit == "custom"
	// Quota reset: recurring reset of AmountUsed during the subscription lifetime
	QuotaResetPeriod        string `gorm:"column:quota_reset_period;size:16;not null;default:'never'" json:"quota_reset_period"` // never/daily/weekly/monthly/custom
	QuotaResetCustomSeconds int64  `gorm:"column:quota_reset_custom_seconds;not null;default:0" json:"quota_reset_custom_seconds"`
	// Purchase controls
	MaxPurchasePerUser  int   `gorm:"column:max_purchase_per_user;not null;default:0" json:"max_purchase_per_user"` // 0 = unlimited
	AllowBalancePay     *bool `gorm:"column:allow_balance_pay" json:"allow_balance_pay,omitempty"`                  // nil = true
	AllowWalletOverflow *bool `gorm:"column:allow_wallet_overflow" json:"allow_wallet_overflow,omitempty"`          // nil = true; allow wallet fallback after quota exhausted
	// User group transitions
	UpgradeGroup   string    `gorm:"column:upgrade_group;size:64;default:''" json:"upgrade_group"`     // upgrade user group on purchase (empty = no change)
	DowngradeGroup string    `gorm:"column:downgrade_group;size:64;default:''" json:"downgrade_group"` // downgrade target on expiry (empty = revert to prev group)
	IncludedModels string    `gorm:"column:included_models;type:text" json:"included_models"`          // JSON array of model IDs
	Features       string    `gorm:"type:text" json:"features"`                                        // JSON object of feature flags
	Active         bool      `gorm:"default:true;not null" json:"active"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Plan) TableName() string { return "billing_plans" }

// Subscription links a user (or tenant) to a plan with quota tracking and renewal state.
type Subscription struct {
	ID           string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID     string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID       string     `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	PlanID       string     `gorm:"column:plan_id;size:36;index;not null" json:"plan_id"`
	Status       string     `gorm:"size:16;not null;default:'active';index" json:"status"`
	QuotaTotal   int64      `gorm:"column:quota_total;not null;default:0" json:"quota_total"`
	QuotaUsed    int64      `gorm:"column:quota_used;not null;default:0" json:"quota_used"`
	QuotaResetAt *time.Time `gorm:"column:quota_reset_at" json:"quota_reset_at,omitempty"` // legacy single reset timestamp; prefer LastResetTime/NextResetTime
	// Reset tracking (mirrors plan's reset period; advanced at runtime)
	LastResetTime *time.Time `gorm:"column:last_reset_time;index" json:"last_reset_time,omitempty"`
	NextResetTime *time.Time `gorm:"column:next_reset_time;index" json:"next_reset_time,omitempty"`
	StartedAt     time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	ExpiresAt     time.Time  `gorm:"column:expires_at;not null;index" json:"expires_at"`
	AutoRenew     bool       `gorm:"column:auto_renew;default:false;not null" json:"auto_renew"`
	PaymentMethod string     `gorm:"column:payment_method;size:32" json:"payment_method"`
	// Source: how this subscription was created (order/admin/grant)
	Source string `gorm:"column:source;size:32;not null;default:'order'" json:"source"`
	// Price snapshot locked at purchase time. Plans may change price later;
	// these fields preserve the originally purchased terms for the subscription's
	// lifetime so renewals and refunds charge the correct amount.
	SnapshotPrice    int64  `gorm:"column:snapshot_price;not null;default:0" json:"snapshot_price"`
	SnapshotCurrency string `gorm:"column:snapshot_currency;size:8;not null;default:'CNY'" json:"snapshot_currency"`
	// User group transitions (snapshotted from plan at purchase time)
	UpgradeGroup   string `gorm:"column:upgrade_group;size:64;default:''" json:"upgrade_group"`
	PrevUserGroup  string `gorm:"column:prev_user_group;size:64;default:''" json:"prev_user_group"`
	DowngradeGroup string `gorm:"column:downgrade_group;size:64;default:''" json:"downgrade_group"`
	// Wallet fallback (snapshotted from plan)
	AllowWalletOverflow bool      `gorm:"column:allow_wallet_overflow;not null;default:true" json:"allow_wallet_overflow"`
	CreatedAt           time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Subscription) TableName() string { return "billing_subscriptions" }

const (
	PlanTypeQuota     = "quota"
	PlanTypeUnlimited = "unlimited"
	PlanTypeHybrid    = "hybrid"

	SubscriptionStatusActive    = "active"
	SubscriptionStatusExpired   = "expired"
	SubscriptionStatusCancelled = "cancelled"
	SubscriptionStatusPending   = "pending"

	SubscriptionDurationYear   = "year"
	SubscriptionDurationMonth  = "month"
	SubscriptionDurationDay    = "day"
	SubscriptionDurationHour   = "hour"
	SubscriptionDurationCustom = "custom"

	SubscriptionResetNever   = "never"
	SubscriptionResetDaily   = "daily"
	SubscriptionResetWeekly  = "weekly"
	SubscriptionResetMonthly = "monthly"
	SubscriptionResetCustom  = "custom"

	SubscriptionSourceOrder = "order"
	SubscriptionSourceAdmin = "admin"
	SubscriptionSourceGrant = "grant"
)
