// Package model defines gateway routing and channel metadata for QuotaGate.
//
// These models are intentionally isolated from internal/model. They are not
// shared with IAM or billing; they belong to the API gateway's own database
// instance and must not be migrated into the shared schema contract.
package model

import "time"

// RouteAbility binds a model name (or pattern) to a concrete upstream channel
// for a specific user group. It is the gateway's equivalent of new-api's
// `abilities` table, with the addition of a tenant scope for multi-tenancy.
type RouteAbility struct {
	ID         string  `gorm:"primaryKey;size:36" json:"id"`
	TenantID   string  `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	Group      string  `gorm:"column:group_name;size:64;not null;index" json:"group_name"`
	Model      string  `gorm:"column:model_name;size:128;not null;index" json:"model_name"`
	ChannelID  string  `gorm:"column:channel_id;size:36;not null;index" json:"channel_id"`
	Enabled    bool    `gorm:"default:true;not null" json:"enabled"`
	Priority   int64   `gorm:"default:0;not null;index" json:"priority"` // higher = preferred
	Weight     uint    `gorm:"default:0;not null;index" json:"weight"`     // load-balance weight
	Tag        string  `gorm:"size:64;index" json:"tag"`                   // optional grouping tag for灰度
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (RouteAbility) TableName() string { return "gateway_route_abilities" }
