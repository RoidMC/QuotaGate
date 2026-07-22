package model

import "time"

// StorageObject records metadata for a file stored via a storage.Provider.
type StorageObject struct {
	ID                string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID          string    `gorm:"column:tenant_id;size:36;not null;index;default:''" json:"tenant_id"`
	StorageProviderID string    `gorm:"column:storage_provider_id;size:36;not null;index" json:"storage_provider_id"`
	Key               string    `gorm:"size:512;not null" json:"key"`
	OriginalName      string    `gorm:"column:original_name;size:255" json:"original_name"`
	ContentType       string    `gorm:"column:content_type;size:100" json:"content_type"`
	Size              int64     `json:"size"`
	Checksum          string    `gorm:"size:64" json:"checksum"`
	URL               string    `gorm:"size:1024" json:"url"`
	Tag               string    `gorm:"size:64;index" json:"tag"`
	UploaderID        string    `gorm:"column:uploader_id;size:36;index" json:"uploader_id"`
	AppID             string    `gorm:"column:app_id;size:36;index" json:"app_id"`
	CreatedAt         time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (StorageObject) TableName() string { return "storage_objects" }

func (StorageObject) TenantAware() bool { return true }
