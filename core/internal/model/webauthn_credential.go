package model

import (
	"time"
)

type WebAuthnCredential struct {
	ID              string     `gorm:"primaryKey;size:36" json:"id"`
	UserID          string     `gorm:"index;size:36;not null" json:"user_id"`
	CredentialID    []byte     `gorm:"uniqueIndex;type:bytea;not null" json:"-"`
	PublicKey       []byte     `gorm:"type:bytea;not null" json:"-"`
	AttestationType string     `gorm:"size:64;not null" json:"attestation_type"`
	AAGUID          []byte     `gorm:"type:bytea" json:"-"`
	SignCount       uint32     `gorm:"default:0;not null" json:"sign_count"`
	Name            string     `gorm:"size:128" json:"name"`
	Transports      string     `gorm:"size:255" json:"transports"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"created_at"`
	LastUsedAt      *time.Time `json:"last_used_at"`
}

func (WebAuthnCredential) TableName() string {
	return "webauthn_credentials"
}
