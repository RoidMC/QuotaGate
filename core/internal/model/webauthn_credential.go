package model

import (
	"errors"
	"time"
)

var ErrCloneWarning = errors.New("quotagate/model: potential credential cloning detected (sign count has not increased)")

type WebAuthnCredential struct {
	ID string `gorm:"primaryKey;size:36" json:"id"`
	// TenantID is the leading column of BOTH composite indexes below — GORM merges
	// tags with the same index name across fields, so we declare priority:1 here
	// for each composite index and the trailing column on the partner field.
	TenantID string `gorm:"column:tenant_id;size:36;not null;default:'';index:idx_webauthn_creds_tenant_user,priority:1;index:idx_webauthn_creds_tenant_cred,priority:1" json:"tenant_id"`
	// idx_webauthn_creds_tenant_user serves the per-user lookups (ListByUserID,
	// DeleteByUserID). Composite because every tenant-scoped query filters on both
	// columns — the previous single-column indexes forced Postgres into bitmap-AND
	// scans at non-trivial scale.
	UserID string `gorm:"size:36;not null;index:idx_webauthn_creds_tenant_user,priority:2" json:"user_id"`
	// idx_webauthn_creds_tenant_cred is the unique constraint. The previous
	// global uniqueIndex broke multi-tenant deployments sharing one RPID: two
	// tenants registering the same hardware key would collide on INSERT even
	// though their (tenant_id, credential_id) pairs are distinct. Composite
	// unique matches how FindByCredentialID actually queries — it always goes
	// through the tenant callback, so tenant_id is part of the lookup key.
	CredentialID    []byte     `gorm:"type:bytea;not null;uniqueIndex:idx_webauthn_creds_tenant_cred,priority:2" json:"-"`
	PublicKey       []byte     `gorm:"type:bytea;not null" json:"-"`
	AttestationType string     `gorm:"size:64;not null" json:"attestation_type"`
	AAGUID          []byte     `gorm:"type:bytea" json:"-"`
	SignCount       uint32     `gorm:"default:0;not null" json:"sign_count"`
	Name            string     `gorm:"size:128" json:"name"`
	Transports      string     `gorm:"size:255" json:"transports"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	LastUsedAt      *time.Time `json:"last_used_at"`

	// Authenticator flags (from FIDO2 webauthn library)
	UserPresent    bool   `json:"user_present"`              // clientExtensionResults: userPresent flag
	UserVerified   bool   `json:"user_verified"`             // clientExtensionResults: userVerified flag
	BackupEligible bool   `json:"backup_eligible"`           // authenticator flags
	BackupState    bool   `json:"backup_state"`              // authenticator flags
	Attachment     string `gorm:"size:32" json:"attachment"` // "platform" or "cross-platform"
}

func (WebAuthnCredential) TableName() string {
	return "webauthn_credentials"
}

func (WebAuthnCredential) TenantAware() bool { return true }
