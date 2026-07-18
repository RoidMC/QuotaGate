package tx_test

import (
	"context"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/roidmc/quotagate/internal/util/tx"
	"gorm.io/gorm"
)

func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	return db
}

func TestWithTxNil(t *testing.T) {
	parent := context.Background()
	got := tx.WithTx(parent, nil)
	if got != parent {
		t.Error("WithTx(parent, nil) should return the parent context unchanged")
	}

	// A context produced with a nil tx must not carry a transaction.
	if _, ok := tx.TxFromContext(got); ok {
		t.Error("TxFromContext should return false when no tx was attached")
	}
}

func TestWithTxAndTxFromContext(t *testing.T) {
	db := newTestDB(t)
	ctx := tx.WithTx(context.Background(), db)

	got, ok := tx.TxFromContext(ctx)
	if !ok {
		t.Fatal("TxFromContext should return true after WithTx")
	}
	if got != db {
		t.Error("TxFromContext returned a different *gorm.DB than the one attached")
	}
}

func TestTxFromContextNilContext(t *testing.T) {
	got, ok := tx.TxFromContext(nil)
	if ok {
		t.Error("TxFromContext(nil) should return ok=false")
	}
	if got != nil {
		t.Error("TxFromContext(nil) should return a nil *gorm.DB")
	}
}

func TestTxFromContextWithoutTx(t *testing.T) {
	got, ok := tx.TxFromContext(context.Background())
	if ok {
		t.Error("TxFromContext should return false for a context without a tx")
	}
	if got != nil {
		t.Error("TxFromContext should return a nil *gorm.DB when none is attached")
	}
}

func TestDBFallsBackToProvidedHandle(t *testing.T) {
	db := newTestDB(t)
	if err := db.Exec("CREATE TABLE fallback_marker (id INTEGER)").Error; err != nil {
		t.Fatalf("failed to create marker: %v", err)
	}

	got := tx.DB(context.Background(), db)
	if got == nil {
		t.Fatal("DB returned nil when falling back to the provided handle")
	}
	// The fallback path must operate on the provided handle, so the marker
	// table we created on it should be visible.
	if !got.Migrator().HasTable("fallback_marker") {
		t.Error("DB fallback should operate on the provided handle")
	}
}

func TestDBUsesContextTx(t *testing.T) {
	txDB := newTestDB(t)
	fallback := newTestDB(t)

	// Distinguish the two in-memory databases with marker tables.
	if err := txDB.Exec("CREATE TABLE tx_marker (id INTEGER)").Error; err != nil {
		t.Fatalf("failed to create tx marker: %v", err)
	}
	if err := fallback.Exec("CREATE TABLE fallback_marker (id INTEGER)").Error; err != nil {
		t.Fatalf("failed to create fallback marker: %v", err)
	}

	ctx := tx.WithTx(context.Background(), txDB)
	got := tx.DB(ctx, fallback)
	if got == nil {
		t.Fatal("DB returned nil when a tx was present in the context")
	}

	// WithContext clones the Config, so we distinguish the source by which
	// marker table is visible rather than comparing Config pointers.
	if !got.Migrator().HasTable("tx_marker") {
		t.Error("DB should operate on the context tx, which contains tx_marker")
	}
	if got.Migrator().HasTable("fallback_marker") {
		t.Error("DB should not operate on the fallback handle when a context tx exists")
	}
}
