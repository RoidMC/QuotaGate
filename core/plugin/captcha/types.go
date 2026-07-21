// QuotaGate captcha plugin interface and registry (second-generation factory
// architecture, shared with sso/identity/storage/payment/relay via kexpluginsdk).
//
// Captcha providers may need per-tenant config (site keys, secrets). Those live
// in the per-tenant ProviderConfig loaded from the database, not in a singleton.
// Factories self-register in init(); instances are built just-in-time via
// Factory.New(cfg).
package captcha

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"github.com/roidmc/quotagate/pkg/kexpluginsdk"
)

// Provider is the minimal captcha backend contract. Concrete providers extend
// it with their own challenge/verify methods.
type Provider interface {
	Name() string
	Version() string
	Type() string
}

// Verifier is the capability implemented by challenge-response captcha
// providers (turnstile, recaptcha, ...). The frontend obtains a short-lived
// token from the provider's widget; the backend must verify that token via
// Verify BEFORE trusting any submitted business data. Providers that do not use
// a token (e.g. purely invisible risk analysis with no backend call) need not
// implement it.
type Verifier interface {
	// Verify checks a frontend-supplied token against the provider's
	// siteverify endpoint. remoteIP is optional but recommended: when non-empty
	// it is forwarded so the provider's abuse heuristics can use it. It returns
	// true only when the challenge actually passed.
	Verify(ctx context.Context, token, remoteIP string) (bool, error)
}

// PublicConfigProvider is implemented by providers whose frontend widget needs a
// public key/config (sitekey, captcha_id, ...) delivered to the browser before
// the challenge can start. The backend exposes these via a /captcha/config
// endpoint; they are NOT secrets. Providers loaded entirely server-side (no
// browser widget) need not implement it.
type PublicConfigProvider interface {
	// PublicConfig returns the public, browser-safe configuration the frontend
	// needs to render the widget.
	PublicConfig() map[string]string
}

// Capability names declared by captcha factories via Factory.Capabilities().
// They mirror the optional provider interfaces above so Validate can confirm at
// boot that a factory's produced instance actually implements what it claims.
const (
	// CapVerifier is declared by providers that verify a frontend-supplied
	// challenge token (turnstile, recaptcha, geetest, ...).
	CapVerifier = "verifier"
	// CapPublicConfig is declared by providers whose widget needs a public
	// key/config (sitekey, captcha_id, ...) delivered to the browser before the
	// challenge can start.
	CapPublicConfig = "public-config"
)

// capabilityIface maps a declared capability name to the Go interface the
// produced Provider must satisfy. Validate uses it to catch mis-wired plugins
// at boot instead of at request time.
var capabilityIface = map[string]reflect.Type{
	CapVerifier:     reflect.TypeOf((*Verifier)(nil)).Elem(),
	CapPublicConfig: reflect.TypeOf((*PublicConfigProvider)(nil)).Elem(),
}

// ProviderConfig is the runtime, per-(tenant, provider) configuration loaded
// from the database.
type ProviderConfig struct {
	TenantID string
	Name     string
	Extra    map[string]string
}

// Method is the public descriptor for captcha methods listing. No instance is
// exposed.
type Method struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Version string `json:"version"`
}

// Factory builds a configured Provider for a single request.
type Factory interface {
	kexpluginsdk.Factory
	// Type reports the provider family, e.g. "recaptcha" | "turnstile".
	Type() string
	// Version is the provider implementation version.
	Version() string
	// Capabilities reports the capability names (CapVerifier, CapPublicConfig,
	// ...) the produced Provider satisfies. Validate uses it to confirm at boot
	// that the instance actually implements the claimed interfaces.
	Capabilities() []string
	// New returns a Provider configured with the given per-tenant config.
	New(cfg ProviderConfig) (Provider, error)
}

// Registry holds all compiled-in captcha factories, indexed by name.
type Registry struct {
	*kexpluginsdk.Registry[Factory]
}

func NewRegistry() *Registry {
	return &Registry{Registry: kexpluginsdk.NewRegistry[Factory]()}
}

// New instantiates a captcha provider for a single request using per-tenant
// config.
func (r *Registry) New(cfg ProviderConfig) (Provider, error) {
	if cfg.Name == "" {
		return nil, fmt.Errorf("captcha: ProviderConfig.Name is empty")
	}
	f, ok := r.Get(cfg.Name)
	if !ok {
		return nil, fmt.Errorf("captcha: unknown provider %q", cfg.Name)
	}
	return f.New(cfg)
}

// Methods returns the descriptor list, sorted by name. No instance is exposed.
func (r *Registry) Methods() []Method {
	out := make([]Method, 0, r.Len())
	r.Range(func(name string, f Factory) bool {
		out = append(out, Method{Name: name, Type: f.Type(), Version: f.Version()})
		return true
	})
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Validate probes every registered factory: it builds a zero-config instance
// and asserts that the instance actually satisfies each capability the factory
// declares (and only known capabilities). It returns a list of human-readable
// problems; an empty slice means the registry is correctly wired. Callers
// (e.g. the plugin aggregator's init) should panic if this returns anything.
func (r *Registry) Validate() []string {
	var problems []string
	r.Range(func(name string, f Factory) bool {
		inst, err := f.New(ProviderConfig{Name: name})
		if err != nil {
			problems = append(problems, fmt.Sprintf("factory %q: New(zero cfg) failed: %v", name, err))
			return true
		}
		for _, cap := range f.Capabilities() {
			iface, ok := capabilityIface[cap]
			if !ok {
				problems = append(problems, fmt.Sprintf("factory %q: declares unknown capability %q", name, cap))
				continue
			}
			if !reflect.TypeOf(inst).Implements(iface) {
				problems = append(problems, fmt.Sprintf("factory %q: declares %q but produced Provider does not implement it", name, cap))
			}
		}
		return true
	})
	return problems
}
