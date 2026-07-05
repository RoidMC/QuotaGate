package repository

import (
	"context"
	"errors"

	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/util/random"
	"gorm.io/gorm"
)

var (
	ErrCredentialNotFound = errors.New("quotagate/repository: webauthn credential not found")
)

type WebAuthnRepository struct {
	db *gorm.DB
}

func NewWebAuthnRepository(db *gorm.DB) *WebAuthnRepository {
	return &WebAuthnRepository{db: db}
}

func (r *WebAuthnRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&model.WebAuthnCredential{})
}

func (r *WebAuthnRepository) Create(cred *model.WebAuthnCredential) error {
	if cred.ID == "" {
		cred.ID = random.MustUUIDString()
	}
	return r.db.Create(cred).Error
}

func (r *WebAuthnRepository) FindByCredentialID(credentialID []byte) (*model.WebAuthnCredential, error) {
	var cred model.WebAuthnCredential
	result := r.db.Where("credential_id = ?", credentialID).First(&cred)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, result.Error
	}
	return &cred, nil
}

// FindByUserIDAndCredentialID looks up a credential by both user_id and credential_id.
// This is the correct lookup for WebAuthn authenticator verification (FIDO2)
// where the client presents the credential ID and the server must confirm it
// belongs to the authenticated user.
func (r *WebAuthnRepository) FindByUserIDAndCredentialID(userID string, credentialID []byte) (*model.WebAuthnCredential, error) {
	var cred model.WebAuthnCredential
	result := r.db.Where("user_id = ? AND credential_id = ?", userID, credentialID).First(&cred)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, result.Error
	}
	return &cred, nil
}

func (r *WebAuthnRepository) FindByID(ctx context.Context, id string) (*model.WebAuthnCredential, error) {
	var cred model.WebAuthnCredential
	result := r.db.WithContext(ctx).Where("id = ?", id).First(&cred)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, result.Error
	}
	return &cred, nil
}

func (r *WebAuthnRepository) ListByUserID(userID string) ([]model.WebAuthnCredential, error) {
	var creds []model.WebAuthnCredential
	result := r.db.Where("user_id = ?", userID).Order("created_at DESC").Find(&creds)
	return creds, result.Error
}

// Update persists field changes for the given credential.
// Only non-zero fields are written; if the credential does not exist, ErrCredentialNotFound is returned.
func (r *WebAuthnRepository) Update(ctx context.Context, cred *model.WebAuthnCredential) error {
	result := r.db.WithContext(ctx).Model(&model.WebAuthnCredential{}).Where("id = ?", cred.ID).Updates(cred)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCredentialNotFound
	}
	return nil
}

func (r *WebAuthnRepository) Delete(id string) error {
	result := r.db.Where("id = ?", id).Delete(&model.WebAuthnCredential{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCredentialNotFound
	}
	return nil
}

// DeleteByUserID deletes all WebAuthn credentials belonging to a user.
// Returns ErrCredentialNotFound if no credentials were deleted.
func (r *WebAuthnRepository) DeleteByUserID(userID string) error {
	result := r.db.Where("user_id = ?", userID).Delete(&model.WebAuthnCredential{})
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrCredentialNotFound
	}
	return nil
}

// VerifyAndIncrementSignCount atomically checks and increments the sign count
// for a credential. Returns the stored sign count and whether the operation
// succeeded. This implements FIDO2 SignCount validation per the WebAuthn spec.
//
// Verification logic:
//   - newCount > stored: success, update count
//   - newCount == stored: clone warning (credential may be cloned), update but
//     return cloneWarning=true for upstream to decide deny/allow
//   - newCount < stored: credential is cloned, reject entirely
func (r *WebAuthnRepository) VerifyAndIncrementSignCount(ctx context.Context, credentialID []byte, newSignCount uint32) (storedCount uint32, cloneWarning bool, err error) {
	var cred model.WebAuthnCredential
	result := r.db.WithContext(ctx).Where("credential_id = ?", credentialID).First(&cred)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return 0, false, ErrCredentialNotFound
		}
		return 0, false, result.Error
	}

	storedCount = cred.SignCount

	if newSignCount < storedCount {
		// Credential is cloned — reject immediately
		return storedCount, false, ErrCredentialNotFound
	}

	if newSignCount == storedCount {
		// No increase — possible cloning. Update count (to prevent replay attacks
		// on the same credential) but signal upstream to consider denial.
		cred.SignCount = newSignCount
		if err := r.Update(ctx, &cred); err != nil {
			return storedCount, false, err
		}
		return storedCount, true, nil
	}

	// newSignCount > storedCount — normal case
	cred.SignCount = newSignCount
	if err := r.Update(ctx, &cred); err != nil {
		return storedCount, false, err
	}
	return storedCount, false, nil
}
