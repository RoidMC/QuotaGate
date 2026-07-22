package model

import "time"

// SSOProviderConfig is the per-(tenant, provider) configuration for a
// third-party Identity Provider. One row = one enabled provider for one
// tenant. A tenant enables GitHub login by inserting a row here with the
// GitHub OAuth app's client_id/secret; the SSO plugin factory is looked up
// by Name at request time and instantiated with this config.
//
// This is the SSO counterpart of WebhookConfig: both are tenant-scoped,
// admin-provisioned third-party endpoint configurations. The provider
// implementation itself is compiled into the binary unconditionally; this
// row only decides whether it is enabled for a tenant and with what
// credentials / link mode.
type SSOProviderConfig struct {
	ID           string `gorm:"primaryKey;size:36" json:"id"`
	TenantID     string `gorm:"column:tenant_id;size:36;not null;default:'';uniqueIndex:idx_sso_provider_configs_tenant_name" json:"tenant_id"`
	Name         string `gorm:"size:32;not null;uniqueIndex:idx_sso_provider_configs_tenant_name" json:"name"`
	DisplayName  string `gorm:"column:display_name;size:128" json:"display_name"`
	ClientID     string `gorm:"column:client_id;size:255;not null" json:"client_id"`
	ClientSecret string `gorm:"column:client_secret;size:512;not null;default:''" json:"-"`
	// RedirectURL is optional; when empty, the handler constructs it from the
	// tenant's canonical domain + the standard callback path.
	RedirectURL string `gorm:"column:redirect_url;size:512" json:"redirect_url,omitempty"`
	// LinkMode governs account creation on first SSO login:
	//   "create"    → auto-create a local user and bind the identity
	//   "bind_only" → reject unknown identities; user must bind manually
	LinkMode string `gorm:"column:link_mode;size:16;not null;default:create" json:"link_mode"`
	Scopes   string `gorm:"type:text" json:"scopes,omitempty"` // comma-separated OAuth scopes
	// Extra is provider-specific config JSON (e.g. WeChat appid variant,
	// custom OIDC discovery URL) that doesn't fit the common fields.
	Extra     string    `gorm:"type:text" json:"extra,omitempty"`
	Active    bool      `gorm:"not null;default:true" json:"active"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SSOProviderConfig) TableName() string {
	return "sso_provider_configs"
}

func (SSOProviderConfig) TenantAware() bool { return true }
