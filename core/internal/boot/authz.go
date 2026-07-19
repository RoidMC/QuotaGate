package boot

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"gorm.io/gorm"

	"github.com/roidmc/quotagate/internal/authz"
	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/repository"
	"github.com/roidmc/quotagate/internal/tenant"
	"github.com/roidmc/quotagate/internal/util/random"
)

func InitAuthz(db *gorm.DB, bus *event.EventBus, roleRepo *repository.RoleRepository, routeMetaRepo *repository.RouteMetaRepository) (*authz.AuthzManager, error) {
	opts := []authz.AuthzManagerOption{}
	if bus != nil {
		loader := func(ctx context.Context) ([]model.RoleDefinition, []model.UserRoleAssignment, error) {
			bypassCtx := tenant.Bypass(ctx)
			defs, err := roleRepo.ListRoles(bypassCtx)
			if err != nil {
				return nil, nil, fmt.Errorf("list roles: %w", err)
			}
			defs = mergeSystemRoles(defs)
			assignments, err := roleRepo.ListAllAssignments(bypassCtx)
			if err != nil {
				return nil, nil, fmt.Errorf("list assignments: %w", err)
			}
			return defs, assignments, nil
		}
		opts = append(opts, authz.WithEventBus(bus, random.MustUUIDString(), loader))
	}

	authzManager, err := authz.NewAuthzManager(db, opts...)
	if err != nil {
		return nil, fmt.Errorf("create authz manager: %w", err)
	}
	if err := authzManager.InitDefaultPolicies(); err != nil {
		return nil, fmt.Errorf("init default policies: %w", err)
	}

	// Seed system roles and default route metas on first startup.
	if err := seedSystemRoles(roleRepo); err != nil {
		return nil, fmt.Errorf("seed system roles: %w", err)
	}
	if err := seedDefaultRouteMetas(routeMetaRepo); err != nil {
		return nil, fmt.Errorf("seed default route metas: %w", err)
	}

	// Load all roles (system + tenant) and user-role assignments into the
	// in-memory grouping rules of the global enforcer.
	ctx := tenant.Bypass(context.Background())
	allRoles, err := roleRepo.ListRoles(ctx)
	if err != nil {
		return nil, fmt.Errorf("list roles for registry: %w", err)
	}
	allRoles = mergeSystemRoles(allRoles)

	assignments, err := roleRepo.ListAllAssignments(ctx)
	if err != nil {
		return nil, fmt.Errorf("list role assignments for registry: %w", err)
	}

	if err := authzManager.InitGroupingRelations(ctx, allRoles, assignments); err != nil {
		return nil, fmt.Errorf("init grouping relations: %w", err)
	}

	if err := authzManager.SubscribeToEvents(ctx); err != nil {
		return nil, fmt.Errorf("subscribe to authz events: %w", err)
	}

	slog.Info("quotagate/boot: Authz enabled (RBAC + ABAC, strict validation)")
	return authzManager, nil
}

func seedSystemRoles(roleRepo *repository.RoleRepository) error {
	ctx := tenant.Bypass(context.Background())
	for _, sys := range authz.DefaultSystemRoles() {
		_, err := roleRepo.GetRoleByName(ctx, sys.Name)
		if err == nil {
			continue
		}
		if !errors.Is(err, repository.ErrRoleNotFound) {
			return fmt.Errorf("check system role %q: %w", sys.Name, err)
		}
		if err := roleRepo.CreateRole(ctx, &sys); err != nil {
			return fmt.Errorf("create system role %q: %w", sys.Name, err)
		}
		slog.Info("quotagate/boot: seeded system role", "role", sys.Name)
	}
	return nil
}

func seedDefaultRouteMetas(routeMetaRepo *repository.RouteMetaRepository) error {
	ctx := tenant.Bypass(context.Background())
	for _, rm := range authz.DefaultRouteMetas() {
		meta := &model.RouteMeta{
			Method:   rm.Method,
			Path:     rm.Path,
			RuleType: model.RuleTypeRBAC,
		}
		if err := routeMetaRepo.CreateOrUpdate(ctx, meta); err != nil {
			return fmt.Errorf("seed route meta %s %s: %w", rm.Method, rm.Path, err)
		}
	}
	if err := routeMetaRepo.LoadCache(ctx); err != nil {
		return fmt.Errorf("load route meta cache: %w", err)
	}
	return nil
}

func mergeSystemRoles(dbRoles []model.RoleDefinition) []model.RoleDefinition {
	seen := make(map[string]bool)
	for _, r := range dbRoles {
		seen[r.Name] = true
	}
	for _, sys := range authz.DefaultSystemRoles() {
		if !seen[sys.Name] {
			dbRoles = append(dbRoles, sys)
		}
	}
	return dbRoles
}
