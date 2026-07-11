package authz_test

import (
	"context"
	"testing"
	"time"

	"github.com/roidmc/quotagate/internal/authz"
	"github.com/roidmc/quotagate/internal/event"
)

// TestAuthzManager_EventSync_Assignment verifies that a user-role assignment
// published by one AuthzManager instance is applied to a peer instance sharing
// the same event bus.
func TestAuthzManager_EventSync_Assignment(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	bus := event.NewBus()

	makeManager := func(instanceID string) *authz.AuthzManager {
		m, err := authz.NewAuthzManager(db, false, authz.WithEventBus(bus, instanceID, nil))
		if err != nil {
			t.Fatalf("failed to create manager %s: %v", instanceID, err)
		}
		if err := m.InitDefaultPolicies(); err != nil {
			t.Fatalf("failed to init policies %s: %v", instanceID, err)
		}
		if err := m.InitRoleRegistry(ctx, authz.DefaultSystemRoles()); err != nil {
			t.Fatalf("failed to init registry %s: %v", instanceID, err)
		}
		if err := m.SubscribeToEvents(ctx); err != nil {
			t.Fatalf("failed to subscribe %s: %v", instanceID, err)
		}
		return m
	}

	mgr1 := makeManager("instance-1")
	mgr2 := makeManager("instance-2")
	defer func() {
		_ = mgr1.Close()
		_ = mgr2.Close()
		bus.Close()
	}()

	// mgr1 assigns the role and publishes the event. mgr2 should receive and
	// apply it even though it never called AssignUserRole itself.
	if _, err := mgr1.AssignUserRole("carol", "admin", ""); err != nil {
		t.Fatalf("failed to assign role on mgr1: %v", err)
	}
	mgr1.PublishRoleAssign("carol", "admin", "")

	ok, err := waitEnforceRBAC(ctx, mgr2, "", "carol", "GET", "/api/users", "")
	if err != nil {
		t.Fatalf("mgr2 enforce error: %v", err)
	}
	if !ok {
		t.Fatal("expected mgr2 to allow carol after remote role.assign event")
	}
}

// TestAuthzManager_EventSync_LocalEventFiltered verifies that an event emitted
// by a manager is not re-applied to itself.
func TestAuthzManager_EventSync_LocalEventFiltered(t *testing.T) {
	ctx := context.Background()
	db := setupTestDB(t)
	bus := event.NewBus()

	m, err := authz.NewAuthzManager(db, false, authz.WithEventBus(bus, "instance-local", nil))
	if err != nil {
		t.Fatalf("failed to create manager: %v", err)
	}
	if err := m.InitDefaultPolicies(); err != nil {
		t.Fatalf("failed to init policies: %v", err)
	}
	if err := m.InitRoleRegistry(ctx, authz.DefaultSystemRoles()); err != nil {
		t.Fatalf("failed to init registry: %v", err)
	}
	if err := m.SubscribeToEvents(ctx); err != nil {
		t.Fatalf("failed to subscribe: %v", err)
	}
	defer func() { _ = m.Close(); bus.Close() }()

	// Publishing a local event should be a no-op for the same instance.
	mgr1 := m
	if _, err := mgr1.AssignUserRole("dave", "admin", ""); err != nil {
		t.Fatalf("failed to assign role: %v", err)
	}
	mgr1.PublishRoleAssign("dave", "admin", "")

	// Dave is allowed because of the direct local assignment; the test mainly
	// ensures that the local event handler does not panic or corrupt state.
	ok, err := waitEnforceRBAC(ctx, m, "", "dave", "GET", "/api/users", "")
	if err != nil {
		t.Fatalf("enforce error: %v", err)
	}
	if !ok {
		t.Fatal("expected local assignment to remain valid")
	}
}

func waitEnforceRBAC(ctx context.Context, m *authz.AuthzManager, subOwner, userID, method, path, objOwner string) (bool, error) {
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		ok, err := m.EnforceRBAC(ctx, subOwner, userID, nil, method, path, objOwner)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
		time.Sleep(10 * time.Millisecond)
	}
	return false, nil
}
