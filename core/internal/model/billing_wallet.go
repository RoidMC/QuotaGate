// QuotaGate-only model: billing wallet and balance (not shared with KexCore IAM)

package model

import "time"

// Wallet stores a user's available balance and frozen (pre-consumed) amount.
// Currency is unified as the smallest unit (e.g. cents) to avoid floating-point
// rounding issues.
type Wallet struct {
	ID                  string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID            string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID              string    `gorm:"column:user_id;size:36;uniqueIndex;not null" json:"user_id"`
	Currency            string    `gorm:"size:8;not null;default:'CNY'" json:"currency"`
	Balance             int64     `gorm:"not null;default:0" json:"balance"`                                // available balance in smallest unit
	Frozen              int64     `gorm:"not null;default:0" json:"frozen"`                                 // pre-consumed / locked amount
	FrozenConsumed      int64     `gorm:"column:frozen_consumed;not null;default:0" json:"frozen_consumed"` // frozen amount already settled
	TotalRecharged      int64     `gorm:"column:total_recharged;not null;default:0" json:"total_recharged"`
	TotalConsumed       int64     `gorm:"column:total_consumed;not null;default:0" json:"total_consumed"`
	TotalRefunded       int64     `gorm:"column:total_refunded;not null;default:0" json:"total_refunded"`
	TotalFrozenConsumed int64     `gorm:"column:total_frozen_consumed;not null;default:0" json:"total_frozen_consumed"` // cumulative settled frozen amount
	TotalFrozenReleased int64     `gorm:"column:total_frozen_released;not null;default:0" json:"total_frozen_released"` // cumulative released frozen amount
	TotalAdjusted       int64     `gorm:"column:total_adjusted;not null;default:0" json:"total_adjusted"`               // cumulative manual adjustment amount
	Version             int64     `gorm:"not null;default:0" json:"version"`                                            // optimistic lock version
	Status              string    `gorm:"size:16;not null;default:'active'" json:"status"`
	CreatedAt           time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt           time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Wallet) TableName() string { return "billing_wallets" }

const (
	WalletStatusActive   = "active"
	WalletStatusFrozen   = "frozen"
	WalletStatusDisabled = "disabled"
)
