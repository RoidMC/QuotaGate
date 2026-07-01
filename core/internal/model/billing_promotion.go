// QuotaGate-only model: billing promotion and coupon (not shared with KexCore IAM)

package model

import "time"

// Promotion defines a global discount/grant campaign: e.g. first recharge 10% bonus,
// or a fixed amount of trial quota.
type Promotion struct {
	ID            string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID      string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	Name          string     `gorm:"size:128;not null" json:"name"`
	Code          string     `gorm:"size:32;uniqueIndex" json:"code"`
	Type          string     `gorm:"size:32;not null" json:"type"` // recharge_bonus / trial_quota / discount
	Value         int64      `gorm:"not null" json:"value"`        // amount or ratio in basis points
	Currency      string     `gorm:"size:8" json:"currency"`
	MinAmount     int64      `gorm:"column:min_amount;default:0" json:"min_amount"`
	MaxDiscount   int64      `gorm:"column:max_discount;default:0" json:"max_discount"`
	MaxUsage      int        `gorm:"column:max_usage;default:0" json:"max_usage"` // 0 = unlimited
	UsedCount     int        `gorm:"column:used_count;default:0" json:"used_count"`
	StartedAt     time.Time  `gorm:"column:started_at;not null" json:"started_at"`
	ExpiresAt     *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	Active        bool       `gorm:"default:true;not null" json:"active"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Promotion) TableName() string { return "billing_promotions" }

// Coupon links a promotion to a specific user (or is issued as a redeemable code).
type Coupon struct {
	ID          string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID    string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID      string     `gorm:"column:user_id;size:36;index" json:"user_id"`
	PromotionID string     `gorm:"column:promotion_id;size:36;index;not null" json:"promotion_id"`
	Code        string     `gorm:"size:32;uniqueIndex" json:"code"`
	Status      string     `gorm:"size:16;not null;default:'unused';index" json:"status"`
	UsedAt      *time.Time `gorm:"column:used_at" json:"used_at,omitempty"`
	ExpiresAt   *time.Time `gorm:"column:expires_at" json:"expires_at,omitempty"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Coupon) TableName() string { return "billing_coupons" }

const (
	PromotionTypeRechargeBonus = "recharge_bonus"
	PromotionTypeTrialQuota    = "trial_quota"
	PromotionTypeDiscount      = "discount"

	CouponStatusUnused   = "unused"
	CouponStatusUsed     = "used"
	CouponStatusExpired  = "expired"
	CouponStatusDisabled = "disabled"
)
