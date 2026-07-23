package authz

// defaultPolicies are the bootstrap p-rules seeded on first startup.
// They use the domain RBAC five-tuple:
// (subOwner, subName, method, urlPath, objOwner).
//
// subOwner = "*" means the policy applies to all tenants (system-level policy).
// objOwner = "*" means the resource is system-wide or tenant-agnostic.
// Role inheritance (e.g. admin -> user) is derived from RoleDefinition.InheritedRoles
// at runtime and is NOT stored here.
var defaultPolicies = [][]string{
	// User self-service
	{"*", "user", "GET", "/api/my-account", "*"},
	{"*", "user", "PUT", "/api/my-account", "*"},
	{"*", "user", "POST", "/api/my-account/password", "*"},

	// Tenant membership bridge (multi-tenant self-service)
	{"*", "user", "POST", "/api/switch-tenant", "*"},
	{"*", "user", "GET", "/api/my-invitations", "*"},
	{"*", "user", "POST", "/api/my-invitations/{id}/accept", "*"},
	{"*", "user", "POST", "/api/my-invitations/{id}/reject", "*"},
	{"*", "user", "POST", "/api/tenants/{tenantId}/invite", "*"},

	{"*", "user", "GET", "/api/tokens", "*"},
	{"*", "user", "POST", "/api/tokens", "*"},
	{"*", "user", "GET", "/api/tokens/{id}", "*"},
	{"*", "user", "PUT", "/api/tokens/{id}", "*"},
	{"*", "user", "DELETE", "/api/tokens/{id}", "*"},

	{"*", "user", "GET", "/api/models", "*"},
	{"*", "user", "GET", "/api/pricing", "*"},
	{"*", "user", "GET", "/api/channels", "*"},
	{"*", "user", "GET", "/api/logs/self", "*"},
	{"*", "user", "GET", "/api/billing/self", "*"},
	{"*", "user", "GET", "/api/topup", "*"},
	{"*", "user", "POST", "/api/topup", "*"},

	// Admin general management
	{"*", "admin", "GET", "/api/users", "*"},
	{"*", "admin", "POST", "/api/users", "*"},
	{"*", "admin", "GET", "/api/users/{id}", "*"},
	{"*", "admin", "PUT", "/api/users/{id}", "*"},
	{"*", "admin", "DELETE", "/api/users/{id}", "*"},

	{"*", "admin", "GET", "/api/roles", "*"},
	{"*", "admin", "POST", "/api/roles", "*"},
	{"*", "admin", "PUT", "/api/roles/{id}", "*"},
	{"*", "admin", "DELETE", "/api/roles/{id}", "*"},

	{"*", "admin", "GET", "/api/policies", "*"},
	{"*", "admin", "POST", "/api/policies", "*"},
	{"*", "admin", "DELETE", "/api/policies/{id}", "*"},

	// Admin gateway management
	{"*", "admin", "POST", "/api/channels", "*"},
	{"*", "admin", "GET", "/api/channels/{id}", "*"},
	{"*", "admin", "PUT", "/api/channels/{id}", "*"},
	{"*", "admin", "DELETE", "/api/channels/{id}", "*"},
	{"*", "admin", "POST", "/api/channels/{id}/test", "*"},

	{"*", "admin", "POST", "/api/models", "*"},
	{"*", "admin", "PUT", "/api/models/{id}", "*"},
	{"*", "admin", "DELETE", "/api/models/{id}", "*"},
	{"*", "admin", "POST", "/api/pricing", "*"},
	{"*", "admin", "PUT", "/api/pricing", "*"},

	// Admin operations
	{"*", "admin", "GET", "/api/logs", "*"},
	{"*", "admin", "GET", "/api/audits", "*"},
	{"*", "admin", "GET", "/api/redemptions", "*"},
	{"*", "admin", "POST", "/api/redemptions", "*"},
	{"*", "admin", "DELETE", "/api/redemptions/{id}", "*"},
	{"*", "admin", "POST", "/api/topup/complete", "*"},
	{"*", "admin", "GET", "/api/billing", "*"},

	// Admin system settings
	{"*", "admin", "GET", "/api/options", "*"},
	{"*", "admin", "PUT", "/api/options", "*"},
	{"*", "admin", "GET", "/api/system/status", "*"},
	{"*", "admin", "GET", "/api/system/performance", "*"},
}

// RouteMetaTuple is a lightweight (method, path) pair used for bootstrap
// route metadata seeding.
type RouteMetaTuple struct {
	Method string
	Path   string
}

// DefaultRouteMetas returns the bootstrap route metadata tuples.
func DefaultRouteMetas() []RouteMetaTuple {
	return defaultRouteMetas
}

// defaultRouteMetas pairs each default policy route with its rule type.
// All bootstrap routes use RBAC; ABAC routes are configured at runtime.
var defaultRouteMetas = []RouteMetaTuple{
	{"GET", "/api/my-account"},
	{"PUT", "/api/my-account"},
	{"POST", "/api/my-account/password"},

	{"GET", "/api/tokens"},
	{"POST", "/api/tokens"},
	{"GET", "/api/tokens/{id}"},
	{"PUT", "/api/tokens/{id}"},
	{"DELETE", "/api/tokens/{id}"},

	{"GET", "/api/models"},
	{"GET", "/api/pricing"},
	{"GET", "/api/channels"},
	{"GET", "/api/logs/self"},
	{"GET", "/api/billing/self"},
	{"GET", "/api/topup"},
	{"POST", "/api/topup"},

	{"GET", "/api/users"},
	{"POST", "/api/users"},
	{"GET", "/api/users/{id}"},
	{"PUT", "/api/users/{id}"},
	{"DELETE", "/api/users/{id}"},

	{"GET", "/api/roles"},
	{"POST", "/api/roles"},
	{"PUT", "/api/roles/{id}"},
	{"DELETE", "/api/roles/{id}"},

	{"GET", "/api/policies"},
	{"POST", "/api/policies"},
	{"DELETE", "/api/policies/{id}"},

	{"POST", "/api/channels"},
	{"GET", "/api/channels/{id}"},
	{"PUT", "/api/channels/{id}"},
	{"DELETE", "/api/channels/{id}"},
	{"POST", "/api/channels/{id}/test"},

	{"POST", "/api/models"},
	{"PUT", "/api/models/{id}"},
	{"DELETE", "/api/models/{id}"},
	{"POST", "/api/pricing"},
	{"PUT", "/api/pricing"},

	{"GET", "/api/logs"},
	{"GET", "/api/audits"},
	{"GET", "/api/redemptions"},
	{"POST", "/api/redemptions"},
	{"DELETE", "/api/redemptions/{id}"},
	{"POST", "/api/topup/complete"},
	{"GET", "/api/billing"},

	{"GET", "/api/options"},
	{"PUT", "/api/options"},
	{"GET", "/api/system/status"},
	{"GET", "/api/system/performance"},
}
