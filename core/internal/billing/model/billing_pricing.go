// QuotaGate-only model: billing pricing for models and channels (not shared with KexCore IAM)

package billingmodel

import "time"

// ModelPricing stores per-model / per-channel token prices and ratio coefficients.
// Two pricing modes are supported:
//   - price mode: PromptPrice / CompletionPrice are absolute amounts per 1M tokens
//   - ratio mode: PromptRatio / CompletionRatio are multipliers against a base unit (QuotaPerUnit)
//
// Mode is selected via BillingMode; both can coexist when migrating from ratio to price.
type ModelPricing struct {
	ID        string `gorm:"primaryKey;size:36" json:"id"`
	TenantID  string `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	VendorID  string `gorm:"column:vendor_id;size:36;index" json:"vendor_id"` // upstream provider id (openai/anthropic/azure/...)
	ChannelID string `gorm:"column:channel_id;size:36;index" json:"channel_id"`
	ModelID   string `gorm:"column:model_id;size:64;index;not null" json:"model_id"`
	// Absolute price mode (per 1M tokens, in smallest currency unit)
	PromptPrice     int64 `gorm:"column:prompt_price;not null;default:0" json:"prompt_price"`
	CompletionPrice int64 `gorm:"column:completion_price;not null;default:0" json:"completion_price"`
	// Ratio mode (multipliers against base unit); only used when BillingMode == "ratio"
	PromptRatio     float64 `gorm:"column:prompt_ratio;not null;default:0" json:"prompt_ratio"`
	CompletionRatio float64 `gorm:"column:completion_ratio;not null;default:0" json:"completion_ratio"`
	// Token-type-specific ratios (nullable pointers: nil means "not supported by this model")
	CacheRatio           *float64 `gorm:"column:cache_ratio" json:"cache_ratio,omitempty"`                       // cache read multiplier vs prompt ratio
	CreateCacheRatio     *float64 `gorm:"column:create_cache_ratio" json:"create_cache_ratio,omitempty"`         // cache write multiplier (e.g. Anthropic 5m cache)
	ImageRatio           *float64 `gorm:"column:image_ratio" json:"image_ratio,omitempty"`                       // image generation multiplier
	AudioRatio           *float64 `gorm:"column:audio_ratio" json:"audio_ratio,omitempty"`                       // audio input multiplier
	AudioCompletionRatio *float64 `gorm:"column:audio_completion_ratio" json:"audio_completion_ratio,omitempty"` // audio output multiplier
	// Billing mode: "price" (absolute) / "ratio" (multiplier) / "tiered_expr" (expression-based)
	BillingMode   string     `gorm:"column:billing_mode;size:16;not null;default:'price';index" json:"billing_mode"`
	BillingExpr   string     `gorm:"column:billing_expr;type:text" json:"billing_expr"` // tiered billing expression (only when BillingMode == "tiered_expr")
	Currency      string     `gorm:"size:8;not null;default:'CNY'" json:"currency"`
	EffectiveFrom time.Time  `gorm:"column:effective_from;not null;index" json:"effective_from"`
	EffectiveTo   *time.Time `gorm:"column:effective_to;index" json:"effective_to,omitempty"`
	Priority      int        `gorm:"default:0;not null" json:"priority"` // higher wins when multiple rules overlap
	Active        bool       `gorm:"default:true;not null" json:"active"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ModelPricing) TableName() string { return "billing_model_pricing" }

func (ModelPricing) TenantAware() bool { return true }

const (
	BillingModePrice      = "price"
	BillingModeRatio      = "ratio"
	BillingModeTieredExpr = "tiered_expr"
)
