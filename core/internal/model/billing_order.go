// QuotaGate-only model: billing order (not shared with KexCore IAM)

package model

import "time"

// Order groups one or more billing operations into a single business unit.
// For API gateway usage, an order typically maps to one upstream request or
// one chat completion call that may involve multiple model/provider attempts.
type Order struct {
	ID                string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID          string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID            string     `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	RequestID         string     `gorm:"column:request_id;size:64;uniqueIndex;not null" json:"request_id"`
	UpstreamRequestID string     `gorm:"column:upstream_request_id;size:128;index" json:"upstream_request_id"` // upstream provider's request id for reconciliation
	WALID             *int64     `gorm:"column:wal_id;index" json:"wal_id,omitempty"`
	TokenID           string     `gorm:"column:token_id;size:36;index" json:"token_id"` // API token used for this request
	ChannelID         string     `gorm:"column:channel_id;size:36;index" json:"channel_id"`
	ChannelProvider   string     `gorm:"column:channel_provider;size:32;index" json:"channel_provider"`         // upstream provider name: openai/anthropic/azure/...
	ModelID           string     `gorm:"column:model_id;size:64;index" json:"model_id"`                         // actual model routed to upstream
	OriginalModelID   string     `gorm:"column:original_model_id;size:64;index" json:"original_model_id"`       // model name requested by user (before mapping)
	APIType           string     `gorm:"column:api_type;size:32;not null;default:'chat';index" json:"api_type"` // chat/embedding/image/audio/rerank/responses/realtime
	GroupName         string     `gorm:"column:group_name;size:64;index" json:"group_name"`                     // user group for group-based pricing
	PromptTokens      int64      `gorm:"column:prompt_tokens;not null;default:0" json:"prompt_tokens"`
	CompletionTokens  int64      `gorm:"column:completion_tokens;not null;default:0" json:"completion_tokens"`
	CachedTokens      int64      `gorm:"column:cached_tokens;not null;default:0" json:"cached_tokens"` // prompt cache hit tokens (priced differently)
	TotalTokens       int64      `gorm:"column:total_tokens;not null;default:0" json:"total_tokens"`
	Amount            int64      `gorm:"not null;default:0" json:"amount"` // final billed amount in smallest unit
	Currency          string     `gorm:"size:8;not null;default:'CNY'" json:"currency"`
	IsStream          bool       `gorm:"column:is_stream;not null;default:false" json:"is_stream"`
	UseTimeMs         int64      `gorm:"column:use_time_ms;not null;default:0" json:"use_time_ms"` // end-to-end latency in milliseconds
	ClientIP          string     `gorm:"column:client_ip;size:64;index" json:"client_ip"`
	FinishReason      string     `gorm:"column:finish_reason;size:32" json:"finish_reason"` // stop/length/content_filter/tool_calls/error
	Status            string     `gorm:"size:16;not null;default:'pending';index" json:"status"`
	SettledAt         *time.Time `gorm:"column:settled_at" json:"settled_at,omitempty"`
	ErrorMessage      string     `gorm:"column:error_message;type:text" json:"error_message"`
	CreatedAt         time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt         time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Order) TableName() string { return "billing_orders" }

const (
	OrderStatusPending   = "pending"
	OrderStatusSettled   = "settled"
	OrderStatusRefunded  = "refunded"
	OrderStatusFailed    = "failed"
	OrderStatusCancelled = "cancelled"

	APITypeChat       = "chat"
	APITypeEmbedding  = "embedding"
	APITypeImage      = "image"
	APITypeAudio      = "audio"
	APITypeRerank     = "rerank"
	APITypeResponses  = "responses"
	APITypeRealtime   = "realtime"
	APITypeModeration = "moderation"
)
