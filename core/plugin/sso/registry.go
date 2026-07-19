package sso

import (
	"fmt"
	"sort"
	"sync"
)

// Method is a provider's public descriptor, surfaced via the registry's
// Methods() call and ultimately through GET /auth/methods so the front-end
// knows which login options exist. Note: this lists providers that are
// *compiled in*. Whether a provider is enabled for a specific tenant is a
// runtime DB lookup, not reflected here.
//
// Display is intentionally NOT populated by the registry — the caller (API
// consumer) is responsible for mapping Name to whatever label its UI needs.
// The registry does not hardcode display names because they are a
// presentation concern, not a plugin concern.
type Method struct {
	// Name is the provider identifier, e.g. "github", "wechat-mp".
	Name string `json:"name"`

	// Flow is "redirect" or "qr", derived from the factory's Flow().
	Flow string `json:"flow"`
}

// Registry holds all compiled-in ProviderFactories, indexed by name.
// Factories self-register in init(); the registry is built once at boot.
// Per-request provider instances are created via New(cfg), not stored here,
// because credentials live in the per-tenant ProviderConfig (from DB).
type Registry struct {
	mu        sync.RWMutex
	factories map[string]ProviderFactory
}

func NewRegistry() *Registry {
	return &Registry{
		factories: make(map[string]ProviderFactory),
	}
}

// RegisterFactory registers a ProviderFactory. Called from init() in each
// provider package. Panics on duplicate name to surface wiring mistakes at
// boot.
func (r *Registry) RegisterFactory(f ProviderFactory) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := f.Name()
	if _, dup := r.factories[name]; dup {
		panic(fmt.Sprintf("sso: duplicate provider factory %q", name))
	}
	r.factories[name] = f
}

// New instantiates a provider for a single request using per-tenant config.
// The caller must have already verified the tenant has this provider enabled
// (i.e. ProviderConfig came from a DB row).
func (r *Registry) New(cfg ProviderConfig) (Provider, error) {
	r.mu.RLock()
	f, ok := r.factories[cfg.Name]
	r.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("sso: unknown provider %q", cfg.Name)
	}
	return f.New(cfg)
}

// Has reports whether a provider name is compiled in.
func (r *Registry) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[name]
	return ok
}

// Methods returns all compiled-in providers, sorted by name, for /auth/methods.
// This is the union of all providers the binary can serve; the front-end
// should intersect it with the tenant's enabled-provider list from the DB.
func (r *Registry) Methods() []Method {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Method, 0, len(r.factories))
	for name, f := range r.factories {
		flow := "redirect"
		if f.Flow() == FlowQR {
			flow = "qr"
		}
		out = append(out, Method{
			Name: name,
			Flow: flow,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// defaultRegistry is the package-level registry populated by init()s of real
// provider packages (standard/github, china/wechat_mp, ...). boot.InitSSO
// returns this. Mock providers do NOT register here — they live in the mock
// subpackage and expose NewRegistry(store) for test isolation.
var defaultRegistry = NewRegistry()

// DefaultRegistry returns the registry populated by real provider init()s.
// Importing a provider package (e.g. _ ".../plugin/sso/standard") causes its
// factory to self-register; boot imports the provider packages it wants
// compiled in.
func DefaultRegistry() *Registry { return defaultRegistry }
