package model

import "time"

type RecoveryCode struct {
	ID        string    `gorm:"primaryKey;size:36" json:"-"`
	UserID    string    `gorm:"index;size:36;not null" json:"-"`
	CodeHash  string    `gorm:"size:255;not null" json:"-"`
	Used      bool      `gorm:"default:false;not null" json:"-"`
	UsedAt    *time.Time `json:"-"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"-"`
}

func (RecoveryCode) TableName() string {
	return "recovery_codes"
}