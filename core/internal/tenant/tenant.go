package tenant

import (
	"context"
	"errors"
	"reflect"

	"gorm.io/gorm"
)

var ErrTenantRequired = errors.New("quotagate/tenant: tenant_id is required for TenantAware model queries")

type tenantKey struct{}
type bypassKey struct{}

// TenantAware marks a model as tenant-scoped.
// GORM callback automatically appends `tenant_id = ?` to every query.
// If the model implements TenantAware but context lacks tenant_id and is not
// marked with Bypass(), the query will fail with ErrTenantRequired.
type TenantAware interface {
	TenantAware() bool
}

// WithTenant injects a tenant id into context.
func WithTenant(ctx context.Context, tenantID string) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenantID)
}

// FromContext extracts the tenant id from context.
// Returns empty string if not set.
func FromContext(ctx context.Context) string {
	v, _ := ctx.Value(tenantKey{}).(string)
	return v
}

// Bypass marks the context as intentionally bypassing tenant filtering.
// Use this only for system-level operations that legitimately need cross-tenant access
// (e.g., login by email, system admin operations).
func Bypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, bypassKey{}, struct{}{})
}

// IsBypassed checks if the context has been explicitly marked to bypass tenant filtering.
func IsBypassed(ctx context.Context) bool {
	_, ok := ctx.Value(bypassKey{}).(struct{})
	return ok
}

// SystemContext returns a context that bypasses tenant filtering.
// Use this for system-level operations (login, migrations, etc.).
func SystemContext(ctx context.Context) context.Context {
	return Bypass(ctx)
}

// FilterScope is a GORM scope that appends `tenant_id = ?` when
// the current query model implements TenantAware and the context
// carries a tenant id. If context has neither tenant_id nor bypass,
// it returns an error.
func FilterScope(ctx context.Context) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		if tx.Statement.Model == nil {
			return tx
		}
		if ta, ok := tx.Statement.Model.(TenantAware); !ok || !ta.TenantAware() {
			return tx
		}

		tenantID := FromContext(ctx)
		if tenantID != "" {
			return tx.Where("tenant_id = ?", tenantID)
		}
		if IsBypassed(ctx) {
			return tx
		}
		tx.AddError(ErrTenantRequired)
		return tx
	}
}

// RegisterCallback installs BeforeQuery/BeforeUpdate/BeforeDelete
// callbacks on the given gorm.DB. For any model implementing TenantAware:
//   - If context has tenant_id → append `WHERE tenant_id = ?`
//   - If context is bypassed → skip filtering (system operations)
//   - If neither → fail with ErrTenantRequired (fail-loud)
func RegisterCallback(db *gorm.DB) error {
	cb := func(tx *gorm.DB) {
		ctx := tx.Statement.Context
		if ctx == nil {
			return
		}

		if tx.Statement.Model != nil {
			if ta, ok := tx.Statement.Model.(TenantAware); ok && ta.TenantAware() {
				applyTenantFilter(ctx, tx)
				return
			}
		}

		if tx.Statement.Dest != nil {
			destVal := reflect.ValueOf(tx.Statement.Dest)
			if destVal.Kind() == reflect.Ptr {
				destVal = destVal.Elem()
			}
			if destVal.Kind() == reflect.Slice && destVal.Len() > 0 {
				elemVal := destVal.Index(0)
				if elemVal.Kind() == reflect.Ptr {
					elemVal = elemVal.Elem()
				}
				if elemVal.CanInterface() {
					if ta, ok := elemVal.Interface().(TenantAware); ok && ta.TenantAware() {
						applyTenantFilter(ctx, tx)
						return
					}
				}
			} else if destVal.CanInterface() {
				if ta, ok := destVal.Interface().(TenantAware); ok && ta.TenantAware() {
					applyTenantFilter(ctx, tx)
					return
				}
			}
		}
	}

	db.Callback().Query().Before("gorm:query").Register("quotagate:tenant_filter", cb)
	db.Callback().Update().Before("gorm:update").Register("quotagate:tenant_filter", cb)
	db.Callback().Delete().Before("gorm:delete").Register("quotagate:tenant_filter", cb)
	return nil
}

func applyTenantFilter(ctx context.Context, tx *gorm.DB) {
	tenantID := FromContext(ctx)
	if tenantID != "" {
		tx.Where("tenant_id = ?", tenantID)
		return
	}
	if IsBypassed(ctx) {
		return
	}
	tx.AddError(ErrTenantRequired)
}
