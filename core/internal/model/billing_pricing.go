// QuotaGate-only model: billing pricing for models and channels (not shared with KexCore IAM)

package model

import "time"

// ModelPricing stores per-model / per-channel token prices.
// Amounts are in the smallest currency unit per 1M tokens.
type ModelPricing struct {
	ID              string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID        string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	ChannelID       string    `gorm:"column:channel_id;size:36;index" json:"channel_id"`
	ModelID         string    `gorm:"column:model_id;size:64;index;not null" json:"model_id"`
	PromptPrice     int64     `gorm:"column:prompt_price;not null;default:0" json:"prompt_price"`         // per 1M prompt tokens
	CompletionPrice int64     `gorm:"column:completion_price;not null;default:0" json:"completion_price"` // per 1M completion tokens
	Currency        string    `gorm:"size:8;not null;default:'CNY'" json:"currency"`
	EffectiveFrom   time.Time `gorm:"column:effective_from;not null;index" json:"effective_from"`
	EffectiveTo     *time.Time `gorm:"column:effective_to;index" json:"effective_to,omitempty"`
	Priority        int       `gorm:"default:0;not null" json:"priority"` // higher wins when multiple rules overlap
	Active          bool      `gorm:"default:true;not null" json:"active"`
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ModelPricing) TableName() string { return "billing_model_pricing" }
