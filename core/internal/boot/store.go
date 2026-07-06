package boot

import (
	"context"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/roidmc/quotagate/internal/config"
	"github.com/roidmc/quotagate/pkg/kexswiftdb"
)

func InitStore(cfg *config.Config) (kexswiftdb.Store, error) {
	switch cfg.Store.Driver {
	case "redis":
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Store.Redis.Host, cfg.Store.Redis.Port),
			Password: cfg.Store.Redis.Password,
			DB:       cfg.Store.Redis.DB,
		})
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := rdb.Ping(ctx).Err(); err != nil {
			return nil, fmt.Errorf("connect redis: %w", err)
		}
		store, err := kexswiftdb.NewRedisStore(rdb)
		if err != nil {
			return nil, fmt.Errorf("create redis store: %w", err)
		}
		return store, nil
	case "badger":
		store, err := kexswiftdb.NewBadgerStore(cfg.Store.Badger.Dir)
		if err != nil {
			return nil, fmt.Errorf("open badger: %w", err)
		}
		return store, nil
	default:
		store, err := kexswiftdb.NewBadgerStore(cfg.Store.Badger.Dir)
		if err != nil {
			return nil, fmt.Errorf("open badger (default): %w", err)
		}
		return store, nil
	}
}
