package kexswiftdb

import (
	"context"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	badger "github.com/dgraph-io/badger/v4"
)

const maxRetries = 8

type BadgerStore struct {
	db     *badger.DB
	dir    string
	closed atomic.Bool
	quit   chan struct{}
}

func NewBadgerStore(dir string) (*BadgerStore, error) {
	opts := badger.DefaultOptions(dir)
	opts.Logger = nil

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	s := &BadgerStore{
		db:   db,
		dir:  dir,
		quit: make(chan struct{}),
	}
	go s.gcLoop()
	return s, nil
}

func NewInMemoryBadgerStore() (*BadgerStore, error) {
	opts := badger.DefaultOptions("").WithInMemory(true)
	opts.Logger = nil

	db, err := badger.Open(opts)
	if err != nil {
		return nil, err
	}

	s := &BadgerStore{
		db:   db,
		dir:  ":memory:",
		quit: make(chan struct{}),
	}
	go s.gcLoop()
	return s, nil
}

func (s *BadgerStore) update(fn func(txn *badger.Txn) error) error {
	for i := 0; i < maxRetries; i++ {
		err := s.db.Update(fn)
		if err == nil {
			return nil
		}
		if err == badger.ErrConflict {
			continue
		}
		return err
	}
	return ErrCASConflict
}

func (s *BadgerStore) Set(ctx context.Context, prefix Prefix, key string, value []byte, ttl time.Duration) error {
	if s.closed.Load() {
		return ErrStoreClosed
	}

	k := buildKey(prefix, key)
	return s.update(func(txn *badger.Txn) error {
		e := badger.NewEntry([]byte(k), value)
		if ttl > 0 {
			e = e.WithTTL(ttl)
		}
		return txn.SetEntry(e)
	})
}

func (s *BadgerStore) Get(ctx context.Context, prefix Prefix, key string) ([]byte, error) {
	if s.closed.Load() {
		return nil, ErrStoreClosed
	}

	k := buildKey(prefix, key)
	var val []byte
	err := s.db.View(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(k))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				return ErrKeyNotFound
			}
			return err
		}
		val, err = item.ValueCopy(nil)
		return err
	})
	if err != nil {
		return nil, err
	}
	return val, nil
}

func (s *BadgerStore) Delete(ctx context.Context, prefix Prefix, key string) error {
	if s.closed.Load() {
		return ErrStoreClosed
	}

	k := buildKey(prefix, key)
	return s.update(func(txn *badger.Txn) error {
		return txn.Delete([]byte(k))
	})
}

func (s *BadgerStore) Exists(ctx context.Context, prefix Prefix, key string) (bool, error) {
	if s.closed.Load() {
		return false, ErrStoreClosed
	}

	k := buildKey(prefix, key)
	err := s.db.View(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(k))
		return err
	})
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *BadgerStore) Increment(ctx context.Context, prefix Prefix, key string, ttl time.Duration) (int64, error) {
	if s.closed.Load() {
		return 0, ErrStoreClosed
	}

	k := buildKey(prefix, key)
	var result int64

	err := s.update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(k))
		if err != nil && err != badger.ErrKeyNotFound {
			return err
		}

		var current int64
		isNew := err == badger.ErrKeyNotFound
		if !isNew {
			val, err := item.ValueCopy(nil)
			if err != nil {
				return err
			}
			current, _ = strconv.ParseInt(string(val), 10, 64)
		}

		result = current + 1
		e := badger.NewEntry([]byte(k), []byte(strconv.FormatInt(result, 10)))
		if isNew && ttl > 0 {
			e = e.WithTTL(ttl)
		}
		return txn.SetEntry(e)
	})

	if err != nil {
		return 0, err
	}
	return result, nil
}

func (s *BadgerStore) SetNX(ctx context.Context, prefix Prefix, key string, value []byte, ttl time.Duration) (bool, error) {
	if s.closed.Load() {
		return false, ErrStoreClosed
	}

	k := buildKey(prefix, key)
	var created bool

	err := s.update(func(txn *badger.Txn) error {
		_, err := txn.Get([]byte(k))
		if err == nil {
			created = false
			return nil
		}
		if err != badger.ErrKeyNotFound {
			return err
		}

		created = true
		e := badger.NewEntry([]byte(k), value)
		if ttl > 0 {
			e = e.WithTTL(ttl)
		}
		return txn.SetEntry(e)
	})

	if err != nil {
		return false, err
	}
	return created, nil
}

func (s *BadgerStore) CompareAndSwap(ctx context.Context, prefix Prefix, key string, oldValue, newValue []byte, ttl time.Duration) (bool, error) {
	if s.closed.Load() {
		return false, ErrStoreClosed
	}

	k := buildKey(prefix, key)
	var swapped bool

	err := s.update(func(txn *badger.Txn) error {
		item, err := txn.Get([]byte(k))
		if err != nil {
			if err == badger.ErrKeyNotFound {
				if oldValue == nil {
					swapped = true
					e := badger.NewEntry([]byte(k), newValue)
					if ttl > 0 {
						e = e.WithTTL(ttl)
					}
					return txn.SetEntry(e)
				}
				swapped = false
				return nil
			}
			return err
		}

		current, err := item.ValueCopy(nil)
		if err != nil {
			return err
		}

		if !bytesEqual(current, oldValue) {
			swapped = false
			return nil
		}

		swapped = true
		e := badger.NewEntry([]byte(k), newValue)
		if ttl > 0 {
			e = e.WithTTL(ttl)
		}
		return txn.SetEntry(e)
	})

	if err != nil {
		return false, err
	}
	return swapped, nil
}

func (s *BadgerStore) Keys(ctx context.Context, prefix Prefix, keyPrefix string) ([]string, error) {
	if s.closed.Load() {
		return nil, ErrStoreClosed
	}

	nsPrefix := string(prefix) + ":"
	if keyPrefix != "" {
		nsPrefix += keyPrefix
	}

	var keys []string
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefixBytes := []byte(nsPrefix)
		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()
			k := string(item.Key())
			localKey := strings.TrimPrefix(k, string(prefix)+":")
			keys = append(keys, localKey)
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return keys, nil
}

func (s *BadgerStore) DeleteByPrefix(ctx context.Context, prefix Prefix, keyPrefix string) (int, error) {
	if s.closed.Load() {
		return 0, ErrStoreClosed
	}

	nsPrefix := string(prefix) + ":"
	if keyPrefix != "" {
		nsPrefix += keyPrefix
	}

	var count int
	err := s.update(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		prefixBytes := []byte(nsPrefix)
		for it.Seek(prefixBytes); it.ValidForPrefix(prefixBytes); it.Next() {
			item := it.Item()
			if err := txn.Delete(item.KeyCopy(nil)); err != nil {
				return err
			}
			count++
		}
		return nil
	})

	if err != nil {
		return 0, err
	}
	return count, nil
}

func (s *BadgerStore) Stats(ctx context.Context) []StoreStats {
	if s.closed.Load() {
		return nil
	}

	nsCounts := make(map[string]int)
	err := s.db.View(func(txn *badger.Txn) error {
		it := txn.NewIterator(badger.DefaultIteratorOptions)
		defer it.Close()

		for it.Rewind(); it.Valid(); it.Next() {
			item := it.Item()
			k := string(item.Key())
			ns := extractPrefix(k)
			if ns != "" {
				nsCounts[ns]++
			}
		}
		return nil
	})

	if err != nil {
		return nil
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

func (s *BadgerStore) Ping(ctx context.Context) error {
	if s.closed.Load() {
		return ErrStoreClosed
	}
	return s.db.View(func(txn *badger.Txn) error {
		return nil
	})
}

func (s *BadgerStore) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(s.quit)
	return s.db.Close()
}

func (s *BadgerStore) gcLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.quit:
			return
		case <-ticker.C:
			for {
				err := s.db.RunValueLogGC(0.5)
				if err != nil {
					break
				}
			}
			_ = s.db.Flatten(2)
		}
	}
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
