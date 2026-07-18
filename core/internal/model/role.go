// QuotaGate-only model: role definitions, user-role assignments, mutual exclusions and route metadata (not shared with KexCore IAM)

package model

import "time"

// RoleDefinition defines a named role and its inheritance chain.
// System roles have an empty TenantID; tenant-scoped roles belong to one tenant.
type RoleDefinition struct {
	ID             string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID       string    `gorm:"column:tenant_id;size:36;not null;default:'';uniqueIndex:idx_role_definitions_tenant_name" json:"tenant_id"`
	Name           string    `gorm:"size:64;not null;uniqueIndex:idx_role_definitions_tenant_name" json:"name"`
	Description    string    `gorm:"size:256" json:"description"`
	IsSystem       bool      `gorm:"column:is_system;default:false;not null" json:"is_system"`
	InheritedRoles []string  `gorm:"column:inherited_roles;serializer:json" json:"inherited_roles"`
	CreatedAt      time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (RoleDefinition) TableName() string { return "role_definitions" }

func (RoleDefinition) TenantAware() bool { return true }

// UserRoleAssignment links a user to a role within a tenant.
// The role name is stored without tenant encoding; the role is resolved
// against the user's tenant plus system roles.
type UserRoleAssignment struct {
	ID       string `gorm:"primaryKey;size:36" json:"id"`
	TenantID string `gorm:"column:tenant_id;size:36;not null;default:'';uniqueIndex:idx_user_role_assignments_tenant_user_role" json:"tenant_id"`
	UserID   string `gorm:"column:user_id;size:36;not null;uniqueIndex:idx_user_role_assignments_tenant_user_role" json:"user_id"`
	Role     string `gorm:"size:64;not null;uniqueIndex:idx_user_role_assignments_tenant_user_role" json:"role"`
}

func (UserRoleAssignment) TableName() string { return "user_role_assignments" }

func (UserRoleAssignment) TenantAware() bool { return true }

// RoleMutualExclusion declares that two roles cannot be assigned to the same user.
// RoleA and RoleB are stored normalized so that RoleA < RoleB lexicographically.
type RoleMutualExclusion struct {
	ID       string `gorm:"primaryKey;size:36" json:"id"`
	TenantID string `gorm:"column:tenant_id;size:36;not null;default:'';uniqueIndex:idx_role_mutual_exclusions_tenant_roles" json:"tenant_id"`
	RoleA    string `gorm:"column:role_a;size:64;not null;uniqueIndex:idx_role_mutual_exclusions_tenant_roles" json:"role_a"`
	RoleB    string `gorm:"column:role_b;size:64;not null;uniqueIndex:idx_role_mutual_exclusions_tenant_roles" json:"role_b"`
}

func (RoleMutualExclusion) TableName() string { return "role_mutual_exclusions" }

func (RoleMutualExclusion) TenantAware() bool { return true }

// RouteMeta stores the authorization rule type for a route.
// It is used by the authz middleware to decide whether a route requires
// RBAC only, an additional ABAC check, or service-level ReBAC validation.
type RouteMeta struct {
	ID       string `gorm:"primaryKey;size:36" json:"id"`
	TenantID string `gorm:"column:tenant_id;size:36;not null;default:'';uniqueIndex:idx_route_metas_tenant_method_path" json:"tenant_id"`
	Method   string `gorm:"size:10;not null;uniqueIndex:idx_route_metas_tenant_method_path" json:"method"`
	Path     string `gorm:"size:512;not null;uniqueIndex:idx_route_metas_tenant_method_path" json:"path"`
	RuleType string `gorm:"column:rule_type;size:16;not null" json:"rule_type"`
}

func (RouteMeta) TableName() string { return "route_metas" }

func (RouteMeta) TenantAware() bool { return true }

const (
	RuleTypeRBAC  = "rbac"
	RuleTypeABAC  = "abac"
	RuleTypeReBAC = "rebac"
)
