// Package model defines catalog metadata and sync state for QuotaGate.
//
// These models are intentionally isolated from internal/model. They are not
// shared with IAM, billing, or gateway runtime; they belong to the catalog
// service's own database instance and must not be migrated into the shared
// schema contract.
package model

import "time"

// ModelMeta stores human-readable metadata for a model name that the gateway
// or UI may expose. It is the persisted form of new-api's `models` table minus
// gateway-runtime fields such as status/sync_official; instead, sync status is
// tracked via ModelSyncState.
type ModelMeta struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	ModelName   string    `gorm:"column:model_name;size:128;not null;uniqueIndex" json:"model_name"`
	Description string    `gorm:"type:text" json:"description"`
	Icon        string    `gorm:"size:128" json:"icon"`
	Tags        string    `gorm:"size:255" json:"tags"`
	VendorID    string    `gorm:"column:vendor_id;size:36;index" json:"vendor_id"`
	Endpoints   string    `gorm:"type:text" json:"endpoints"` // JSON map of endpoint names -> paths/methods
	Enabled     bool      `gorm:"default:true;not null" json:"enabled"`
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (ModelMeta) TableName() string { return "sync_model_meta" }
