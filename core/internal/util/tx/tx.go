// Package tx provides context propagation for GORM transactions.
//
// It mirrors the pattern used by the event package's transactional outbox:
// callers attach a transaction to a context with WithTx, and downstream
// repository/webhook code extracts it with TxFromContext. This keeps the
// transaction handle out of method signatures while still allowing multiple
// operations to participate in the same database transaction.
package tx

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// WithTx returns a context that carries the given GORM transaction.
// Passing nil returns the parent context unchanged.
func WithTx(parent context.Context, tx *gorm.DB) context.Context {
	if tx == nil {
		return parent
	}
	return context.WithValue(parent, txKey{}, tx)
}

// TxFromContext retrieves the GORM transaction attached by WithTx.
func TxFromContext(ctx context.Context) (*gorm.DB, bool) {
	if ctx == nil {
		return nil, false
	}
	tx, ok := ctx.Value(txKey{}).(*gorm.DB)
	return tx, ok
}

// DB returns the transaction from the context when available; otherwise it
// falls back to the provided database handle.
func DB(ctx context.Context, db *gorm.DB) *gorm.DB {
	if tx, ok := TxFromContext(ctx); ok {
		return tx.WithContext(ctx)
	}
	return db.WithContext(ctx)
}
