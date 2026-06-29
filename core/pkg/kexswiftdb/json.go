package kexswiftdb

import (
	"context"
	"encoding/json"
	"time"
)

func GetJSON[T any](ctx context.Context, s Store, prefix Prefix, key string) (*T, error) {
	data, err := s.Get(ctx, prefix, key)
	if err != nil {
		return nil, err
	}
	var val T
	if err := json.Unmarshal(data, &val); err != nil {
		return nil, err
	}
	return &val, nil
}

func SetJSON[T any](ctx context.Context, s Store, prefix Prefix, key string, val T, ttl time.Duration) error {
	data, err := json.Marshal(val)
	if err != nil {
		return err
	}
	return s.Set(ctx, prefix, key, data, ttl)
}

type MutateFunc[T any] func(current *T) (newVal T, ok bool, ttl time.Duration, err error)

func MutateJSON[T any](ctx context.Context, s Store, prefix Prefix, key string, fn MutateFunc[T]) (*T, error) {
	for i := 0; i < maxRetries; i++ {
		data, err := s.Get(ctx, prefix, key)
		if err != nil {
			if err == ErrKeyNotFound {
				newVal, ok, ttl, fnErr := fn(nil)
				if fnErr != nil {
					return nil, fnErr
				}
				if !ok {
					return nil, nil
				}
				newData, mErr := json.Marshal(newVal)
				if mErr != nil {
					return nil, mErr
				}
				set, setErr := s.SetNX(ctx, prefix, key, newData, ttl)
				if setErr != nil {
					return nil, setErr
				}
				if set {
					return &newVal, nil
				}
				continue
			}
			return nil, err
		}

		var current T
		if err := json.Unmarshal(data, &current); err != nil {
			return nil, err
		}

		newVal, ok, ttl, fnErr := fn(&current)
		if fnErr != nil {
			return nil, fnErr
		}
		if !ok {
			return &current, nil
		}

		newData, mErr := json.Marshal(newVal)
		if mErr != nil {
			return nil, mErr
		}

		swapped, casErr := s.CompareAndSwap(ctx, prefix, key, data, newData, ttl)
		if casErr != nil {
			return nil, casErr
		}
		if swapped {
			return &newVal, nil
		}
	}

	return nil, ErrCASConflict
}
