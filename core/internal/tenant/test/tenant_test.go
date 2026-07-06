package tenant_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/roidmc/quotagate/internal/tenant"
)

type testTenantModel struct {
	ID       string `gorm:"primaryKey;size:36"`
	TenantID string `gorm:"column:tenant_id;size:36;not null"`
	Name     string `gorm:"size:128"`
}

func (testTenantModel) TableName() string { return "test_tenant_models" }
func (testTenantModel) TenantAware() bool { return true }

type testNonTenantModel struct {
	ID   string `gorm:"primaryKey;size:36"`
	Name string `gorm:"size:128"`
}

func (testNonTenantModel) TableName() string { return "test_non_tenant_models" }

func setupDB(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&testTenantModel{}, &testNonTenantModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	if err := tenant.RegisterCallback(db); err != nil {
		t.Fatalf("failed to register callback: %v", err)
	}
	return db, tenant.Bypass(context.Background())
}

type testParentModel struct {
	ID       string           `gorm:"primaryKey;size:36"`
	TenantID string           `gorm:"column:tenant_id;size:36;not null"`
	Name     string           `gorm:"size:128"`
	Children []testChildModel `gorm:"foreignKey:ParentID;references:ID"`
}

func (testParentModel) TableName() string { return "test_parent_models" }
func (testParentModel) TenantAware() bool { return true }

type testChildModel struct {
	ID       string `gorm:"primaryKey;size:36"`
	ParentID string `gorm:"column:parent_id;size:36;not null"`
	TenantID string `gorm:"column:tenant_id;size:36;not null"`
	Name     string `gorm:"size:128"`
}

func (testChildModel) TableName() string { return "test_child_models" }
func (testChildModel) TenantAware() bool { return true }

func setupDBWithPreload(t *testing.T) (*gorm.DB, context.Context) {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file::memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open sqlite: %v", err)
	}
	if err := db.AutoMigrate(&testParentModel{}, &testChildModel{}); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	if err := tenant.RegisterCallback(db); err != nil {
		t.Fatalf("failed to register callback: %v", err)
	}
	return db, tenant.Bypass(context.Background())
}

func TestContextOperations(t *testing.T) {
	ctx := context.Background()

	if tenant.FromContext(ctx) != "" {
		t.Error("empty context should return empty tenant ID")
	}

	ctx = tenant.WithTenant(ctx, "tenant-a")
	if tenant.FromContext(ctx) != "tenant-a" {
		t.Errorf("expected tenant-a, got %s", tenant.FromContext(ctx))
	}

	if tenant.IsBypassed(ctx) {
		t.Error("context with tenant should not be bypassed")
	}

	ctx = tenant.Bypass(ctx)
	if !tenant.IsBypassed(ctx) {
		t.Error("bypassed context should be marked as bypassed")
	}

	if tenant.FromContext(ctx) != "tenant-a" {
		t.Errorf("bypass should not clear tenant ID, got %s", tenant.FromContext(ctx))
	}

	ctx2 := context.Background()
	ctx2 = tenant.SystemContext(ctx2)
	if !tenant.IsBypassed(ctx2) {
		t.Error("SystemContext should mark context as bypassed")
	}
}

func TestQueryWithTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-a", Name: "item-a2"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "3", TenantID: "tenant-b", Name: "item-b1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var items []testTenantModel
	if err := db.WithContext(ctx).Find(&items).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}

	for _, item := range items {
		if item.TenantID != "tenant-a" {
			t.Errorf("unexpected tenant ID: %s", item.TenantID)
		}
	}
}

func TestFilterScopeWithTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-a", Name: "item-a2"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "3", TenantID: "tenant-b", Name: "item-b1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var items []testTenantModel
	if err := db.Scopes(tenant.FilterScope(ctx)).Find(&items).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

func TestFilterScopeBypass(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-b", Name: "item-b1"})

	ctx := tenant.Bypass(context.Background())

	var items []testTenantModel
	if err := db.Scopes(tenant.FilterScope(ctx)).Find(&items).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("bypass should return all items, got %d", len(items))
	}
}

func TestFilterScopeNoTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a1"})

	ctx := context.Background()

	var items []testTenantModel
	err := db.Scopes(tenant.FilterScope(ctx)).Find(&items).Error
	if !errors.Is(err, tenant.ErrTenantRequired) {
		t.Errorf("expected ErrTenantRequired, got %v", err)
	}
}

func TestFilterScopeNonTenantModel(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testNonTenantModel{ID: "1", Name: "item-1"})
	db.WithContext(bypassCtx).Create(&testNonTenantModel{ID: "2", Name: "item-2"})

	ctx := context.Background()

	var items []testNonTenantModel
	if err := db.Scopes(tenant.FilterScope(ctx)).Find(&items).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("non-tenant model should return all items, got %d", len(items))
	}
}

func TestCallbackQueryWithTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-a", Name: "item-a2"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "3", TenantID: "tenant-b", Name: "item-b1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var items []testTenantModel
	if err := db.WithContext(ctx).Find(&items).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("expected 2 items for tenant-a, got %d", len(items))
	}
}

func TestCallbackQueryBypass(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-b", Name: "item-b1"})

	ctx := tenant.Bypass(context.Background())

	var items []testTenantModel
	if err := db.WithContext(ctx).Find(&items).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("bypass should return all items, got %d", len(items))
	}
}

func TestCallbackQueryNoTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a1"})

	ctx := context.Background()

	var items []testTenantModel
	err := db.WithContext(ctx).Find(&items).Error
	if !errors.Is(err, tenant.ErrTenantRequired) {
		t.Errorf("expected ErrTenantRequired, got %v", err)
	}
}

func TestCallbackUpdateWithTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "original"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-b", Name: "original"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	result := db.WithContext(ctx).Model(&testTenantModel{}).Where("id = ?", "1").Update("name", "updated")
	if result.Error != nil {
		t.Fatalf("update failed: %v", result.Error)
	}

	if result.RowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", result.RowsAffected)
	}

	var item1 testTenantModel
	db.WithContext(bypassCtx).Where("id = ?", "1").First(&item1)
	if item1.Name != "updated" {
		t.Errorf("expected name 'updated', got %s", item1.Name)
	}

	var item2 testTenantModel
	db.WithContext(bypassCtx).Where("id = ?", "2").First(&item2)
	if item2.Name != "original" {
		t.Errorf("tenant-b item should not be updated, got %s", item2.Name)
	}
}

func TestCallbackUpdateNoTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "original"})

	ctx := context.Background()

	result := db.WithContext(ctx).Model(&testTenantModel{}).Where("id = ?", "1").Update("name", "updated")
	if !errors.Is(result.Error, tenant.ErrTenantRequired) {
		t.Errorf("expected ErrTenantRequired, got %v", result.Error)
	}
}

func TestCallbackDeleteWithTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-b", Name: "item-b"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	result := db.WithContext(ctx).Where("id = ?", "1").Delete(&testTenantModel{})
	if result.Error != nil {
		t.Fatalf("delete failed: %v", result.Error)
	}

	if result.RowsAffected != 1 {
		t.Errorf("expected 1 row deleted, got %d", result.RowsAffected)
	}

	var count int64
	db.WithContext(bypassCtx).Model(&testTenantModel{}).Count(&count)
	if count != 1 {
		t.Errorf("expected 1 item remaining, got %d", count)
	}
}

func TestCallbackDeleteNoTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a"})

	ctx := context.Background()

	result := db.WithContext(ctx).Where("id = ?", "1").Delete(&testTenantModel{})
	if !errors.Is(result.Error, tenant.ErrTenantRequired) {
		t.Errorf("expected ErrTenantRequired, got %v", result.Error)
	}
}

func TestCallbackNonTenantModel(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testNonTenantModel{ID: "1", Name: "item-1"})
	db.WithContext(bypassCtx).Create(&testNonTenantModel{ID: "2", Name: "item-2"})

	ctx := context.Background()

	var items []testNonTenantModel
	if err := db.WithContext(ctx).Find(&items).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if len(items) != 2 {
		t.Errorf("non-tenant model should return all items, got %d", len(items))
	}

	result := db.WithContext(ctx).Model(&testNonTenantModel{}).Where("id = ?", "1").Update("name", "updated")
	if result.Error != nil {
		t.Fatalf("update failed: %v", result.Error)
	}

	result = db.WithContext(ctx).Where("id = ?", "1").Delete(&testNonTenantModel{})
	if result.Error != nil {
		t.Fatalf("delete failed: %v", result.Error)
	}
}

func TestCallbackFindByIDWithTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-b", Name: "item-b"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var item testTenantModel
	if err := db.WithContext(ctx).Where("id = ?", "1").First(&item).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}

	if item.TenantID != "tenant-a" {
		t.Errorf("expected tenant-a, got %s", item.TenantID)
	}
}

func TestCallbackFindByIDWrongTenant(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "item-a"})

	ctx := tenant.WithTenant(context.Background(), "tenant-b")

	var item testTenantModel
	result := db.WithContext(ctx).Where("id = ?", "1").First(&item)
	if result.Error != gorm.ErrRecordNotFound {
		t.Errorf("expected record not found for wrong tenant, got %v", result.Error)
	}
}

// Count with Model should apply tenant filter
func TestCallbackCountWithModel(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-a", Name: "a2"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "3", TenantID: "tenant-b", Name: "b1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var count int64
	if err := db.WithContext(ctx).Model(&testTenantModel{}).Count(&count).Error; err != nil {
		t.Fatalf("count failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected count 2 for tenant-a, got %d", count)
	}
}

// Pluck should apply tenant filter
func TestCallbackPluck(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-a", Name: "a2"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "3", TenantID: "tenant-b", Name: "b1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var ids []string
	if err := db.WithContext(ctx).Model(&testTenantModel{}).Pluck("id", &ids).Error; err != nil {
		t.Fatalf("pluck failed: %v", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 ids for tenant-a, got %d", len(ids))
	}
}

// Updates with map should be tenant-scoped
func TestCallbackUpdatesWithMap(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-b", Name: "b1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	result := db.WithContext(ctx).Model(&testTenantModel{}).Where("id = ?", "1").Updates(map[string]interface{}{"name": "updated"})
	if result.Error != nil {
		t.Fatalf("update failed: %v", result.Error)
	}
	if result.RowsAffected != 1 {
		t.Errorf("expected 1 row affected, got %d", result.RowsAffected)
	}

	// Cross-tenant update should affect 0 rows
	result = db.WithContext(ctx).Model(&testTenantModel{}).Where("id = ?", "2").Updates(map[string]interface{}{"name": "hacked"})
	if result.Error != nil {
		t.Fatalf("update failed: %v", result.Error)
	}
	if result.RowsAffected != 0 {
		t.Errorf("cross-tenant update should affect 0 rows, got %d", result.RowsAffected)
	}
}

// Table() + Find should still detect TenantAware from Dest
func TestCallbackTableWithDest(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-b", Name: "b1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var items []testTenantModel
	if err := db.WithContext(ctx).Table("test_tenant_models").Find(&items).Error; err != nil {
		t.Fatalf("query failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item for tenant-a, got %d", len(items))
	}
}

// Raw + Scan with Model() set: callback can detect TenantAware and require Bypass.
func TestCallbackRawScan(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-b", Name: "b1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	// Model(...) + Raw + Scan: Model is set, callback fires, fail-loud.
	var items []testTenantModel
	err := db.WithContext(ctx).Model(&testTenantModel{}).Raw("SELECT * FROM test_tenant_models").Scan(&items).Error
	if !errors.Is(err, tenant.ErrRawRequiresBypass) {
		t.Fatalf("expected ErrRawRequiresBypass, got %v", err)
	}

	// With explicit Bypass + manual tenant_id in SQL, raw scan works.
	bypassWithTenant := tenant.Bypass(ctx)
	if err := db.WithContext(bypassWithTenant).Model(&testTenantModel{}).Raw("SELECT * FROM test_tenant_models WHERE tenant_id = ?", "tenant-a").Scan(&items).Error; err != nil {
		t.Fatalf("raw scan with bypass failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item for tenant-a via bypass, got %d", len(items))
	}
}

// Raw().Scan() WITHOUT Model() cannot be auto-detected by the callback
// (GORM doesn't expose Dest to the Row callback at that point). Callers must
// use the SafeRawScan helper, which inspects Dest directly.
func TestSafeRawScanRejectsTenantAwareWithoutBypass(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})
	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "2", TenantID: "tenant-b", Name: "b1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var items []testTenantModel
	err := tenant.SafeRawScan(ctx, db, "SELECT * FROM test_tenant_models", nil, &items)
	if !errors.Is(err, tenant.ErrRawRequiresBypass) {
		t.Fatalf("expected ErrRawRequiresBypass, got %v", err)
	}

	// With Bypass + manual tenant_id, SafeRawScan returns the filtered rows.
	bypassWithTenant := tenant.BypassWithReason(ctx, "ad-hoc report: cross-tenant aggregate")
	if err := tenant.SafeRawScan(bypassWithTenant, db, "SELECT * FROM test_tenant_models WHERE tenant_id = ?", []interface{}{"tenant-a"}, &items); err != nil {
		t.Fatalf("safe raw scan failed: %v", err)
	}
	if len(items) != 1 {
		t.Errorf("expected 1 item, got %d", len(items))
	}
}

// Raw + Scan on a non-TenantAware Dest should be unaffected.
func TestCallbackRawScanNonTenantModel(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testNonTenantModel{ID: "1", Name: "n1"})
	db.WithContext(bypassCtx).Create(&testNonTenantModel{ID: "2", Name: "n2"})

	ctx := context.Background() // no tenant, no bypass

	var items []testNonTenantModel
	if err := tenant.SafeRawScan(ctx, db, "SELECT * FROM test_non_tenant_models", nil, &items); err != nil {
		t.Fatalf("raw scan failed: %v", err)
	}
	if len(items) != 2 {
		t.Errorf("expected 2 items, got %d", len(items))
	}
}

// Transaction should inherit ctx from tx.WithContext
func TestCallbackTransaction(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	err := db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var item testTenantModel
		if err := tx.WithContext(ctx).Where("id = ?", "1").First(&item).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		t.Fatalf("transaction failed: %v", err)
	}
}

// Transaction WITHOUT WithContext inside should fail-loud
func TestCallbackTransactionMissingCtx(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})

	err := db.Transaction(func(tx *gorm.DB) error {
		var item testTenantModel
		// Note: no WithContext(ctx) here — ctx is lost inside transaction
		return tx.Where("id = ?", "1").First(&item).Error
	})
	if !errors.Is(err, tenant.ErrTenantRequired) {
		t.Errorf("expected ErrTenantRequired when ctx missing in tx, got %v", err)
	}
}

// FirstOrCreate should be tenant-scoped on the query part
func TestCallbackFirstOrCreate(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var item testTenantModel
	result := db.WithContext(ctx).Where(testTenantModel{ID: "1"}).FirstOrCreate(&item)
	if result.Error != nil {
		t.Fatalf("FirstOrCreate failed: %v", result.Error)
	}
	if item.TenantID != "tenant-a" {
		t.Errorf("expected tenant-a, got %s", item.TenantID)
	}
}

// Create: ctx with tenant_id auto-fills TenantID on the model.
func TestCreateAutoFillsTenantID(t *testing.T) {
	db, _ := setupDB(t)

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	// Note: TenantID intentionally left empty — callback should fill it.
	item := &testTenantModel{ID: "1", Name: "auto-filled"}
	if err := db.WithContext(ctx).Create(item).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if item.TenantID != "tenant-a" {
		t.Errorf("expected TenantID auto-filled to tenant-a, got %s", item.TenantID)
	}

	// Verify it was actually persisted with the right tenant_id.
	bypassCtx := tenant.Bypass(context.Background())
	var fetched testTenantModel
	if err := db.WithContext(bypassCtx).Where("id = ?", "1").First(&fetched).Error; err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if fetched.TenantID != "tenant-a" {
		t.Errorf("persisted TenantID = %s, want tenant-a", fetched.TenantID)
	}
}

// Create: ctx without tenant_id and without Bypass should fail-loud.
func TestCreateNoTenantFailsLoud(t *testing.T) {
	db, _ := setupDB(t)

	ctx := context.Background()
	err := db.WithContext(ctx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "x"}).Error
	if !errors.Is(err, tenant.ErrTenantRequired) {
		t.Errorf("expected ErrTenantRequired, got %v", err)
	}
}

// Create: batch insert should fill TenantID on every row.
func TestCreateBatchAutoFillsTenantID(t *testing.T) {
	db, _ := setupDB(t)

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	items := []*testTenantModel{
		{ID: "1", Name: "a1"},
		{ID: "2", Name: "a2"},
		{ID: "3", Name: "a3"},
	}
	if err := db.WithContext(ctx).Create(&items).Error; err != nil {
		t.Fatalf("create batch failed: %v", err)
	}
	for _, it := range items {
		if it.TenantID != "tenant-a" {
			t.Errorf("row %s: TenantID = %s, want tenant-a", it.ID, it.TenantID)
		}
	}
}

// Create: caller-supplied TenantID is OVERWRITTEN by ctx tenant_id.
// tenant_id is sourced exclusively from ctx — the model field is ignored.
// Cross-tenant creation must go through Bypass() (which is auditable).
func TestCreateOverwritesExplicitTenantID(t *testing.T) {
	db, _ := setupDB(t)

	// Caller tries to create a row in tenant-a while ctx is for tenant-b.
	// The ctx wins; the row is created in tenant-b.
	ctx := tenant.WithTenant(context.Background(), "tenant-b")
	item := &testTenantModel{ID: "1", TenantID: "tenant-a", Name: "explicit"}
	if err := db.WithContext(ctx).Create(item).Error; err != nil {
		t.Fatalf("create failed: %v", err)
	}
	if item.TenantID != "tenant-b" {
		t.Errorf("expected ctx tenant-b to overwrite explicit tenant-a, got %s", item.TenantID)
	}

	// Verify persisted value.
	bypassCtx := tenant.Bypass(context.Background())
	var fetched testTenantModel
	if err := db.WithContext(bypassCtx).Where("id = ?", "1").First(&fetched).Error; err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if fetched.TenantID != "tenant-b" {
		t.Errorf("persisted TenantID = %s, want tenant-b (ctx)", fetched.TenantID)
	}
}

// Update: SET tenant_id = ... (column form) is rejected.
func TestUpdateRejectsTenantIDColumnMutation(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")
	err := db.WithContext(ctx).Model(&testTenantModel{}).Where("id = ?", "1").Update("tenant_id", "tenant-b").Error
	if !errors.Is(err, tenant.ErrTenantIDImmutable) {
		t.Errorf("expected ErrTenantIDImmutable, got %v", err)
	}

	// Verify the row was not moved.
	var fetched testTenantModel
	db.WithContext(bypassCtx).Where("id = ?", "1").First(&fetched)
	if fetched.TenantID != "tenant-a" {
		t.Errorf("tenant_id was mutated to %s despite rejection", fetched.TenantID)
	}
}

// Update: Updates(map) containing tenant_id is rejected.
func TestUpdateRejectsTenantIDMapMutation(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")
	err := db.WithContext(ctx).Model(&testTenantModel{}).Where("id = ?", "1").Updates(map[string]interface{}{
		"name":      "updated",
		"tenant_id": "tenant-b",
	}).Error
	if !errors.Is(err, tenant.ErrTenantIDImmutable) {
		t.Errorf("expected ErrTenantIDImmutable, got %v", err)
	}
}

// Update: Updates(struct) with non-zero tenant_id is silently dropped by GORM
// (struct Updates only update non-zero fields), so the field effectively can't
// be moved via a struct form. We don't need to enforce this case — GORM's
// default behavior is fine. This test documents the contract.
func TestUpdateStructFormCannotMutateTenantID(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")
	// Struct Updates with tenant_id="tenant-b" set — GORM will update it
	// (string non-zero). Our callback should still reject via Selects path
	// because GORM adds tenant_id to Statement.Selects for struct Updates.
	err := db.WithContext(ctx).Model(&testTenantModel{}).Where("id = ?", "1").Updates(testTenantModel{TenantID: "tenant-b", Name: "updated"}).Error
	if !errors.Is(err, tenant.ErrTenantIDImmutable) {
		t.Errorf("expected ErrTenantIDImmutable, got %v", err)
	}
}

// Create: map[string]any form should auto-fill tenant_id.
func TestCreateMapAutoFillsTenantID(t *testing.T) {
	db, _ := setupDB(t)

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	err := db.WithContext(ctx).Model(&testTenantModel{}).Create(map[string]interface{}{
		"id":   "1",
		"name": "via-map",
	}).Error
	if err != nil {
		t.Fatalf("create map failed: %v", err)
	}

	bypassCtx := tenant.Bypass(context.Background())
	var fetched testTenantModel
	if err := db.WithContext(bypassCtx).Where("id = ?", "1").First(&fetched).Error; err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if fetched.TenantID != "tenant-a" {
		t.Errorf("persisted TenantID = %s, want tenant-a", fetched.TenantID)
	}
}

// Create: map[string]any form with caller-supplied tenant_id is OVERWRITTEN.
func TestCreateMapOverwritesExplicitTenantID(t *testing.T) {
	db, _ := setupDB(t)

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	err := db.WithContext(ctx).Model(&testTenantModel{}).Create(map[string]interface{}{
		"id":        "1",
		"name":      "via-map",
		"tenant_id": "tenant-b", // attacker tries to set cross-tenant
	}).Error
	if err != nil {
		t.Fatalf("create map failed: %v", err)
	}

	bypassCtx := tenant.Bypass(context.Background())
	var fetched testTenantModel
	if err := db.WithContext(bypassCtx).Where("id = ?", "1").First(&fetched).Error; err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if fetched.TenantID != "tenant-a" {
		t.Errorf("persisted TenantID = %s, want tenant-a (caller value should be overwritten)", fetched.TenantID)
	}
}

// Update: SET tenant_id via Clauses(clause.Assignments) is rejected.
func TestUpdateRejectsClauseSetTenantIDMutation(t *testing.T) {
	db, bypassCtx := setupDB(t)

	db.WithContext(bypassCtx).Create(&testTenantModel{ID: "1", TenantID: "tenant-a", Name: "a1"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")
	err := db.WithContext(ctx).Model(&testTenantModel{}).Where("id = ?", "1").Clauses(
		clause.Assignments(map[string]interface{}{"tenant_id": "tenant-b"}),
	).Update("name", "x").Error
	if !errors.Is(err, tenant.ErrTenantIDImmutable) {
		t.Errorf("expected ErrTenantIDImmutable, got %v", err)
	}

	// Verify the row was not moved.
	var fetched testTenantModel
	db.WithContext(bypassCtx).Where("id = ?", "1").First(&fetched)
	if fetched.TenantID != "tenant-a" {
		t.Errorf("tenant_id was mutated to %s despite rejection", fetched.TenantID)
	}
}

// Create + OnConflict with DoUpdates listing tenant_id is rejected.
// This is the upsert path — INSERT...ON CONFLICT DO UPDATE SET tenant_id = EXCLUDED.tenant_id
// would let an attacker overwrite an existing row's tenant_id on conflict.
// The error message must guide developers to the fix.
func TestCreateRejectsOnConflictDoUpdatesTenantID(t *testing.T) {
	db, _ := setupDB(t)

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"tenant_id", "name"}),
	}).Create(&testTenantModel{ID: "1", Name: "x"}).Error
	if !errors.Is(err, tenant.ErrTenantIDImmutable) {
		t.Errorf("expected ErrTenantIDImmutable, got %v", err)
	}
	if !strings.Contains(err.Error(), "remove tenant_id from DoUpdates") {
		t.Errorf("error message should guide developer to remove tenant_id from DoUpdates, got: %s", err.Error())
	}
}

// Create + OnConflict with UpdateAll=true is rejected.
// UpdateAll means "DO UPDATE SET <all columns>", which includes tenant_id.
// The error message must explain why UpdateAll is forbidden and how to fix it.
func TestCreateRejectsOnConflictUpdateAll(t *testing.T) {
	db, _ := setupDB(t)

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	err := db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		UpdateAll: true,
	}).Create(&testTenantModel{ID: "1", Name: "x"}).Error
	if !errors.Is(err, tenant.ErrTenantIDImmutable) {
		t.Errorf("expected ErrTenantIDImmutable, got %v", err)
	}
	if !strings.Contains(err.Error(), "list specific columns in DoUpdates instead") {
		t.Errorf("error message should guide developer to list specific columns, got: %s", err.Error())
	}
}

// Create + OnConflict that does NOT touch tenant_id is allowed.
// This is the legitimate upsert path — only non-tenant_id columns are updated.
func TestCreateAllowsOnConflictWithoutTenantID(t *testing.T) {
	db, _ := setupDB(t)

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	// First insert.
	err := db.WithContext(ctx).Create(&testTenantModel{ID: "1", Name: "original"}).Error
	if err != nil {
		t.Fatalf("first create failed: %v", err)
	}

	// Upsert with OnConflict on name only — should succeed.
	err = db.WithContext(ctx).Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns([]string{"name"}),
	}).Create(&testTenantModel{ID: "1", Name: "updated"}).Error
	if err != nil {
		t.Fatalf("upsert failed: %v", err)
	}

	// Verify the row is still in tenant-a and name was updated.
	bypassCtx := tenant.Bypass(context.Background())
	var fetched testTenantModel
	if err := db.WithContext(bypassCtx).Where("id = ?", "1").First(&fetched).Error; err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if fetched.TenantID != "tenant-a" {
		t.Errorf("tenant_id = %s, want tenant-a", fetched.TenantID)
	}
	if fetched.Name != "updated" {
		t.Errorf("name = %s, want updated", fetched.Name)
	}
}

// BypassWithReason stores the reason retrievable via BypassReason.
func TestBypassWithReason(t *testing.T) {
	ctx := context.Background()
	if tenant.BypassReason(ctx) != "" {
		t.Error("empty context should have empty bypass reason")
	}

	ctx = tenant.BypassWithReason(ctx, "system migration: backfill users")
	if !tenant.IsBypassed(ctx) {
		t.Error("BypassWithReason should also mark bypassed")
	}
	if got := tenant.BypassReason(ctx); got != "system migration: backfill users" {
		t.Errorf("expected reason, got %q", got)
	}

	// Bypass() without reason returns empty reason but still bypasses.
	ctx2 := tenant.Bypass(context.Background())
	if !tenant.IsBypassed(ctx2) {
		t.Error("Bypass should mark bypassed")
	}
	if got := tenant.BypassReason(ctx2); got != "" {
		t.Errorf("Bypass without reason should return empty, got %q", got)
	}
}

// Preload: related TenantAware rows should be filtered by the parent ctx's tenant.
// This is a regression test for the concern that Preload's sub-queries might
// bypass the tenant filter.
func TestPreloadFiltersRelatedTenantModel(t *testing.T) {
	db, bypassCtx := setupDBWithPreload(t)

	// tenant-a parent with 2 children (both tenant-a)
	db.WithContext(bypassCtx).Create(&testParentModel{ID: "p1", TenantID: "tenant-a", Name: "pa"})
	db.WithContext(bypassCtx).Create(&testChildModel{ID: "c1", ParentID: "p1", TenantID: "tenant-a", Name: "ca1"})
	db.WithContext(bypassCtx).Create(&testChildModel{ID: "c2", ParentID: "p1", TenantID: "tenant-a", Name: "ca2"})

	// tenant-a parent with 1 child that belongs to tenant-b (cross-tenant leakage attempt)
	db.WithContext(bypassCtx).Create(&testParentModel{ID: "p2", TenantID: "tenant-a", Name: "pa2"})
	db.WithContext(bypassCtx).Create(&testChildModel{ID: "c3", ParentID: "p2", TenantID: "tenant-b", Name: "cb-leak"})

	// tenant-b parent (should not be visible to tenant-a)
	db.WithContext(bypassCtx).Create(&testParentModel{ID: "p3", TenantID: "tenant-b", Name: "pb"})

	ctx := tenant.WithTenant(context.Background(), "tenant-a")

	var parents []testParentModel
	if err := db.WithContext(ctx).Preload("Children").Find(&parents).Error; err != nil {
		t.Fatalf("preload find failed: %v", err)
	}

	if len(parents) != 2 {
		t.Fatalf("expected 2 parents for tenant-a, got %d", len(parents))
	}

	totalChildren := 0
	for _, p := range parents {
		for _, c := range p.Children {
			totalChildren++
			if c.TenantID != "tenant-a" {
				t.Errorf("child %s of parent %s has tenant_id %s, want tenant-a (LEAK)", c.ID, p.ID, c.TenantID)
			}
		}
	}
	// p1 has 2 children (both tenant-a), p2's only child is tenant-b and should be filtered out.
	if totalChildren != 2 {
		t.Errorf("expected 2 children total, got %d (cross-tenant leak via Preload)", totalChildren)
	}
}
