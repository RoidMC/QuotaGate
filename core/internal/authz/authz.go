// QuotaGate authorization manager.
//
// The design uses a single global Casbin SyncedEnforcer for the primary domain
// RBAC check. At startup (and after any role mutation) the enforcer is loaded
// with:
//   - p policies from the casbin_rule adapter;
//   - role inheritance g rules derived from RoleDefinition.InheritedRoles;
//   - user-role assignment g rules from UserRoleAssignment.
//
// Per-request authorization simply calls Enforce on the shared enforcer. A
// domain-matching function allows system-level inheritance rules (domain "*")
// to apply to every tenant request.

package authz

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/casbin/casbin/v3"
	casbinmodel "github.com/casbin/casbin/v3/model"
	"github.com/casbin/casbin/v3/util"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/types"
	"github.com/roidmc/quotagate/pkg/kexswiftbus"
	"gorm.io/gorm"
)

var ErrInvalidABACRule = errors.New("quotagate/authz: invalid ABAC rule")

const (
	rbacTable  = "casbin_rule"
	abacTable  = "casbin_rule_abac"
	systemRole = "*" // wildcard owner used for system-level policies
)

// Event payloads for cross-instance authz synchronization.
type roleAssignEvent struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

type roleRevokeEvent struct {
	UserID   string `json:"user_id"`
	Role     string `json:"role"`
	TenantID string `json:"tenant_id"`
}

type roleChangedEvent struct {
	RoleName string `json:"role_name"`
}

// GroupingLoader loads the full set of role definitions and user-role
// assignments from the authoritative store. It is used by AuthzManager when
// reacting to cross-instance "role changed" events.
type GroupingLoader func(ctx context.Context) ([]model.RoleDefinition, []model.UserRoleAssignment, error)

// AuthzManagerOption configures an AuthzManager during construction.
type AuthzManagerOption func(*AuthzManager)

// WithEventBus wires the manager to an event bus for cross-instance authz
// synchronization. instanceID identifies this process so that events published
// locally are not re-applied locally. loader is optional and is required for
// processing role-definition change events from peers.
func WithEventBus(bus *event.EventBus, instanceID string, loader GroupingLoader) AuthzManagerOption {
	return func(m *AuthzManager) {
		m.eventBus = bus
		m.instanceID = instanceID
		m.groupingLoader = loader
	}
}

// AuthzManager coordinates the primary domain RBAC enforcer, the in-memory
// role inheritance registry, and the optional secondary ABAC enforcer.
type AuthzManager struct {
	enforcer     *casbin.SyncedEnforcer
	abacEnforcer *casbin.SyncedEnforcer
	roleRegistry *RoleRegistry
	adapter      *gormadapter.Adapter
	abacAdapter  *gormadapter.Adapter

	// Cached grouping inputs so that ReloadPolicy can rebuild in-memory g
	// rules after reloading p policies from the adapter.
	lastDefs        []model.RoleDefinition
	lastAssignments []model.UserRoleAssignment

	// Guards lastDefs/lastAssignments during concurrent runtime updates.
	mu sync.Mutex

	// Cross-instance synchronization.
	eventBus       *event.EventBus
	instanceID     string
	groupingLoader GroupingLoader
	cancelSubs     []kexswiftbus.CancelFunc

	initOnce sync.Once
	initDone bool
	initErr  error
}

// NewAuthzManager creates the authorization manager.
// ABAC enforcer is unconditionally initialized; there is no longer a toggle
// to disable it. RBAC + ABAC are both always active.
func NewAuthzManager(db *gorm.DB, opts ...AuthzManagerOption) (*AuthzManager, error) {
	adapter, err := gormadapter.NewAdapterByDBUseTableName(db, "", rbacTable)
	if err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to create rbac adapter: %w", err)
	}

	m, err := casbinmodel.NewModelFromString(RBACWithDomainsModel)
	if err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to create rbac model: %w", err)
	}

	enforcer, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to create rbac enforcer: %w", err)
	}
	enforcer.EnableAutoSave(true)
	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to load rbac policy: %w", err)
	}

	// Allow system-level grouping rules (domain "*") to match any concrete
	// tenant domain used during enforcement.
	rm := enforcer.GetRoleManager()
	rm.AddDomainMatchingFunc("keyMatch", util.KeyMatch)

	manager := &AuthzManager{
		enforcer:     enforcer,
		roleRegistry: NewRoleRegistry(),
		adapter:      adapter,
	}

	abacAdapter, err := gormadapter.NewAdapterByDBUseTableName(db, "", abacTable)
	if err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to create abac adapter: %w", err)
	}
	abacModel, err := casbinmodel.NewModelFromString(ABACWithDomainsModel)
	if err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to create abac model: %w", err)
	}
	abacEnforcer, err := casbin.NewSyncedEnforcer(abacModel, abacAdapter)
	if err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to create abac enforcer: %w", err)
	}
	abacEnforcer.EnableAutoSave(true)
	if err := abacEnforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to load abac policy: %w", err)
	}
	manager.abacEnforcer = abacEnforcer
	manager.abacAdapter = abacAdapter

	for _, opt := range opts {
		opt(manager)
	}

	return manager, nil
}

// InitGroupingRelations loads role inheritance and user-role assignment rules
// into the global enforcer. It must be called once at startup after default
// policies have been seeded, and again after ReloadPolicy or full role
// mutations.
//
// Grouping rules are kept in memory only (auto-save is disabled while they are
// loaded) so that the casbin_rule adapter continues to store only p policies.
func (m *AuthzManager) InitGroupingRelations(ctx context.Context, defs []model.RoleDefinition, assignments []model.UserRoleAssignment) error {
	return m.rebuildGroupingRelations(ctx, defs, assignments)
}

// ReloadGroupingRelations rebuilds all in-memory grouping rules. It is a
// convenience wrapper for callers that already have defs and assignments.
func (m *AuthzManager) ReloadGroupingRelations(ctx context.Context, defs []model.RoleDefinition, assignments []model.UserRoleAssignment) error {
	return m.rebuildGroupingRelations(ctx, defs, assignments)
}

// InitRoleRegistry initializes the in-memory role inheritance registry without
// touching user-role assignments. It is intended for tests and for callers that
// load assignments separately.
func (m *AuthzManager) InitRoleRegistry(ctx context.Context, defs []model.RoleDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.roleRegistry.Load(ctx, defs); err != nil {
		return err
	}
	m.lastDefs = defs
	m.lastAssignments = nil

	m.enforcer.EnableAutoSave(false)
	defer m.enforcer.EnableAutoSave(true)

	if _, err := m.enforcer.RemoveFilteredGroupingPolicy(0, "", "", ""); err != nil {
		return fmt.Errorf("quotagate/authz: failed to clear grouping policies: %w", err)
	}
	for _, rules := range m.roleRegistry.rules {
		for _, rule := range rules {
			if _, err := m.enforcer.AddGroupingPolicy(rule); err != nil {
				return fmt.Errorf("quotagate/authz: failed to add inheritance rule: %w", err)
			}
		}
	}
	return nil
}

// ReloadRoleRegistry rebuilds the role inheritance rules while preserving the
// current user-role assignment cache. Use it after RoleDefinition mutations
// that do not change assignments.
func (m *AuthzManager) ReloadRoleRegistry(ctx context.Context, defs []model.RoleDefinition) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.roleRegistry.Load(ctx, defs); err != nil {
		return err
	}
	m.lastDefs = defs

	m.enforcer.EnableAutoSave(false)
	defer m.enforcer.EnableAutoSave(true)

	if _, err := m.enforcer.RemoveFilteredGroupingPolicy(0, "", "", ""); err != nil {
		return fmt.Errorf("quotagate/authz: failed to clear grouping policies: %w", err)
	}
	for _, rules := range m.roleRegistry.rules {
		for _, rule := range rules {
			if _, err := m.enforcer.AddGroupingPolicy(rule); err != nil {
				return fmt.Errorf("quotagate/authz: failed to add inheritance rule: %w", err)
			}
		}
	}
	for _, a := range m.lastAssignments {
		if a.UserID == "" || a.Role == "" {
			continue
		}
		if _, err := m.enforcer.AddGroupingPolicy(a.UserID, a.Role, a.TenantID); err != nil {
			return fmt.Errorf("quotagate/authz: failed to add user-role rule: %w", err)
		}
	}
	return nil
}

// AssignUserRole adds a single user-role assignment to the global enforcer and
// keeps the cached assignment list in sync. Auto-save is disabled so that g
// rules remain in memory only.
func (m *AuthzManager) AssignUserRole(userID, role, tenantID string) (bool, error) {
	m.enforcer.EnableAutoSave(false)
	defer m.enforcer.EnableAutoSave(true)

	added, err := m.enforcer.AddGroupingPolicy(userID, role, tenantID)
	if err != nil {
		return false, err
	}
	if added {
		m.mu.Lock()
		m.lastAssignments = append(m.lastAssignments, model.UserRoleAssignment{
			UserID:   userID,
			Role:     role,
			TenantID: tenantID,
		})
		m.mu.Unlock()
	}
	return added, nil
}

// RevokeUserRole removes a single user-role assignment from the global enforcer
// and keeps the cached assignment list in sync. Auto-save is disabled so that g
// rules remain in memory only.
func (m *AuthzManager) RevokeUserRole(userID, role, tenantID string) (bool, error) {
	m.enforcer.EnableAutoSave(false)
	defer m.enforcer.EnableAutoSave(true)

	removed, err := m.enforcer.RemoveGroupingPolicy(userID, role, tenantID)
	if err != nil {
		return false, err
	}
	if removed {
		m.mu.Lock()
		m.lastAssignments = filterAssignment(m.lastAssignments, userID, role, tenantID)
		m.mu.Unlock()
	}
	return removed, nil
}

// InitDefaultPolicies seeds the base RBAC policies and system roles if the
// policy table is empty. It is idempotent.
func (m *AuthzManager) InitDefaultPolicies() error {
	m.initOnce.Do(func() {
		policies, err := m.enforcer.GetPolicy()
		if err != nil {
			m.initErr = fmt.Errorf("quotagate/authz: failed to get policy: %w", err)
			return
		}
		if len(policies) > 0 {
			m.initDone = true
			return
		}

		if _, err := m.enforcer.AddPolicies(defaultPolicies); err != nil {
			m.initErr = fmt.Errorf("quotagate/authz: failed to add default policies: %w", err)
			return
		}
		m.initDone = true
	})

	if !m.initDone {
		return fmt.Errorf("quotagate/authz: initialization previously failed: %w", m.initErr)
	}
	return nil
}

// DefaultSystemRoles returns the bootstrap system role definitions.
func DefaultSystemRoles() []model.RoleDefinition {
	return []model.RoleDefinition{
		{
			Name:           "user",
			Description:    "Default user with self-service permissions",
			IsSystem:       true,
			InheritedRoles: nil,
		},
		{
			Name:           "admin",
			Description:    "System administrator",
			IsSystem:       true,
			InheritedRoles: []string{"user"},
		},
	}
}

// EnforceRBAC performs a domain-aware RBAC check using the global enforcer.
// The roles argument is kept for API compatibility but is no longer used for
// the RBAC decision; grouping rules are resolved from the global enforcer.
func (m *AuthzManager) EnforceRBAC(ctx context.Context, subOwner, userID string, roles []string, method, path, objOwner string) (bool, error) {
	_ = ctx
	_ = roles
	return m.enforcer.Enforce(subOwner, userID, method, path, objOwner)
}

// publishEvent sends a non-blocking event to peers when an authz mutation
// succeeds. Events originating from this instance are ignored by the local
// subscriber to avoid double application.
func (m *AuthzManager) publishEvent(et types.EventType, data interface{}) {
	if m.eventBus == nil {
		return
	}
	m.eventBus.PublishEvent(event.Event{
		ID:        eventID(),
		Type:      et,
		Source:    m.instanceID,
		Data:      data,
		Timestamp: time.Now().UTC(),
	})
}

func eventID() string {
	// Small helper; EventBus does not require globally-unique IDs for pub/sub.
	return fmt.Sprintf("%d", time.Now().UnixNano())
}

// SubscribeToEvents subscribes to cross-instance authz synchronization events.
// It is safe to call when no event bus is configured (no-op). Subscriptions are
// remembered so that Close can unsubscribe cleanly.
func (m *AuthzManager) SubscribeToEvents(ctx context.Context) error {
	_ = ctx
	if m.eventBus == nil {
		return nil
	}

	subscribe := func(et types.EventType, handler event.EventHandler) error {
		cancel, err := m.eventBus.SubscribeEvent(et, handler)
		if err != nil {
			return fmt.Errorf("quotagate/authz: failed to subscribe to %s: %w", et, err)
		}
		m.cancelSubs = append(m.cancelSubs, cancel)
		return nil
	}

	if err := subscribe(types.ActionRoleAssign, m.handleRoleAssign); err != nil {
		return err
	}
	if err := subscribe(types.ActionRoleRevoke, m.handleRoleRevoke); err != nil {
		return err
	}
	if err := subscribe(types.ActionRoleChanged, m.handleRoleChanged); err != nil {
		return err
	}
	return nil
}

// PublishRoleAssign notifies peers that a user-role assignment succeeded.
func (m *AuthzManager) PublishRoleAssign(userID, role, tenantID string) {
	m.publishEvent(types.ActionRoleAssign, roleAssignEvent{
		UserID:   userID,
		Role:     role,
		TenantID: tenantID,
	})
}

// PublishRoleRevoke notifies peers that a user-role assignment was removed.
func (m *AuthzManager) PublishRoleRevoke(userID, role, tenantID string) {
	m.publishEvent(types.ActionRoleRevoke, roleRevokeEvent{
		UserID:   userID,
		Role:     role,
		TenantID: tenantID,
	})
}

// PublishRoleChanged notifies peers that a role definition was mutated.
func (m *AuthzManager) PublishRoleChanged(roleName string) {
	m.publishEvent(types.ActionRoleChanged, roleChangedEvent{
		RoleName: roleName,
	})
}

// isLocalEvent reports whether the event originated from this process.
func (m *AuthzManager) isLocalEvent(evt event.Event) bool {
	return m.instanceID != "" && evt.Source == m.instanceID
}

func (m *AuthzManager) handleRoleAssign(evt event.Event) {
	if m.isLocalEvent(evt) {
		return
	}
	var payload roleAssignEvent
	if err := decodeEventData(evt.Data, &payload); err != nil {
		return
	}
	if payload.UserID == "" || payload.Role == "" {
		return
	}
	_, _ = m.AssignUserRole(payload.UserID, payload.Role, payload.TenantID)
}

func (m *AuthzManager) handleRoleRevoke(evt event.Event) {
	if m.isLocalEvent(evt) {
		return
	}
	var payload roleRevokeEvent
	if err := decodeEventData(evt.Data, &payload); err != nil {
		return
	}
	if payload.UserID == "" || payload.Role == "" {
		return
	}
	_, _ = m.RevokeUserRole(payload.UserID, payload.Role, payload.TenantID)
}

func (m *AuthzManager) handleRoleChanged(evt event.Event) {
	if m.isLocalEvent(evt) {
		return
	}
	if m.groupingLoader == nil {
		return
	}
	defs, assignments, err := m.groupingLoader(context.Background())
	if err != nil {
		return
	}
	_ = m.ReloadGroupingRelations(context.Background(), defs, assignments)
}

// decodeEventData normalizes event data from Redis (map[string]interface{})
// or in-memory bus (struct/[]byte/string) into a concrete struct.
func decodeEventData(data interface{}, out interface{}) error {
	switch v := data.(type) {
	case []byte:
		return json.Unmarshal(v, out)
	case string:
		return json.Unmarshal([]byte(v), out)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		return json.Unmarshal(b, out)
	}
}

// EnforceABAC performs the secondary ABAC check for routes marked with
// RuleTypeABAC. It returns false when ABAC is disabled.
func (m *AuthzManager) EnforceABAC(ctx context.Context, sub Subject, dom, method, path string, obj Object) (bool, error) {
	if m.abacEnforcer == nil {
		return false, nil
	}
	return m.abacEnforcer.Enforce(sub, dom, method, path, obj)
}

// AddPolicy adds a primary RBAC policy tuple.
func (m *AuthzManager) AddPolicy(subOwner, subName, method, path, objOwner string) (bool, error) {
	return m.enforcer.AddPolicy([]string{subOwner, subName, method, path, objOwner})
}

// RemovePolicy removes a primary RBAC policy tuple.
func (m *AuthzManager) RemovePolicy(subOwner, subName, method, path, objOwner string) (bool, error) {
	return m.enforcer.RemovePolicy([]string{subOwner, subName, method, path, objOwner})
}

// HasPolicy checks whether a primary RBAC policy tuple exists.
func (m *AuthzManager) HasPolicy(subOwner, subName, method, path, objOwner string) (bool, error) {
	return m.enforcer.HasPolicy(subOwner, subName, method, path, objOwner)
}

// GetPolicy returns all primary RBAC policy tuples.
func (m *AuthzManager) GetPolicy() ([][]string, error) {
	return m.enforcer.GetPolicy()
}

// AddABACPolicy adds a secondary ABAC policy tuple.
func (m *AuthzManager) AddABACPolicy(subRule, dom, method, pathRule, objRule string) (bool, error) {
	if m.abacEnforcer == nil {
		return false, fmt.Errorf("quotagate/authz: ABAC is not enabled")
	}
	if err := ValidateABACSubRule(subRule); err != nil {
		return false, err
	}
	if err := ValidateABACObjRule(objRule); err != nil {
		return false, err
	}
	return m.abacEnforcer.AddPolicy([]string{subRule, dom, method, pathRule, objRule})
}

// RemoveABACPolicy removes a secondary ABAC policy tuple.
func (m *AuthzManager) RemoveABACPolicy(subRule, dom, method, pathRule, objRule string) (bool, error) {
	if m.abacEnforcer == nil {
		return false, fmt.Errorf("quotagate/authz: ABAC is not enabled")
	}
	return m.abacEnforcer.RemovePolicy([]string{subRule, dom, method, pathRule, objRule})
}

// HasABACPolicy checks whether a secondary ABAC policy tuple exists.
func (m *AuthzManager) HasABACPolicy(subRule, dom, method, pathRule, objRule string) (bool, error) {
	if m.abacEnforcer == nil {
		return false, fmt.Errorf("quotagate/authz: ABAC is not enabled")
	}
	return m.abacEnforcer.HasPolicy(subRule, dom, method, pathRule, objRule)
}

// GetABACPolicy returns all secondary ABAC policy tuples.
func (m *AuthzManager) GetABACPolicy() ([][]string, error) {
	if m.abacEnforcer == nil {
		return nil, nil
	}
	return m.abacEnforcer.GetPolicy()
}

// ReloadPolicy reloads primary RBAC policies from the adapter and rebuilds
// the in-memory grouping rules.
func (m *AuthzManager) ReloadPolicy() error {
	if err := m.enforcer.LoadPolicy(); err != nil {
		return err
	}
	if m.lastDefs != nil {
		return m.initGroupingRelations(m.lastDefs, m.lastAssignments)
	}
	return nil
}

// rebuildGroupingRelations is the internal variant that holds the mutex and
// updates both the enforcer and the cached inputs.
func (m *AuthzManager) rebuildGroupingRelations(ctx context.Context, defs []model.RoleDefinition, assignments []model.UserRoleAssignment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.roleRegistry.Load(ctx, defs); err != nil {
		return err
	}
	m.lastDefs = defs
	m.lastAssignments = assignments

	m.enforcer.EnableAutoSave(false)
	defer m.enforcer.EnableAutoSave(true)

	if _, err := m.enforcer.RemoveFilteredGroupingPolicy(0, "", "", ""); err != nil {
		return fmt.Errorf("quotagate/authz: failed to clear grouping policies: %w", err)
	}

	for _, rules := range m.roleRegistry.rules {
		for _, rule := range rules {
			if _, err := m.enforcer.AddGroupingPolicy(rule); err != nil {
				return fmt.Errorf("quotagate/authz: failed to add inheritance rule: %w", err)
			}
		}
	}

	for _, a := range assignments {
		if a.UserID == "" || a.Role == "" {
			continue
		}
		if _, err := m.enforcer.AddGroupingPolicy(a.UserID, a.Role, a.TenantID); err != nil {
			return fmt.Errorf("quotagate/authz: failed to add user-role rule: %w", err)
		}
	}
	return nil
}

// initGroupingRelations is the internal variant used by ReloadPolicy.
func (m *AuthzManager) initGroupingRelations(defs []model.RoleDefinition, assignments []model.UserRoleAssignment) error {
	return m.rebuildGroupingRelations(context.Background(), defs, assignments)
}

// filterAssignment returns a new slice with the first matching assignment removed.
func filterAssignment(assignments []model.UserRoleAssignment, userID, role, tenantID string) []model.UserRoleAssignment {
	for i, a := range assignments {
		if a.UserID == userID && a.Role == role && a.TenantID == tenantID {
			return append(assignments[:i], assignments[i+1:]...)
		}
	}
	return assignments
}

// ReloadABACPolicy reloads secondary ABAC policies from the adapter.
func (m *AuthzManager) ReloadABACPolicy() error {
	if m.abacEnforcer == nil {
		return nil
	}
	return m.abacEnforcer.LoadPolicy()
}

// Close closes the underlying adapters and cancels event subscriptions.
func (m *AuthzManager) Close() error {
	for _, cancel := range m.cancelSubs {
		cancel()
	}
	m.cancelSubs = nil

	var err error
	if m.adapter != nil {
		if closeErr := m.adapter.Close(); closeErr != nil {
			err = closeErr
		}
	}
	if m.abacAdapter != nil {
		if closeErr := m.abacAdapter.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
	}
	return err
}
