// QuotaGate-only model: billing subscription pre-consume idempotency record (not shared with KexCore IAM)

package billingmodel

import "time"

// SubscriptionPreConsumeRecord stores idempotent pre-consume operations per request
// when billing from a subscription. Without this table, retried requests would
// double-charge the subscription quota.
//
// Lifecycle:
//  1. Pre-consume: insert with Status="consumed" and PreConsumed>0
//  2. On retry:    if record exists with Status="consumed", reuse the same values (no new charge)
//  3. On refund:   set Status="refunded" and revert PreConsumed from subscription's QuotaUsed
type SubscriptionPreConsumeRecord struct {
	ID             string    `gorm:"primaryKey;size:36" json:"id"`
	RequestID      string    `gorm:"column:request_id;size:64;uniqueIndex;not null" json:"request_id"` // idempotency key
	TenantID       string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID         string    `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	SubscriptionID string    `gorm:"column:subscription_id;size:36;index;not null" json:"subscription_id"`
	PreConsumed    int64     `gorm:"column:pre_consumed;not null;default:0" json:"pre_consumed"`
	Status         string    `gorm:"column:status;size:16;not null;default:'consumed';index" json:"status"` // consumed / refunded
	CreatedAt      time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SubscriptionPreConsumeRecord) TableName() string { return "billing_subscription_pre_consume" }

func (SubscriptionPreConsumeRecord) TenantAware() bool { return true }

const (
	SubscriptionPreConsumeStatusConsumed = "consumed"
	SubscriptionPreConsumeStatusRefunded = "refunded"
)
