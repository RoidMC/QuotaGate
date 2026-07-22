package model

import "time"

// TenantMembership links a single user identity to one or more tenants. It is
// the bridge that lets one globally-unique user row belong to multiple
// organizations — the model Casdoor issue #4156 asks for and that Keycloak
// provides via its Organization feature.
//
// IMPORTANT: TenantMembership deliberately does NOT implement TenantAware().
// It is a cross-tenant bridge: login must enumerate a user's memberships
// regardless of which tenant the request arrived on, so the tenant GORM
// callback must not append `tenant_id = ?` to its queries. All access goes
// through an explicit tenant context or tenant.Bypass.
type TenantMembership struct {
	ID       string    `gorm:"primaryKey;size:36" json:"id"`
	UserID   string    `gorm:"column:user_id;size:36;not null;uniqueIndex:idx_membership_user_tenant" json:"user_id"`
	TenantID string    `gorm:"column:tenant_id;size:36;not null;uniqueIndex:idx_membership_user_tenant" json:"tenant_id"`
	// Roles is a comma-separated list of roles valid within this specific
	// tenant. Stored per-membership so the same user can hold different roles
	// in different organizations.
	Roles     string    `gorm:"type:text" json:"roles"`
	Status    string    `gorm:"size:20;default:active;not null" json:"status"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
}

func (TenantMembership) TableName() string { return "tenant_memberships" }

// IsActive reports whether the membership is in the active state.
func (m TenantMembership) IsActive() bool { return m.Status == "active" || m.Status == "" }
