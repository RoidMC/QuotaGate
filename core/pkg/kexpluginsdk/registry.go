// Package kexpluginsdk holds the generic plugin-registry skeleton shared by
// KexCore Universal Plugin System. It deliberately
// knows nothing about any specific plugin: it only manages registration and
// lookup of factories by name, behind a concurrency-safe, duplicate-rejecting
// API.
//
// Why a factory registry (not an instance registry):
//
//   - Per-request provider instances carry credentials (OAuth secrets, S3
//     keys, ...) that must come from per-tenant DB config, never be cached in
//     a long-lived singleton. Factories are stateless and self-register in
//     init(); instances are built just-in-time via the system's own New(cfg).
//
//   - Presentation is the caller's concern. The registry exposes Range for
//     iteration and Get/Has for lookup, but defines no Method descriptor — each
//     plugin system builds its own Methods() from the factories it holds.
package kexpluginsdk

import (
	"fmt"
	"sync"
)

// Factory is the minimal contract every plugin factory satisfies: a unique
// Name used as the registry key. Each plugin system extends this with its own
// factory methods (e.g. New(cfg)) and its own capability markers.
type Factory interface {
	// Name is the unique provider identifier, e.g. "github", "local",
	// "webauthn". Used as the registry key and in route paths.
	Name() string
}

// Registry is the generic, concurrency-safe plugin registry. It holds
// compiled-in factories of type F indexed by Name; it does NOT hold
// provisioned provider instances.
type Registry[F Factory] struct {
	mu        sync.RWMutex
	factories map[string]F
}

// NewRegistry returns an empty registry ready for Register calls.
func NewRegistry[F Factory]() *Registry[F] {
	return &Registry[F]{factories: make(map[string]F)}
}

// Register adds a factory. It panics on a duplicate name so wiring mistakes
// (two packages registering the same provider) surface at boot instead of
// silently overwriting the earlier registration.
func (r *Registry[F]) Register(f F) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name := f.Name()
	if _, dup := r.factories[name]; dup {
		panic(fmt.Sprintf("kexpluginsdk: duplicate factory %q", name))
	}
	r.factories[name] = f
}

// Get returns the factory registered under name.
func (r *Registry[F]) Get(name string) (F, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.factories[name]
	return f, ok
}

// Has reports whether a factory is registered under name.
func (r *Registry[F]) Has(name string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	_, ok := r.factories[name]
	return ok
}

// Range iterates factories in unspecified order. Returning false from fn
// stops iteration. Used by each system to build its own Methods() descriptor.
func (r *Registry[F]) Range(fn func(name string, f F) bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for name, f := range r.factories {
		if !fn(name, f) {
			return
		}
	}
}

// Len returns the number of registered factories.
func (r *Registry[F]) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.factories)
}
