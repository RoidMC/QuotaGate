package identity

import (
	"context"
	"errors"

	"github.com/roidmc/kex-utils/pkg/crypto"
)

var (
	ErrInvalidCredentials = errors.New("quotagate/identity: invalid credentials")
	ErrMissingCredentials = errors.New("quotagate/identity: missing credentials")
)

type PasswordProvider struct{}

func NewPasswordProvider() *PasswordProvider {
	return &PasswordProvider{}
}

func (p *PasswordProvider) Name() string {
	return "password"
}

func (p *PasswordProvider) Authenticate(ctx context.Context, credentials map[string]string) (*Identity, error) {
	passwordHash, ok := credentials["password_hash"]
	if !ok {
		return nil, ErrMissingCredentials
	}

	password, ok := credentials["password"]
	if !ok {
		return nil, ErrMissingCredentials
	}

	userID, ok := credentials["user_id"]
	if !ok {
		return nil, ErrMissingCredentials
	}

	match, err := crypto.Argon2idVerify(password, passwordHash)
	if err != nil {
		return nil, err
	}

	if !match {
		return nil, ErrInvalidCredentials
	}

	return &Identity{
		UserID:   userID,
		Provider: p.Name(),
	}, nil
}
