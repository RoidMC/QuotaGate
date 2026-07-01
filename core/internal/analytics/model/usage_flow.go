// Package model defines analytics aggregation tables for QuotaGate.
//
// These models are intentionally isolated from internal/model. They are not
// shared with IAM, billing, or gateway runtime; they belong to the analytics
// service's own database instance and must not be migrated into the shared
// schema contract.
package model

import "time"

// UsageFlow is a pre-aggregated time-series row for dashboards and trend
// queries. It is derived from QuotaData and refreshed by a background worker.
type UsageFlow struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID      string    `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	TokenID     string    `gorm:"column:token_id;size:36;index;not null" json:"token_id"`
	UseGroup    string    `gorm:"column:use_group;size:64;index" json:"use_group"`
	ModelName   string    `gorm:"column:model_name;size:128;index;not null" json:"model_name"`
	ChannelID   string    `gorm:"column:channel_id;size:36;index" json:"channel_id"`
	NodeName    string    `gorm:"column:node_name;size:128;index" json:"node_name"`
	PeriodStart time.Time `gorm:"column:period_start;not null;index" json:"period_start"` // bucket boundary, e.g. hour start
	PeriodEnd   time.Time `gorm:"column:period_end;not null;index" json:"period_end"`
	Quota       int64     `gorm:"not null;default:0" json:"quota"`
	TokenUsed   int64     `gorm:"column:token_used;not null;default:0" json:"token_used"`
	Count       int64     `gorm:"not null;default:0" json:"count"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UsageFlow) TableName() string { return "analytics_usage_flow" }
