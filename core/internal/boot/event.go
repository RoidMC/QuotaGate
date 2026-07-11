package boot

import (
	"fmt"

	"github.com/redis/go-redis/v9"

	"github.com/roidmc/quotagate/internal/config"
	"github.com/roidmc/quotagate/internal/event"
)

// InitEventBus creates the application event bus. When the configured store
// driver is "redis" the bus is backed by Redis Pub/Sub so that events are
// delivered across instances; otherwise a memory-backed bus is used for
// single-instance deployments.
func InitEventBus(cfg *config.Config) (*event.EventBus, error) {
	switch cfg.Store.Driver {
	case "redis":
		rdb := redis.NewClient(&redis.Options{
			Addr:     fmt.Sprintf("%s:%d", cfg.Store.Redis.Host, cfg.Store.Redis.Port),
			Password: cfg.Store.Redis.Password,
			DB:       cfg.Store.Redis.DB,
		})
		bus, err := event.NewRedisBus(rdb)
		if err != nil {
			return nil, fmt.Errorf("create redis event bus: %w", err)
		}
		return bus, nil
	default:
		return event.NewBus(), nil
	}
}
