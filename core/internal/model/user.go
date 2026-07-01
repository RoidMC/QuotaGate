package model

import (
	"time"
)

type User struct {
	ID                 string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID           string     `gorm:"column:tenant_id;size:36;not null;default:''" json:"tenant_id"`
	Email              string     `gorm:"uniqueIndex;size:255" json:"email"`
	EmailVerified      bool       `gorm:"column:email_verified;default:false;not null" json:"email_verified"`
	Phone              string     `gorm:"uniqueIndex:idx_users_phone;size:20;default:''" json:"phone"`
	PhoneVerified      bool       `gorm:"column:phone_verified;default:false;not null" json:"phone_verified"`
	Username           string     `gorm:"uniqueIndex;size:64;not null" json:"username"`
	PasswordHash       string     `gorm:"column:password_hash;size:255" json:"-"`
	DisplayName        string     `gorm:"size:128" json:"display_name"`
	AvatarURL          string     `gorm:"size:512" json:"avatar_url"`
	RegistrationMethod string     `gorm:"column:registration_method;size:32;default:password;not null" json:"registration_method"`
	Role               UserRole   `gorm:"size:20;default:user;not null" json:"role"`
	Status             UserStatus `gorm:"size:20;default:active;not null" json:"status"`
	Metadata           string     `gorm:"type:text" json:"metadata"`
	LastLoginIP        string     `gorm:"column:last_login_ip;size:45" json:"last_login_ip"`
	LastLoginAt        *time.Time `gorm:"column:last_login_at" json:"last_login_at"`
	RegisterIP         string     `gorm:"column:register_ip;size:45" json:"register_ip"`
	CreatedAt          time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt          time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (User) TableName() string {
	return "users"
}

type UserRole string

const (
	RoleAdmin UserRole = "admin"
	RoleUser  UserRole = "user"
)

func (r UserRole) IsAdmin() bool {
	return r == RoleAdmin
}

type UserStatus string

const (
	UserStatusActive    UserStatus = "active"
	UserStatusInactive  UserStatus = "inactive"
	UserStatusLocked    UserStatus = "locked"
	UserStatusDisabled  UserStatus = "disabled"
	UserStatusSuspended UserStatus = "suspended"
	// quotagate uses hard delete only — no soft delete.
	// Before deletion, personal data (email, username, phone) is
	// SM3-hashed into the audit log for compliance, then the record
	// is physically removed. This satisfies GDPR "right to erasure"
	// and frees unique constraints for reuse.
	UserStatusPendingDelete UserStatus = "pending_delete"
)

func (s UserStatus) IsValid() bool {
	switch s {
	case UserStatusActive, UserStatusInactive, UserStatusLocked, UserStatusDisabled, UserStatusSuspended, UserStatusPendingDelete:
		return true
	}
	return false
}
