package tenant

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"gorm.io/gorm/schema"
)

var ErrTenantRequired = errors.New("quotagate/tenant: tenant_id is required for TenantAware model queries")

// ErrTenantIDImmutable is returned when an UPDATE/UPDATE_COLUMN attempt to
// modify the tenant_id column of a TenantAware model.
var ErrTenantIDImmutable = errors.New("quotagate/tenant: tenant_id column is immutable")

// ErrRawRequiresBypass is returned when a Raw().Scan() targets a TenantAware
// model. Raw SQL cannot be safely auto-filtered, so callers must explicitly
// opt in via Bypass() (and ideally supply a reason via BypassWithReason).
var ErrRawRequiresBypass = errors.New("quotagate/tenant: Raw queries on TenantAware models require explicit tenant.Bypass()")

type tenantKey struct{}
type bypassKey struct{}
type bypassReasonKey struct{}

// TenantAware marks a model as tenant-scoped.
// GORM callback automatically appends `tenant_id = ?` to every Query/Update/Delete.
// Create callback auto-fills the TenantID field from ctx and fail-louds if missing.
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
//
// For auditable bypass, prefer BypassWithReason.
func Bypass(ctx context.Context) context.Context {
	return context.WithValue(ctx, bypassKey{}, struct{}{})
}

// BypassWithReason is like Bypass but records a human-readable reason that can
// be retrieved by BypassReason(ctx) for audit logging.
func BypassWithReason(ctx context.Context, reason string) context.Context {
	ctx = Bypass(ctx)
	if reason == "" {
		return ctx
	}
	return context.WithValue(ctx, bypassReasonKey{}, reason)
}

// IsBypassed checks if the context has been explicitly marked to bypass tenant filtering.
func IsBypassed(ctx context.Context) bool {
	_, ok := ctx.Value(bypassKey{}).(struct{})
	return ok
}

// BypassReason returns the reason supplied to BypassWithReason, or empty string
// if Bypass was called without a reason. Useful for audit logging.
func BypassReason(ctx context.Context) string {
	v, _ := ctx.Value(bypassReasonKey{}).(string)
	return v
}

// SystemContext returns a context that bypasses tenant filtering.
// Use this for system-level operations (login, migrations, etc.).
func SystemContext(ctx context.Context) context.Context {
	return Bypass(ctx)
}

// FilterScope returns a GORM scope that injects the given ctx into the statement
// so the registered callback can apply tenant filtering.
//
// Use this when you need to switch ctx inside a Scopes chain, e.g.:
//
//	db.Scopes(tenant.FilterScope(ctx), OtherScope).Find(&items)
//
// For the common case, prefer db.WithContext(ctx).Find(&items) directly.
func FilterScope(ctx context.Context) func(*gorm.DB) *gorm.DB {
	return func(tx *gorm.DB) *gorm.DB {
		return tx.WithContext(ctx)
	}
}

func RegisterCallback(db *gorm.DB) error {
	cb := func(tx *gorm.DB) {
		ctx := tx.Statement.Context
		if ctx == nil {
			return
		}

		if !isTenantAwareType(tx) {
			return
		}

		applyTenantFilter(ctx, tx)
	}

	// Create: auto-fill TenantID from ctx and fail-loud if missing.
	// Runs before gorm:before_create so that user-supplied BeforeCreate hooks
	// see the populated TenantID.
	db.Callback().Create().Before("gorm:before_create").Register("quotagate:tenant_filter", func(tx *gorm.DB) {
		ctx := tx.Statement.Context
		if ctx == nil {
			return
		}
		if !isTenantAwareType(tx) {
			return
		}
		applyCreateTenantFilter(ctx, tx)
		// Reject OnConflict.DoUpdates that try to mutate tenant_id on upsert.
		// DoUpdates is a clause.Set nested inside the ON CONFLICT clause
		// expression; it runs at Create time, not Update time, so the Update
		// callback's detectTenantIDMutation doesn't see it.
		if tx.Error == nil {
			rejectOnConflictTenantIDMutation(tx)
		}
	})
	db.Callback().Query().Before("gorm:query").Register("quotagate:tenant_filter", cb)
	// Update: reject tenant_id mutation first, then apply tenant WHERE filter.
	db.Callback().Update().Before("gorm:update").Register("quotagate:tenant_filter", func(tx *gorm.DB) {
		ctx := tx.Statement.Context
		if ctx == nil {
			return
		}
		if !isTenantAwareType(tx) {
			return
		}
		// Detect SET tenant_id = ... (column update / map update) before
		// the WHERE filter runs, so attempts to "move" a row to a different
		// tenant are rejected regardless of the WHERE clause.
		detectTenantIDMutation(tx)
		if tx.Error != nil {
			return
		}
		applyTenantFilter(ctx, tx)
	})
	db.Callback().Delete().Before("gorm:delete").Register("quotagate:tenant_filter", cb)

	// Raw().Scan() and Raw().Rows() go through the Row callback (not Raw —
	// Raw is for Exec only). User-supplied SQL cannot be safely auto-filtered,
	// so we require explicit Bypass whenever the Dest is a TenantAware model.
	db.Callback().Row().Before("gorm:row").Register("quotagate:tenant_filter", func(tx *gorm.DB) {
		ctx := tx.Statement.Context
		if ctx == nil {
			return
		}
		if !isTenantAwareType(tx) {
			return
		}
		applyRawTenantFilter(ctx, tx)
	})
	// Raw().Exec() goes through the Raw callback. Same restriction applies:
	// if the user is executing raw SQL that touches a TenantAware table, they
	// must opt in via Bypass.
	db.Callback().Raw().Before("gorm:raw").Register("quotagate:tenant_filter", func(tx *gorm.DB) {
		ctx := tx.Statement.Context
		if ctx == nil {
			return
		}
		if !isTenantAwareType(tx) {
			return
		}
		applyRawTenantFilter(ctx, tx)
	})
	return nil
}

func isTenantAwareType(tx *gorm.DB) bool {
	if tx.Statement.Model != nil {
		if ta, ok := tx.Statement.Model.(TenantAware); ok && ta.TenantAware() {
			return true
		}
	}

	if tx.Statement.Dest != nil {
		destVal := reflect.ValueOf(tx.Statement.Dest)
		if destVal.Kind() == reflect.Ptr {
			destVal = destVal.Elem()
		}
		if destVal.Kind() == reflect.Slice {
			elemType := destVal.Type().Elem()
			if elemType.Kind() == reflect.Ptr {
				elemType = elemType.Elem()
			}
			if elemType.Kind() == reflect.Struct {
				elemVal := reflect.New(elemType).Elem()
				if elemVal.CanInterface() {
					if ta, ok := elemVal.Interface().(TenantAware); ok && ta.TenantAware() {
						return true
					}
				}
			}
		} else if destVal.CanInterface() {
			if ta, ok := destVal.Interface().(TenantAware); ok && ta.TenantAware() {
				return true
			}
		}
	}

	return false
}

func applyTenantFilter(ctx context.Context, tx *gorm.DB) {
	tenantID := FromContext(ctx)
	if tenantID != "" {
		tx.Statement.AddClause(clause.Where{Exprs: []clause.Expression{
			clause.Eq{Column: clause.Column{Name: "tenant_id"}, Value: tenantID},
		}})
		return
	}
	if IsBypassed(ctx) {
		return
	}
	tx.AddError(ErrTenantRequired)
}

// applyCreateTenantFilter enforces that tenant_id is sourced from ctx only.
// If ctx has no tenant_id and is not bypassed, fail-loud with ErrTenantRequired.
//
// Security note: any caller-supplied tenant_id on the model is OVERWRITTEN by
// the ctx tenant_id. Tenants cannot be cross-created by tampering with the
// model field — the only way to create a row in another tenant is via
// tenant.Bypass(), which is auditable via BypassReason(ctx).
func applyCreateTenantFilter(ctx context.Context, tx *gorm.DB) {
	tenantID := FromContext(ctx)
	if tenantID == "" {
		if IsBypassed(ctx) {
			return
		}
		tx.AddError(ErrTenantRequired)
		return
	}
	if tx.Statement.Schema == nil {
		return
	}
	field := tx.Statement.Schema.LookUpField("tenant_id")
	if field == nil {
		// TenantAware model without a tenant_id schema field — caller error.
		// Leave it; GORM itself will likely error on insert.
		return
	}

	// Path 1: struct / slice of structs — overwrite tenant_id on every row.
	setTenantIDOnCreates(tx, field, tenantID)

	// Path 2: map[string]any form (db.Create(map[string]any{...})) — GORM
	// does not parse Dest into Schema for the map form, so we mutate the map
	// directly. The user's tenant_id, if any, is overwritten.
	if m, ok := tx.Statement.Dest.(map[string]interface{}); ok {
		m["tenant_id"] = tenantID
	} else if mp, ok := tx.Statement.Dest.(*map[string]interface{}); ok && mp != nil {
		(*mp)["tenant_id"] = tenantID
	}
}

// setTenantIDOnCreates walks Statement.ReflectValue (struct or slice) and
// OVERWRITES tenant_id on every row with the ctx tenant_id. Caller-supplied
// values are intentionally clobbered — tenant_id must come from ctx, never
// from the model. Cross-tenant creation must go through Bypass().
func setTenantIDOnCreates(tx *gorm.DB, field *schema.Field, tenantID string) {
	rv := tx.Statement.ReflectValue
	switch rv.Kind() {
	case reflect.Slice:
		for i := 0; i < rv.Len(); i++ {
			elem := rv.Index(i)
			if elem.Kind() == reflect.Ptr {
				if elem.IsNil() {
					continue
				}
				elem = elem.Elem()
			}
			overwriteTenantID(tx, field, elem, tenantID)
		}
	case reflect.Struct:
		overwriteTenantID(tx, field, rv, tenantID)
	}
}

// overwriteTenantID unconditionally sets tenant_id. Errors are ignored
// because Set only fails on truly unaddressable values, which GORM itself
// would also fail to insert anyway.
func overwriteTenantID(tx *gorm.DB, field *schema.Field, rv reflect.Value, tenantID string) {
	_ = field.Set(tx.Statement.Context, rv, tenantID)
}

// applyRawTenantFilter handles Raw().Scan() — SQL is user-supplied so we can't
// safely inject WHERE. Require explicit Bypass for TenantAware Dests.
func applyRawTenantFilter(ctx context.Context, tx *gorm.DB) {
	if IsBypassed(ctx) {
		return
	}
	// If caller already wrote tenant_id into SQL we can't reliably detect it
	// (could be in JOIN/CTE/etc.). Safest is to require Bypass.
	tx.AddError(ErrRawRequiresBypass)
}

// SafeRawScan wraps db.Raw(sql, args...).Scan(dest) with an explicit TenantAware
// check on dest. Use this instead of the chainable Raw().Scan() form whenever
// the destination might be a TenantAware model — the chainable form does not
// expose Dest to the Row callback until after the SQL has executed, so the
// callback alone cannot enforce the bypass requirement.
//
// Behavior:
//   - If dest is not TenantAware, runs the query as-is.
//   - If dest is TenantAware and ctx is marked with Bypass(), runs the query
//     (caller is responsible for including `WHERE tenant_id = ?` in sql).
//   - If dest is TenantAware and ctx is not bypassed, returns ErrRawRequiresBypass.
//
// For the bypassed path, prefer BypassWithReason so the bypass is auditable.
func SafeRawScan(ctx context.Context, db *gorm.DB, sql string, args []interface{}, dest interface{}) error {
	if isTenantAwareDest(dest) && !IsBypassed(ctx) {
		return ErrRawRequiresBypass
	}
	return db.WithContext(ctx).Raw(sql, args...).Scan(dest).Error
}

// isTenantAwareDest mirrors isTenantAwareType but operates on Dest directly
// instead of pulling it out of tx.Statement. Used by SafeRawScan because
// the chainable Raw().Scan() form does not expose Dest to callbacks.
func isTenantAwareDest(dest interface{}) bool {
	if dest == nil {
		return false
	}
	dv := reflect.ValueOf(dest)
	if dv.Kind() == reflect.Ptr {
		if dv.IsNil() {
			return false
		}
		dv = dv.Elem()
	}
	if dv.Kind() == reflect.Slice {
		elemType := dv.Type().Elem()
		if elemType.Kind() == reflect.Ptr {
			elemType = elemType.Elem()
		}
		if elemType.Kind() == reflect.Struct {
			elemVal := reflect.New(elemType).Elem()
			if elemVal.CanInterface() {
				if ta, ok := elemVal.Interface().(TenantAware); ok && ta.TenantAware() {
					return true
				}
			}
		}
		return false
	}
	if dv.Kind() != reflect.Struct {
		return false
	}
	if dv.CanInterface() {
		if ta, ok := dv.Interface().(TenantAware); ok && ta.TenantAware() {
			return true
		}
	}
	return false
}

// detectTenantIDMutation inspects the Update statement for SET tenant_id
// attempts and rejects them. Called by the Update callback before gorm:update.
//
// Four forms are handled:
//  1. Update("tenant_id", v)         — Dest is map[string]any{"tenant_id": v}
//  2. Updates(map[string]any{...})    — Dest is map[string]any
//  3. Updates(struct{TenantID: x})   — Dest is a struct/struct ptr with non-zero tenant_id
//  4. Clauses(clause.Assignments(...).Update(...)) — SET clause contains tenant_id
//
// OnConflict.DoUpdates is handled separately by rejectOnConflictTenantIDMutation
// in the Create callback, since it runs at INSERT time.
func detectTenantIDMutation(tx *gorm.DB) {
	if tx.Statement == nil {
		return
	}

	// Map form (covers Update(col, val) and Updates(map[string]any)).
	if tx.Statement.Dest != nil {
		if assigns, ok := tx.Statement.Dest.(map[string]interface{}); ok {
			for k := range assigns {
				if strings.EqualFold(k, "tenant_id") {
					tx.AddError(ErrTenantIDImmutable)
					return
				}
			}
		} else {
			// Struct / *struct form. GORM's Updates(struct) only writes non-zero fields,
			// so a non-zero tenant_id means the caller is trying to mutate it.
			rv := reflect.ValueOf(tx.Statement.Dest)
			if rv.Kind() == reflect.Ptr {
				if rv.IsNil() {
					goto checkClauses
				}
				rv = rv.Elem()
			}
			if rv.Kind() == reflect.Struct && tx.Statement.Schema != nil {
				if field := tx.Statement.Schema.LookUpField("tenant_id"); field != nil {
					val, zero := field.ValueOf(tx.Statement.Context, rv)
					if !zero && val != nil {
						if s, ok := val.(string); !ok || s != "" {
							tx.AddError(ErrTenantIDImmutable)
							return
						}
					}
				}
			}
		}
	}

checkClauses:
	// clause.Set form: db.Clauses(clause.Assignments{...}).Updates(...) or
	// db.Clauses(clause.Assignments(...)).Update(...). The SET clause is stored
	// in Statement.Clauses under the "SET" key as a clause.Set (which is
	// []clause.Assignment).
	for _, c := range tx.Statement.Clauses {
		set, ok := c.Expression.(clause.Set)
		if !ok {
			continue
		}
		for _, assignment := range set {
			if strings.EqualFold(assignment.Column.Name, "tenant_id") {
				tx.AddError(ErrTenantIDImmutable)
				return
			}
		}
	}
}

// rejectOnConflictTenantIDMutation scans the ON CONFLICT clause for
// DoUpdates entries that would mutate tenant_id on upsert. This runs in
// the Create callback because INSERT...ON CONFLICT DO UPDATE executes at
// INSERT time, not Update time — detectTenantIDMutation doesn't see it.
//
// GORM v1.31.x does NOT have a Statement.Assigns field; OnConflict.DoUpdates
// is a clause.Set nested inside the OnConflict clause expression.
func rejectOnConflictTenantIDMutation(tx *gorm.DB) {
	if tx.Statement == nil {
		return
	}
	c, ok := tx.Statement.Clauses["ON CONFLICT"]
	if !ok {
		return
	}
	onConflict, ok := c.Expression.(clause.OnConflict)
	if !ok {
		return
	}
	// UpdateAll=true means "DO UPDATE SET <all columns>" — that includes
	// tenant_id. TenantAware models forbid this because tenant_id must be
	// immutable after Create. Callers must enumerate non-tenant_id columns
	// explicitly in DoUpdates.
	if onConflict.UpdateAll {
		tx.AddError(fmt.Errorf(
			"%w: ON CONFLICT DO UPDATE SET * (UpdateAll) is not allowed on TenantAware models "+
				"because it would mutate tenant_id; list specific columns in DoUpdates instead",
			ErrTenantIDImmutable,
		))
		return
	}
	for _, assignment := range onConflict.DoUpdates {
		if strings.EqualFold(assignment.Column.Name, "tenant_id") {
			tx.AddError(fmt.Errorf(
				"%w: ON CONFLICT DO UPDATE SET tenant_id is not allowed on TenantAware models "+
					"because tenant_id is immutable; remove tenant_id from DoUpdates",
				ErrTenantIDImmutable,
			))
			return
		}
	}
}
