package kexswiftdb

import (
	"context"
	"strings"
	"time"
)

// Prefix is an arbitrary caller-defined key prefix, replacing the old fixed Namespace enum.
type Prefix string

// Common namespaces. Callers may compose sub-namespaces by concatenating, e.g.
// Prefix(PrefixAuth + ":revoke").
const (
	PrefixAuth      Prefix = "auth"
	PrefixBilling   Prefix = "billing"
	PrefixSession   Prefix = "session"
	PrefixQRCode    Prefix = "qrcode"
	PrefixRateLimit Prefix = "ratelimit"
	PrefixWebhook   Prefix = "webhook"
	PrefixAudit     Prefix = "audit"
	PrefixAnalytics Prefix = "analytics"
)

type StoreStats struct {
	Namespace string `json:"namespace"`
	KeyCount  int    `json:"key_count"`
}

type Store interface {
	Set(ctx context.Context, prefix Prefix, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, prefix Prefix, key string) ([]byte, error)
	Delete(ctx context.Context, prefix Prefix, key string) error
	Exists(ctx context.Context, prefix Prefix, key string) (bool, error)
	SetNX(ctx context.Context, prefix Prefix, key string, value []byte, ttl time.Duration) (bool, error)
	CompareAndSwap(ctx context.Context, prefix Prefix, key string, oldValue, newValue []byte, ttl time.Duration) (bool, error)
	CompareAndDelete(ctx context.Context, prefix Prefix, key string, expected []byte) (bool, error)
	Increment(ctx context.Context, prefix Prefix, key string, ttl time.Duration) (int64, error)
	Keys(ctx context.Context, prefix Prefix, keyPrefix string) ([]string, error)
	DeleteByPrefix(ctx context.Context, prefix Prefix, keyPrefix string) (int, error)
	Stats(ctx context.Context) []StoreStats
	Ping(ctx context.Context) error
	Close() error
}

func buildKey(prefix Prefix, key string) string {
	return string(prefix) + ":" + key
}

func extractPrefix(key string) string {
	idx := strings.Index(key, ":")
	if idx > 0 {
		return key[:idx]
	}
	return key
}
