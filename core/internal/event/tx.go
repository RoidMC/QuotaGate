package event

import (
	"context"

	"gorm.io/gorm"
)

type txKey struct{}

// WithTx attaches a GORM transaction to the context. Event publishers can use
// this to make outbox writes participate in the same transaction as the
// business operation, without the publisher caring whether the underlying
// event bus uses memory, Redis, or any other store.
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
