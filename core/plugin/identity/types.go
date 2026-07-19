package identity

import (
	"context"
	"encoding/json"
)

type Authenticator interface {
	Name() string
	Authenticate(ctx context.Context, credentials map[string]string) (*Identity, error)
}

type ChallengeAuthenticator interface {
	Name() string
	BeginChallenge(ctx context.Context, params map[string]string) (*Challenge, error)
	FinishChallenge(ctx context.Context, challengeID string, response json.RawMessage) (*Identity, error)
}

type DelegatedAuthenticator interface {
	Name() string
	Generate(ctx context.Context, params map[string]string) (*Delegation, error)
	Scan(ctx context.Context, code string, userID string) error
	Confirm(ctx context.Context, code string, userID string) (*Identity, error)
	Cancel(ctx context.Context, code string, userID string) error
	Status(ctx context.Context, code string) (*DelegationStatus, error)
}

type Identity struct {
	UserID   string            `json:"user_id"`
	Provider string            `json:"provider"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

type Challenge struct {
	ChallengeID string          `json:"challenge_id"`
	Options     json.RawMessage `json:"options"`
}

type Delegation struct {
	Code      string `json:"code"`
	QRData    string `json:"qr_data"`
	ExpiresAt int64  `json:"expires_at"`
}

type DelegationStatus struct {
	Status string `json:"status"`
	UserID string `json:"user_id,omitempty"`
}

const (
	DelegationPending   = "pending"
	DelegationScanned   = "scanned"
	DelegationConfirmed = "confirmed"
	DelegationCancelled = "cancelled"
	DelegationExpired   = "expired"
)

type Method struct {
	Name    string `json:"name"`
	Display string `json:"display"`
	Type    string `json:"type"`
}

type Registry struct {
	authenticators map[string]Authenticator
	challengers    map[string]ChallengeAuthenticator
	delegators     map[string]DelegatedAuthenticator
}

func NewRegistry() *Registry {
	return &Registry{
		authenticators: make(map[string]Authenticator),
		challengers:    make(map[string]ChallengeAuthenticator),
		delegators:     make(map[string]DelegatedAuthenticator),
	}
}

func (r *Registry) RegisterAuthenticator(a Authenticator) {
	r.authenticators[a.Name()] = a
}

func (r *Registry) RegisterChallenger(c ChallengeAuthenticator) {
	r.challengers[c.Name()] = c
}

func (r *Registry) RegisterDelegator(d DelegatedAuthenticator) {
	r.delegators[d.Name()] = d
}

func (r *Registry) GetAuthenticator(name string) (Authenticator, bool) {
	a, ok := r.authenticators[name]
	return a, ok
}

func (r *Registry) GetChallenger(name string) (ChallengeAuthenticator, bool) {
	c, ok := r.challengers[name]
	return c, ok
}

func (r *Registry) GetDelegator(name string) (DelegatedAuthenticator, bool) {
	d, ok := r.delegators[name]
	return d, ok
}

func (r *Registry) Methods() []Method {
	methods := make([]Method, 0, len(r.authenticators)+len(r.challengers)+len(r.delegators))
	for _, a := range r.authenticators {
		methods = append(methods, Method{
			Name:    a.Name(),
			Display: displayName(a.Name()),
			Type:    "authenticator",
		})
	}
	for _, c := range r.challengers {
		methods = append(methods, Method{
			Name:    c.Name(),
			Display: displayName(c.Name()),
			Type:    "challenge",
		})
	}
	for _, d := range r.delegators {
		methods = append(methods, Method{
			Name:    d.Name(),
			Display: displayName(d.Name()),
			Type:    "delegated",
		})
	}
	return methods
}

func displayName(name string) string {
	switch name {
	case "password":
		return "Password"
	case "webauthn":
		return "Passkey"
	case "qrcode":
		return "QR Code"
	default:
		return name
	}
}
