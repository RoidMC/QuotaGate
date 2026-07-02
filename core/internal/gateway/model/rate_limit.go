// Package model defines gateway rate-limit and quota enforcement tables for
// QuotaGate.
//
// These models are intentionally isolated from internal/model. They belong to
// the gateway service's own database instance and must not be migrated into
// the shared schema contract.
package model

import "time"

// QuotaLimitKind distinguishes rate limits (RPM/TPM) from quota limits
// (total tokens / total requests).
const (
	QuotaLimitKindRPM         = "rpm"
	QuotaLimitKindTPM         = "tpm"
	QuotaLimitKindConcurrent  = "concurrent"
	QuotaLimitKindTotalQuota  = "total_quota"
	QuotaLimitKindDailyQuota  = "daily_quota"
)

// RateLimitQuota stores persistent rate-limit and quota rules evaluated by
// the gateway middleware. It is the DB-backed source of truth; Redis is used
// as a hot cache for counters only.
type RateLimitQuota struct {
	ID            string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID      string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	ScopeType     string    `gorm:"column:scope_type;size:16;not null;default:'user';index" json:"scope_type"` // user / group / token / global
	ScopeID       string    `gorm:"column:scope_id;size:36;not null;index" json:"scope_id"`                   // user_id / group_name / token_id / '*'
	ModelID       string    `gorm:"column:model_id;size:128;index" json:"model_id"`                           // empty means applies to all models
	LimitKind     string    `gorm:"column:limit_kind;size:16;not null;index" json:"limit_kind"`               // rpm / tpm / concurrent / total_quota / daily_quota
	LimitValue    int64     `gorm:"column:limit_value;not null" json:"limit_value"`                           // e.g. 100 RPM
	WindowSeconds int64     `gorm:"column:window_seconds;default:0;not null" json:"window_seconds"`           // 0 for sliding-window; >0 for fixed window
	Priority      int64     `gorm:"default:0;not null;index" json:"priority"`                                 // higher = evaluated first
	Enabled       bool      `gorm:"default:true;not null" json:"enabled"`
	ErrorMessage  string    `gorm:"column:error_message;size:255" json:"error_message"`                       // custom rejection message
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (RateLimitQuota) TableName() string { return "gateway_rate_limit_quotas" }

// RateLimitSnapshot is an append-only counter snapshot for debugging and
// reconciliation. It is written periodically by the gateway so operators can
// inspect actual counters vs configured limits.
type RateLimitSnapshot struct {
	ID            string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID      string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	QuotaID       string    `gorm:"column:quota_id;size:36;index;not null" json:"quota_id"`
	ScopeType     string    `gorm:"column:scope_type;size:16;not null;index" json:"scope_type"`
	ScopeID       string    `gorm:"column:scope_id;size:36;not null;index" json:"scope_id"`
	ModelID       string    `gorm:"column:model_id;size:128;index" json:"model_id"`
	LimitKind     string    `gorm:"column:limit_kind;size:16;not null;index" json:"limit_kind"`
	WindowStart   time.Time `gorm:"column:window_start;not null;index" json:"window_start"`
	WindowEnd     time.Time `gorm:"column:window_end;not null;index" json:"window_end"`
	CurrentValue  int64     `gorm:"column:current_value;not null;default:0" json:"current_value"`
	LimitValue    int64     `gorm:"column:limit_value;not null" json:"limit_value"`
	HitCount      int64     `gorm:"column:hit_count;not null;default:0" json:"hit_count"` // times the limit was enforced
	CreatedAt     time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (RateLimitSnapshot) TableName() string { return "gateway_rate_limit_snapshots" }
