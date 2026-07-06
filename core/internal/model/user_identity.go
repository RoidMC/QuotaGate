package model

import (
	"time"
)

type UserIdentity struct {
	ID          string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID    string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID      string     `gorm:"uniqueIndex:idx_provider_subject;size:36;not null" json:"user_id"`
	Provider    string     `gorm:"uniqueIndex:idx_provider_subject;size:32;not null" json:"provider"`
	Subject     string     `gorm:"uniqueIndex:idx_provider_subject;size:255;not null" json:"subject"`
	Email       string     `gorm:"size:255" json:"email"`
	DisplayName string     `gorm:"size:128" json:"display_name"`
	AvatarURL   string     `gorm:"size:512" json:"avatar_url"`
	RawData     string     `gorm:"type:text" json:"-"`
	LastUsedAt  *time.Time `json:"last_used_at"`
	CreatedAt   time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (UserIdentity) TableName() string {
	return "user_identities"
}

func (UserIdentity) TenantAware() bool { return true }
