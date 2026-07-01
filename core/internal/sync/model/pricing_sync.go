// Package model defines catalog metadata and sync state for QuotaGate.
//
// These models are intentionally isolated from internal/model. They are not
// shared with IAM, billing, or gateway runtime; they belong to the catalog
// service's own database instance and must not be migrated into the shared
// schema contract.
package model

import "time"

// PricingSyncState records the last known sync state for a model's pricing
// source. It is used to detect drift, schedule refreshes, and avoid redundant
// upstream calls.
type PricingSyncState struct {
	ID           string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID     string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	ModelName    string    `gorm:"column:model_name;size:128;not null;uniqueIndex" json:"model_name"`
	Source       string    `gorm:"size:64;not null" json:"source"` // official / vendor / manual
	Version      string    `gorm:"size:64" json:"version"`          // upstream version or ETag/commit hash
	LastSyncAt   time.Time `gorm:"column:last_sync_at;index" json:"last_sync_at"`
	NextSyncAt   time.Time `gorm:"column:next_sync_at;index" json:"next_sync_at"`
	LastSyncStatus string `gorm:"column:last_sync_status;size:16;not null;default:'success'" json:"last_sync_status"` // success / failed / skipped
	LastSyncError  string `gorm:"column:last_sync_error;type:text" json:"last_sync_error"`
	CreatedAt    time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (PricingSyncState) TableName() string { return "sync_pricing_state" }
