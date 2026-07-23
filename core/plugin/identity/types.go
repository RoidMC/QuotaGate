package identity

import (
	"context"
	"encoding/json"
	"sort"

	"github.com/roidmc/kex-utils/pkg/kexpluginsdk"
)

type Authenticator interface {
	Name() string
	Authenticate(ctx context.Context, credentials map[string]string) (*Identity, error)
}

type ChallengeAuthenticator interface {
	Name() string
	// BeginChallenge / FinishChallenge take the request host and scheme so the
	// provider can derive the relying-party ID (RPID) from Host instead of a
	// fixed boot config. host is the request Host header (may include a port);
	// scheme is "http" or "https".
	BeginChallenge(ctx context.Context, params map[string]string, host, scheme string) (*Challenge, error)
	FinishChallenge(ctx context.Context, challengeID string, response json.RawMessage, host, scheme string) (*Identity, error)
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

// Method is the public descriptor surfaced via Methods(). Display is
// intentionally absent — presentation (labels, icons) is the caller's
// concern, not the plugin's.
type Method struct {
	Name string `json:"name"`
	// Type discriminates the capability: "authenticator" | "challenge" |
	// "delegated". A single provider may appear under multiple types if it
	// implements more than one capability interface.
	Type string `json:"type"`
}

// Factory builds the provider instances for a single named identity method.
// A factory may implement one or more capabilities; Capabilities() reports
// which, and New() returns the corresponding facets (nil for unsupported
// ones). This collapses the old three-map design (authenticators /
// challengers / delegators) into one map keyed by name with a capability
// discriminator.
//
// Identity providers are platform-level (configured at boot, not per-tenant),
// so factories hold the provisioned instance and New() returns it; no
// per-request credentials are cached.
type Factory interface {
	kexpluginsdk.Factory
	// Capabilities returns the capability type strings this factory can
	// produce, e.g. {"authenticator"} or {"challenge", "delegated"}.
	Capabilities() []string
	// New returns the provider facets. Each facet is nil when the factory
	// does not support that capability.
	New() (Authenticator, ChallengeAuthenticator, DelegatedAuthenticator, error)
}

// Registry holds all compiled-in identity factories, indexed by name. The
// underlying store is the shared kexpluginsdk.Registry; this type adds the
// capability-aware accessors that return the right typed facet.
type Registry struct {
	*kexpluginsdk.Registry[Factory]
}

func NewRegistry() *Registry {
	return &Registry{Registry: kexpluginsdk.NewRegistry[Factory]()}
}

// GetAuthenticator returns the Authenticator facet of the named factory, if
// it supports the "authenticator" capability.
func (r *Registry) GetAuthenticator(name string) (Authenticator, bool) {
	f, ok := r.Get(name)
	if !ok {
		return nil, false
	}
	a, _, _, _ := f.New()
	if a == nil {
		return nil, false
	}
	return a, true
}

// GetChallenger returns the ChallengeAuthenticator facet of the named factory.
func (r *Registry) GetChallenger(name string) (ChallengeAuthenticator, bool) {
	f, ok := r.Get(name)
	if !ok {
		return nil, false
	}
	_, c, _, _ := f.New()
	if c == nil {
		return nil, false
	}
	return c, true
}

// GetDelegator returns the DelegatedAuthenticator facet of the named factory.
func (r *Registry) GetDelegator(name string) (DelegatedAuthenticator, bool) {
	f, ok := r.Get(name)
	if !ok {
		return nil, false
	}
	_, _, d, _ := f.New()
	if d == nil {
		return nil, false
	}
	return d, true
}

// Methods returns the descriptor list for /auth/methods: one entry per
// (factory, capability) pair, sorted by name then type for stable output.
// Display names are deliberately omitted — the API consumer maps Name to its
// own label set.
func (r *Registry) Methods() []Method {
	out := make([]Method, 0, r.Len())
	r.Range(func(name string, f Factory) bool {
		for _, cap := range f.Capabilities() {
			out = append(out, Method{Name: name, Type: cap})
		}
		return true
	})
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Type < out[j].Type
	})
	return out
}
