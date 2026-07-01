package model

import "time"

type Session struct {
	ID           string     `gorm:"primaryKey;size:36" json:"id"`
	UserID       string     `gorm:"index;size:36;not null" json:"user_id"`
	TokenHash    string     `gorm:"column:token_hash;size:255;not null" json:"-"`
	DeviceName   string     `gorm:"column:device_name;size:128" json:"device_name"`
	DeviceType   string     `gorm:"column:device_type;size:20" json:"device_type"`
	IP           string     `gorm:"size:45" json:"ip"`
	UserAgent    string     `gorm:"column:user_agent;size:512" json:"user_agent"`
	Region       string     `gorm:"size:128" json:"region"`
	ExpiresAt    time.Time  `gorm:"column:expires_at;not null" json:"expires_at"`
	LastActiveAt *time.Time `gorm:"column:last_active_at" json:"last_active_at"`
	CreatedAt    time.Time  `gorm:"autoCreateTime" json:"created_at"`
}

func (Session) TableName() string {
	return "sessions"
}
