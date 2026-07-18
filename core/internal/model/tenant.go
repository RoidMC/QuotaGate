package model

import "time"

type Tenant struct {
	ID        string    `gorm:"primaryKey;size:36" json:"id"`
	Name      string    `gorm:"size:128;not null" json:"name"`
	Slug      string    `gorm:"uniqueIndex;size:64;not null" json:"slug"`
	Settings  string    `gorm:"type:text" json:"settings"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}
 
func (Tenant) TableName() string { return "tenants" }

func (Tenant) TenantAware() bool { return true }
