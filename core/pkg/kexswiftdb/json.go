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

// ConsumeJSON atomically reads a JSON value and deletes the key, but only if
// the value currently stored is the one we read (compare-and-delete). The
// callback decides whether to consume: return ok=false to leave the entry
// untouched and receive nil (treated as "not consumed"); return ok=true to
// delete the key and receive the value that was read.
//
// This is the primitive behind one-time tokens (OAuth state, WebAuthn
// challenges, QR tickets): the first consumer wins and the key disappears, so
// a concurrent consumer observes a miss instead of a tombstone. Unlike
// MutateJSON it performs a real delete — no short-TTL tombstone workaround.
func ConsumeJSON[T any](ctx context.Context, s Store, prefix Prefix, key string, fn func(current *T) (bool, error)) (*T, error) {
	for i := 0; i < maxRetries; i++ {
		data, err := s.Get(ctx, prefix, key)
		if err != nil {
			if err == ErrKeyNotFound {
				return nil, nil
			}
			return nil, err
		}

		var current T
		if err := json.Unmarshal(data, &current); err != nil {
			return nil, err
		}

		ok, fnErr := fn(&current)
		if fnErr != nil {
			return nil, fnErr
		}
		if !ok {
			// Callback declined to consume; leave the entry as-is.
			return nil, nil
		}

		deleted, delErr := s.CompareAndDelete(ctx, prefix, key, data)
		if delErr != nil {
			return nil, delErr
		}
		if deleted {
			return &current, nil
		}
		// Someone changed the value underneath us; retry the read+compare.
	}
	return nil, ErrCASConflict
}
