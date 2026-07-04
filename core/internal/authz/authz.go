//The AuthZ module is simplified based on the KexCore IAM control architecture, retaining only RBAC and ABAC

package authz

import (
	"fmt"
	"sync"

	"github.com/casbin/casbin/v3"
	"github.com/casbin/casbin/v3/model"
	gormadapter "github.com/casbin/gorm-adapter/v3"
	"gorm.io/gorm"
)

type Mode string

const (
	ModeRBAC Mode = "rbac"
	ModeABAC Mode = "abac"
)

type AuthzManager struct {
	enforcer *casbin.SyncedEnforcer
	mode     Mode
	adapter  *gormadapter.Adapter
	initOnce sync.Once
	initDone bool
	initErr  error
}

func NewAuthzManager(db *gorm.DB, mode Mode) (*AuthzManager, error) {
	adapter, err := gormadapter.NewAdapterByDB(db)
	if err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to create adapter: %w", err)
	}

	modelStr, err := getModelString(mode)
	if err != nil {
		return nil, err
	}

	m, err := model.NewModelFromString(modelStr)
	if err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to create model: %w", err)
	}

	enforcer, err := casbin.NewSyncedEnforcer(m, adapter)
	if err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to create enforcer: %w", err)
	}

	enforcer.EnableAutoSave(true)

	if err := enforcer.LoadPolicy(); err != nil {
		return nil, fmt.Errorf("quotagate/authz: failed to load policy: %w", err)
	}

	return &AuthzManager{
		enforcer: enforcer,
		mode:     mode,
		adapter:  adapter,
	}, nil
}

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

		for _, policy := range defaultPolicies {
			if _, err := m.enforcer.AddPolicy(policy); err != nil {
				m.initErr = fmt.Errorf("quotagate/authz: failed to add default policy: %w", err)
				return
			}
		}

		for _, role := range defaultRoles {
			if _, err := m.enforcer.AddRoleForUser(role[0], role[1]); err != nil {
				m.initErr = fmt.Errorf("quotagate/authz: failed to add default role: %w", err)
				return
			}
		}

		m.initDone = true
	})

	if !m.initDone {
		return fmt.Errorf("quotagate/authz: initialization previously failed: %w", m.initErr)
	}
	return nil
}

func (m *AuthzManager) Enforce(rvals ...interface{}) (bool, error) {
	return m.enforcer.Enforce(rvals...)
}

func (m *AuthzManager) EnforceRBAC(sub, obj, act string) (bool, error) {
	return m.enforcer.Enforce(sub, obj, act)
}

func (m *AuthzManager) AddPolicy(sub, obj, act string) (bool, error) {
	if m.mode == ModeABAC {
		if err := validateABACSubRule(sub); err != nil {
			return false, err
		}
	}
	return m.enforcer.AddPolicy([]string{sub, obj, act})
}

func validateABACSubRule(subRule string) error {
	dangerousPatterns := []string{
		"true",
		"1==1",
		"1 == 1",
		"r.sub == r.sub",
		"r.obj == r.obj",
		"r.act == r.act",
	}

	for _, pattern := range dangerousPatterns {
		if subRule == pattern {
			return fmt.Errorf("quotagate/authz: ABAC sub_rule cannot be constant true expression")
		}
	}

	return nil
}

func (m *AuthzManager) RemovePolicy(sub, obj, act string) (bool, error) {
	return m.enforcer.RemovePolicy([]string{sub, obj, act})
}

func (m *AuthzManager) HasPolicy(sub, obj, act string) (bool, error) {
	return m.enforcer.HasPolicy(sub, obj, act)
}

func (m *AuthzManager) AddRoleForUser(user, role string) (bool, error) {
	return m.enforcer.AddRoleForUser(user, role)
}

func (m *AuthzManager) DeleteRoleForUser(user, role string) (bool, error) {
	return m.enforcer.DeleteRoleForUser(user, role)
}

func (m *AuthzManager) GetRolesForUser(user string) ([]string, error) {
	return m.enforcer.GetRolesForUser(user)
}

func (m *AuthzManager) GetUsersForRole(role string) ([]string, error) {
	return m.enforcer.GetUsersForRole(role)
}

func (m *AuthzManager) GetPolicy() ([][]string, error) {
	return m.enforcer.GetPolicy()
}

func (m *AuthzManager) GetAllRoles() ([]string, error) {
	return m.enforcer.GetAllRoles()
}

func (m *AuthzManager) ReloadPolicy() error {
	return m.enforcer.LoadPolicy()
}

func (m *AuthzManager) GetMode() Mode {
	return m.mode
}

func (m *AuthzManager) Close() error {
	if m.adapter != nil {
		return m.adapter.Close()
	}
	return nil
}

func getModelString(mode Mode) (string, error) {
	switch mode {
	case ModeRBAC:
		return RBACModel, nil
	case ModeABAC:
		return ABACModel, nil
	default:
		return "", fmt.Errorf("quotagate/authz: unsupported mode: %s", mode)
	}
}
