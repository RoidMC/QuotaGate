// Package model defines gateway routing and channel metadata for QuotaGate.
//
// These models are intentionally isolated from internal/model. They are not
// shared with IAM or billing; they belong to the API gateway's own database
// instance and must not be migrated into the shared schema contract.
package model

import "time"

// RouteRule allows model name routing by prefix/suffix/contains/regex without
// enumerating every alias in RouteAbility. It is evaluated before the exact
// ability lookup so operators can define catch-all rules.
type RouteRule struct {
	ID         string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID   string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	NameRule   int       `gorm:"not null;default:0;index" json:"name_rule"` // exact=0, prefix=1, contains=2, suffix=3, regex=4
	ModelName  string    `gorm:"column:model_name;size:128;not null;index" json:"model_name"`
	TargetModel string   `gorm:"column:target_model;size:128;not null" json:"target_model"` // mapped model name
	Enabled    bool      `gorm:"default:true;not null" json:"enabled"`
	Priority   int       `gorm:"default:0;not null;index" json:"priority"` // higher = applied first
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt  time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (RouteRule) TableName() string { return "gateway_route_rules" }
