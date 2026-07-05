package model

import "time"

type MFAMethod struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID  string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID    string    `gorm:"uniqueIndex:idx_user_method;size:36;not null" json:"user_id"`
	Method    string    `gorm:"uniqueIndex:idx_user_method;size:32;not null" json:"method"`
	Secret    string    `gorm:"size:255" json:"-"`
	Enabled   bool      `gorm:"default:false;not null" json:"enabled"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (MFAMethod) TableName() string {
	return "mfa_methods"
}
