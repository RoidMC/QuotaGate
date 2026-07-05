package model

import (
	"errors"
	"time"
)

var ErrCloneWarning = errors.New("quotagate/model: potential credential cloning detected (sign count has not increased)")

type WebAuthnCredential struct {
	ID              string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID        string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID          string     `gorm:"index;size:36;not null" json:"user_id"`
	CredentialID    []byte     `gorm:"uniqueIndex;type:blob;not null" json:"-"`
	PublicKey       []byte     `gorm:"type:blob;not null" json:"-"`
	AttestationType string     `gorm:"size:64;not null" json:"attestation_type"`
	AAGUID          []byte     `gorm:"type:blob" json:"-"`
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
