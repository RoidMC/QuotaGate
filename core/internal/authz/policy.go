package authz

// defaultPolicies are the bootstrap rules seeded on first startup.
// Admins can customize them later via the policy API; these defaults
// only need to cover the common "admin manages the gateway, user manages self" case.
var defaultPolicies = [][]string{
	// Public auth endpoints
	{"anonymous", "/api/auth/register", "POST"},
	{"anonymous", "/api/auth/login", "POST"},
	{"anonymous", "/api/auth/refresh", "POST"},
	{"anonymous", "/api/auth/methods", "GET"},

	// User self-service
	{"user", "/api/my-account", "GET"},
	{"user", "/api/my-account", "PUT"},
	{"user", "/api/my-account/password", "POST"},

	{"user", "/api/tokens", "GET"},
	{"user", "/api/tokens", "POST"},
	{"user", "/api/tokens/{id}", "GET"},
	{"user", "/api/tokens/{id}", "PUT"},
	{"user", "/api/tokens/{id}", "DELETE"},

	{"user", "/api/models", "GET"},
	{"user", "/api/pricing", "GET"},
	{"user", "/api/channels", "GET"},
	{"user", "/api/logs/self", "GET"},
	{"user", "/api/billing/self", "GET"},
	{"user", "/api/topup", "GET"},
	{"user", "/api/topup", "POST"},

	// Admin general management
	{"admin", "/api/users", "GET"},
	{"admin", "/api/users", "POST"},
	{"admin", "/api/users/{id}", "GET"},
	{"admin", "/api/users/{id}", "PUT"},
	{"admin", "/api/users/{id}", "DELETE"},

	{"admin", "/api/roles", "GET"},
	{"admin", "/api/roles", "POST"},
	{"admin", "/api/roles/{id}", "PUT"},
	{"admin", "/api/roles/{id}", "DELETE"},

	{"admin", "/api/policies", "GET"},
	{"admin", "/api/policies", "POST"},
	{"admin", "/api/policies/{id}", "DELETE"},

	// Admin gateway management
	{"admin", "/api/channels", "POST"},
	{"admin", "/api/channels/{id}", "GET"},
	{"admin", "/api/channels/{id}", "PUT"},
	{"admin", "/api/channels/{id}", "DELETE"},
	{"admin", "/api/channels/{id}/test", "POST"},

	{"admin", "/api/models", "POST"},
	{"admin", "/api/models/{id}", "PUT"},
	{"admin", "/api/models/{id}", "DELETE"},
	{"admin", "/api/pricing", "POST"},
	{"admin", "/api/pricing", "PUT"},

	// Admin operations
	{"admin", "/api/logs", "GET"},
	{"admin", "/api/audits", "GET"},
	{"admin", "/api/redemptions", "GET"},
	{"admin", "/api/redemptions", "POST"},
	{"admin", "/api/redemptions/{id}", "DELETE"},
	{"admin", "/api/topup/complete", "POST"},
	{"admin", "/api/billing", "GET"},

	// Admin system settings
	{"admin", "/api/options", "GET"},
	{"admin", "/api/options", "PUT"},
	{"admin", "/api/system/status", "GET"},
	{"admin", "/api/system/performance", "GET"},
}

var defaultRoles = [][]string{
	{"admin", "user"},
}
