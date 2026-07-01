// Package model defines analytics aggregation tables for QuotaGate.
//
// These models are intentionally isolated from internal/model. They are not
// shared with IAM, billing, or gateway runtime; they belong to the analytics
// service's own database instance and must not be migrated into the shared
// schema contract.
package model

import "time"

// QuotaData is an append-only usage fact row emitted from the gateway for
// billing/analytics pipelines. It corresponds to new-api's `quota_data` table
// and is the source of truth for usage-flow aggregation.
type QuotaData struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID   string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID     string    `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	TokenID    string    `gorm:"column:token_id;size:36;index;not null" json:"token_id"`
	UseGroup   string    `gorm:"column:use_group;size:64;index" json:"use_group"`
	ModelName  string    `gorm:"column:model_name;size:128;index;not null" json:"model_name"`
	ChannelID  string    `gorm:"column:channel_id;size:36;index" json:"channel_id"`
	NodeName   string    `gorm:"column:node_name;size:128;index" json:"node_name"`
	Quota      int64     `gorm:"not null;default:0" json:"quota"`            // billed quota amount
	TokenUsed  int64     `gorm:"column:token_used;not null;default:0" json:"token_used"` // total prompt+completion tokens
	Count      int64     `gorm:"not null;default:0" json:"count"`            // request count for aggregation
	CreatedAt  time.Time `gorm:"autoCreateTime;index" json:"created_at"`
}

func (QuotaData) TableName() string { return "analytics_quota_data" }
