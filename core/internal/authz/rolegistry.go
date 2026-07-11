package authz

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/roidmc/quotagate/internal/model"
)

// RoleRegistry keeps role inheritance rules in memory.
//
// It is used at startup and on role mutations to build the grouping rules
// that are loaded into the global Casbin enforcer. System-level inheritance
// (e.g. admin -> user) is stored under the wildcard domain "*".
type RoleRegistry struct {
	mu    sync.RWMutex
	rules map[string][][]string // domain -> []g-rule(child, parent, domain)
}

// NewRoleRegistry creates an empty registry.
func NewRoleRegistry() *RoleRegistry {
	return &RoleRegistry{
		rules: make(map[string][][]string),
	}
}

// Load rebuilds the registry from the provided role definitions.
// It expands InheritedRoles recursively. System roles (TenantID == "") are
// placed under the wildcard domain "*" so that Casbin's domain matching can
// apply them to every tenant request.
func (r *RoleRegistry) Load(ctx context.Context, defs []model.RoleDefinition) error {
	// Build per-domain inheritance graphs: domain -> role -> direct parents.
	graphs := make(map[string]map[string][]string)
	for _, def := range defs {
		if def.Name == "" {
			continue
		}
		domain := def.TenantID
		if domain == "" {
			domain = "*"
		}
		if graphs[domain] == nil {
			graphs[domain] = make(map[string][]string)
		}
		graphs[domain][def.Name] = def.InheritedRoles
	}

	newRules := make(map[string][][]string)
	for domain, graph := range graphs {
		memo := make(map[string][]string)
		for role := range graph {
			ancestors, err := expandInheritedRoles(role, graph, memo, map[string]bool{})
			if err != nil {
				return fmt.Errorf("quotagate/authz: expand inheritance for role %q in domain %q: %w", role, domain, err)
			}
			for _, parent := range ancestors {
				// Casbin g-rule: g(child, parent, domain) means child inherits parent.
				newRules[domain] = append(newRules[domain], []string{role, parent, domain})
			}
		}
		// Deterministic ordering helps tests and debugging.
		sort.Slice(newRules[domain], func(i, j int) bool {
			a, b := newRules[domain][i], newRules[domain][j]
			if a[0] != b[0] {
				return a[0] < b[0]
			}
			return a[1] < b[1]
		})
	}

	r.mu.Lock()
	r.rules = newRules
	r.mu.Unlock()
	_ = ctx // reserved for future use (logging / tracing)
	return nil
}

// expandInheritedRoles recursively expands the inheritance chain for role
// within a single domain. It detects cycles by tracking visited roles.
func expandInheritedRoles(role string, graph map[string][]string, memo map[string][]string, visiting map[string]bool) ([]string, error) {
	if visiting[role] {
		return nil, fmt.Errorf("cyclic inheritance detected at role %q", role)
	}
	if cached, ok := memo[role]; ok {
		return cached, nil
	}

	visiting[role] = true
	defer delete(visiting, role)

	seen := make(map[string]bool)
	var result []string
	for _, parent := range graph[role] {
		if parent == "" || parent == role || seen[parent] {
			continue
		}
		seen[parent] = true
		result = append(result, parent)

		grandParents, err := expandInheritedRoles(parent, graph, memo, visiting)
		if err != nil {
			return nil, err
		}
		for _, gp := range grandParents {
			if !seen[gp] {
				seen[gp] = true
				result = append(result, gp)
			}
		}
	}

	memo[role] = result
	return result, nil
}

// GetInheritanceRules returns all g-rules for the requested domain.
//
// System inheritance rules (stored under domain "*") are always included,
// because chains such as admin -> user must apply to every tenant. When the
// requested domain is not "*", tenant-scoped rules are appended after system
// rules.
func (r *RoleRegistry) GetInheritanceRules(domain string) [][]string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var out [][]string
	if systemRules, ok := r.rules["*"]; ok {
		out = append(out, systemRules...)
	}
	if domain != "" && domain != "*" {
		if tenantRules, ok := r.rules[domain]; ok {
			out = append(out, tenantRules...)
		}
	}
	return out
}

// Reload is a convenience wrapper around Load.
func (r *RoleRegistry) Reload(ctx context.Context, defs []model.RoleDefinition) error {
	return r.Load(ctx, defs)
}
