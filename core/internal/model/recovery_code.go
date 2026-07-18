package model

import "time"

type RecoveryCode struct {
    ID          string     `gorm:"primaryKey;size:36" json:"-"`
    TenantID    string     `gorm:"column:tenant_id;size:36;index:idx_user_unused;index;not null;default:''" json:"-"`
    UserID      string     `gorm:"index:idx_user_unused;size:36;not null" json:"-"`
    BatchID     string     `gorm:"column:batch_id;size:36;index" json:"-"`
    CodeHash    string     `gorm:"size:255;not null" json:"-"`
    Used        bool       `gorm:"index:idx_user_unused;default:false;not null" json:"-"`
    UsedAt      *time.Time `json:"-"`
    MFAMethodID string     `gorm:"column:mfa_method_id;size:36;index" json:"-"` // 可选，绑定方法
    CreatedAt   time.Time  `gorm:"autoCreateTime" json:"-"`
}

func (RecoveryCode) TableName() string {
	return "recovery_codes"
}

func (RecoveryCode) TenantAware() bool { return true }
