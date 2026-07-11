package authz

// RBACWithDomainsModel is the primary RBAC model with tenant domains.
// Request: (subOwner, subName, method, urlPath, objOwner)
// Policy:  (subOwner, subName, method, urlPath, objOwner)
// Grouping: g = _, _, _  (user/role relationships are injected per-request)
const RBACWithDomainsModel = `
[request_definition]
r = subOwner, subName, method, urlPath, objOwner

[policy_definition]
p = subOwner, subName, method, urlPath, objOwner

[role_definition]
g = _, _, _

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = g(r.subName, p.subName, r.subOwner) && keyMatch(r.subOwner, p.subOwner) && r.method == p.method && keyMatch3(r.urlPath, p.urlPath) && keyMatch(r.objOwner, p.objOwner)
`

// ABACWithDomainsModel is the secondary ABAC model used for routes marked
// with RuleTypeABAC. It evaluates structured Subject/Object expressions
// inside a tenant domain.
const ABACWithDomainsModel = `
[request_definition]
r = sub, dom, method, path, obj

[policy_definition]
p = subRule, dom, method, pathRule, objRule

[policy_effect]
e = some(where (p.eft == allow))

[matchers]
m = eval(p.subRule) && r.dom == p.dom && r.method == p.method && keyMatch3(r.path, p.pathRule) && eval(p.objRule)
`

// Subject is passed to the ABAC enforcer as r.sub.
type Subject struct {
	ID    string
	Owner string
	Roles []interface{}
	Attrs map[string]any
}

// Object is passed to the ABAC enforcer as r.obj.
type Object struct {
	Owner string
	Name  string
	Attrs map[string]any
}
