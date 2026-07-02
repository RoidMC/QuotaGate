// Package model defines optional growth modules for QuotaGate:
// affiliate, inviter, check-in, and top-up.
//
// These models are intentionally isolated from internal/model. They belong to
// the growth service's own database instance and must not be migrated into
// the shared schema contract.
package model

import "time"

// AffiliateRelation records one level of an affiliate chain: who referred
// whom and at what ratio.  Supports multi-level affiliate programs without
// denormalizing the user or transaction tables.
type AffiliateRelation struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	AffiliateID string    `gorm:"column:affiliate_id;size:36;index;not null" json:"affiliate_id"` // referrer
	ReferredID  string    `gorm:"column:referred_id;size:36;index;not null" json:"referred_id"`   // referral target
	Level       int       `gorm:"not null;default:1" json:"level"` // 1 = direct, 2 = indirect, etc.
	ReferralCode string   `gorm:"column:referral_code;size:32;index" json:"referral_code"` // optional source code
	CreatedAt   time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (AffiliateRelation) TableName() string { return "affiliate_relations" }

// AffiliateEarning records a concrete commission credit.  Earning events are
// append-only; a settlement worker periodically moves them to Wallet.
type AffiliateEarning struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	AffiliateID string    `gorm:"column:affiliate_id;size:36;index;not null" json:"affiliate_id"`
	ReferredID  string    `gorm:"column:referred_id;size:36;not null" json:"referred_id"`
	OrderID     string    `gorm:"column:order_id;size:36;index;not null" json:"order_id"` // which Order triggered this
	Amount      int64     `gorm:"not null" json:"amount"`             // commission amount
	Rate        int64     `gorm:"not null" json:"rate"`               // basis points (e.g. 1000 = 10%)
	Status      string    `gorm:"size:16;not null;default:'pending';index" json:"status"` // pending / settled / failed
	SettledAt   *time.Time `gorm:"column:settled_at" json:"settled_at,omitempty"`
	ErrorMessage string   `gorm:"column:error_message;size:255" json:"error_message"`
	CreatedAt   time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AffiliateEarning) TableName() string { return "affiliate_earnings" }

// AffiliateCommissionRule defines the affiliate commission policy per tenant
// or per plan.  The engine evaluates rules by tenant first, then falls back to
// a global default rule.
type AffiliateCommissionRule struct {
	ID            string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID      string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	PlanID        string    `gorm:"column:plan_id;size:36;index" json:"plan_id"` // empty = global default
	Rate          int64     `gorm:"not null" json:"rate"`                       // basis points, 0-10000
	PayoutMethod  string    `gorm:"column:payout_method;size:16;not null;default:'wallet'" json:"payout_method"` // wallet / invoice
	MinPayout     int64     `gorm:"column:min_payout;default:0" json:"min_payout"` // threshold before settling
	Active        bool      `gorm:"default:true;not null" json:"active"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (AffiliateCommissionRule) TableName() string { return "affiliate_commission_rules" }
