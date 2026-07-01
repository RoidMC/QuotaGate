// Package model defines gateway routing and channel metadata for QuotaGate.
//
// These models are intentionally isolated from internal/model. They are not
// shared with IAM or billing; they belong to the API gateway's own database
// instance and must not be migrated into the shared schema contract.
package model

import "time"

// Channel represents an upstream provider endpoint and its credentials/config.
// It is intentionally lightweight; secrets should be injected at runtime via
// secret manager rather than persisted here in production.
type Channel struct {
	ID              string  `gorm:"primaryKey;size:36" json:"id"`
	TenantID        string  `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	Name            string  `gorm:"size:128;not null" json:"name"`
	Type            int     `gorm:"not null;index" json:"type"` // provider / channel type enum
	Status          string  `gorm:"size:16;not null;default:'enabled';index" json:"status"`
	BaseURL         string  `gorm:"column:base_url;size:512" json:"base_url"`
	Key             string  `gorm:"size:255" json:"-"` // redacted from JSON responses
	Organization    string  `gorm:"size:128" json:"organization"`
	Models          string  `gorm:"type:text" json:"models"` // comma-separated model names supported
	Group           string  `gorm:"size:128;index" json:"group"` // load-balance group
	Priority        int64   `gorm:"default:0;not null;index" json:"priority"`
	Weight          int     `gorm:"default:0;not null" json:"weight"`
	Tag             string  `gorm:"size:64;index" json:"tag"`
	TestModel       string  `gorm:"column:test_model;size:128" json:"test_model"`
	ParamOverride   string  `gorm:"column:param_override;type:text" json:"param_override"` // JSON
	HeadersOverride string  `gorm:"column:headers_override;type:text" json:"headers_override"` // JSON
	OtherSettings   string  `gorm:"column:other_settings;type:text" json:"other_settings"` // JSON
	CreatedAt       time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Channel) TableName() string { return "gateway_channels" }
