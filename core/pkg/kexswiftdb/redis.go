package kexswiftdb

import (
	"context"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"
)

type RedisStore struct {
	client *redis.Client
	closed atomic.Bool
}

func NewRedisStore(client *redis.Client) (*RedisStore, error) {
	if client == nil {
		return nil, ErrNilClient
	}
	return &RedisStore{client: client}, nil
}

func (s *RedisStore) Set(ctx context.Context, prefix Prefix, key string, value []byte, ttl time.Duration) error {
	if s.closed.Load() {
		return ErrStoreClosed
	}
	k := buildKey(prefix, key)
	return s.client.Set(ctx, k, value, ttl).Err()
}

func (s *RedisStore) Get(ctx context.Context, prefix Prefix, key string) ([]byte, error) {
	if s.closed.Load() {
		return nil, ErrStoreClosed
	}
	k := buildKey(prefix, key)
	val, err := s.client.Get(ctx, k).Bytes()
	if err != nil {
		if err == redis.Nil {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return val, nil
}

func (s *RedisStore) Delete(ctx context.Context, prefix Prefix, key string) error {
	if s.closed.Load() {
		return ErrStoreClosed
	}
	k := buildKey(prefix, key)
	return s.client.Del(ctx, k).Err()
}

func (s *RedisStore) Exists(ctx context.Context, prefix Prefix, key string) (bool, error) {
	if s.closed.Load() {
		return false, ErrStoreClosed
	}
	k := buildKey(prefix, key)
	n, err := s.client.Exists(ctx, k).Result()
	if err != nil {
		return false, err
	}
	return n > 0, nil
}

func (s *RedisStore) Increment(ctx context.Context, prefix Prefix, key string, ttl time.Duration) (int64, error) {
	if s.closed.Load() {
		return 0, ErrStoreClosed
	}
	k := buildKey(prefix, key)
	val, err := s.client.Incr(ctx, k).Result()
	if err != nil {
		return 0, err
	}
	if val == 1 && ttl > 0 {
		if err := s.client.Expire(ctx, k, ttl).Err(); err != nil {
			return 0, err
		}
	}
	return val, nil
}

func (s *RedisStore) SetNX(ctx context.Context, prefix Prefix, key string, value []byte, ttl time.Duration) (bool, error) {
	if s.closed.Load() {
		return false, ErrStoreClosed
	}
	k := buildKey(prefix, key)
	ok, err := s.client.SetNX(ctx, k, value, ttl).Result()
	if err != nil {
		return false, err
	}
	return ok, nil
}

func (s *RedisStore) CompareAndSwap(ctx context.Context, prefix Prefix, key string, oldValue, newValue []byte, ttl time.Duration) (bool, error) {
	if s.closed.Load() {
		return false, ErrStoreClosed
	}
	k := buildKey(prefix, key)

	for i := 0; i < maxRetries; i++ {
		var swapped bool

		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			current, err := tx.Get(ctx, k).Bytes()
			if err != nil {
				if err == redis.Nil {
					if oldValue == nil {
						swapped = true
						_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
							pipe.Set(ctx, k, newValue, ttl)
							return nil
						})
						return err
					}
					swapped = false
					return nil
				}
				return err
			}

			if !bytesEqual(current, oldValue) {
				swapped = false
				return nil
			}

			swapped = true
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Set(ctx, k, newValue, ttl)
				return nil
			})
			return err
		}, k)

		if err == nil {
			return swapped, nil
		}
		if err == redis.TxFailedErr {
			continue
		}
		return false, err
	}

	return false, ErrCASConflict
}

func (s *RedisStore) CompareAndDelete(ctx context.Context, prefix Prefix, key string, expected []byte) (bool, error) {
	if s.closed.Load() {
		return false, ErrStoreClosed
	}
	k := buildKey(prefix, key)

	for i := 0; i < maxRetries; i++ {
		var deleted bool

		err := s.client.Watch(ctx, func(tx *redis.Tx) error {
			current, err := tx.Get(ctx, k).Bytes()
			if err != nil {
				if err == redis.Nil {
					deleted = false
					return nil
				}
				return err
			}

			if !bytesEqual(current, expected) {
				deleted = false
				return nil
			}

			deleted = true
			_, err = tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
				pipe.Del(ctx, k)
				return nil
			})
			return err
		}, k)

		if err == nil {
			return deleted, nil
		}
		if err == redis.TxFailedErr {
			continue
		}
		return false, err
	}

	return false, ErrCASConflict
}

func (s *RedisStore) Keys(ctx context.Context, prefix Prefix, keyPrefix string) ([]string, error) {
	if s.closed.Load() {
		return nil, ErrStoreClosed
	}
	pattern := string(prefix) + ":"
	if keyPrefix != "" {
		pattern += keyPrefix + "*"
	} else {
		pattern += "*"
	}

	var keys []string
	var cursor uint64
	for {
		var batch []string
		var err error
		batch, cursor, err = s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, err
		}
		for _, k := range batch {
			localKey := k[len(string(prefix))+1:]
			keys = append(keys, localKey)
		}
		if cursor == 0 {
			break
		}
	}

	return keys, nil
}

func (s *RedisStore) DeleteByPrefix(ctx context.Context, prefix Prefix, keyPrefix string) (int, error) {
	if s.closed.Load() {
		return 0, ErrStoreClosed
	}
	pattern := string(prefix) + ":"
	if keyPrefix != "" {
		pattern += keyPrefix + "*"
	} else {
		pattern += "*"
	}

	var total int
	var cursor uint64
	for {
		var batch []string
		var err error
		batch, cursor, err = s.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return total, err
		}
		if len(batch) > 0 {
			_, err = s.client.Del(ctx, batch...).Result()
			if err != nil {
				return total, err
			}
			total += len(batch)
		}
		if cursor == 0 {
			break
		}
	}

	return total, nil
}

func (s *RedisStore) Stats(ctx context.Context) []StoreStats {
	if s.closed.Load() {
		return nil
	}

	var cursor uint64
	nsCounts := make(map[string]int)
	for {
		batch, next, err := s.client.Scan(ctx, cursor, "*", 1000).Result()
		if err != nil {
			return nil
		}
		for _, k := range batch {
			ns := extractPrefix(k)
			if ns != "" {
				nsCounts[ns]++
			}
		}
		cursor = next
		if cursor == 0 {
			break
		}
	}

	stats := make([]StoreStats, 0, len(nsCounts))
	for ns, count := range nsCounts {
		stats = append(stats, StoreStats{
			Namespace: ns,
			KeyCount:  count,
		})
	}
	return stats
}

func (s *RedisStore) Ping(ctx context.Context) error {
	if s.closed.Load() {
		return ErrStoreClosed
	}
	return s.client.Ping(ctx).Err()
}

func (s *RedisStore) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	return s.client.Close()
}

func ParseInt64(data []byte) (int64, error) {
	return strconv.ParseInt(string(data), 10, 64)
}

func FormatInt64(val int64) []byte {
	return []byte(strconv.FormatInt(val, 10))
}
