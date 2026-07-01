package kexswiftbus

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// receiveTimeout is the maximum time a consume goroutine will block waiting for
// the next Redis message before checking closeCh and ctx again.
const receiveTimeout = 5 * time.Second

// RedisStore implements Store using Redis Pub/Sub for cross-instance message
// delivery. Unlike MemoryStore, RedisStore supports multi-process distribution.
//
// Notes:
//   - Redis PUB/SUB does not support wildcard matching. Subscribing to "*"
//     via MemoryStore is not supported by RedisStore.
//   - Each Subscribe() call spawns a background goroutine that blocks on
//     Redis SUBSCRIBE. Unsubscribe() cleans up local state and closes PubSub.
//   - The channel buffer is 64 messages by default.
//   - Network errors during subscription are forwarded to the configured
//     OnError handler instead of being swallowed.
type RedisStore struct {
	client     *redis.Client
	subClients map[*redis.PubSub]chan *Message
	mu         sync.RWMutex
	closed     bool
	closeCh    chan struct{}
	prefix     string
	onError    func(topic string, err error)
	wg         sync.WaitGroup
}

// RedisStoreOption configures a RedisStore via functional options.
type RedisStoreOption func(*RedisStore)

// WithRedisPrefix sets a prefix for all Redis PUB/SUB channel names.
// This allows multiple QuotaGate instances to share the same Redis server
// without channel name collisions. If not set, no prefix is used.
func WithRedisPrefix(prefix string) RedisStoreOption {
	return func(s *RedisStore) {
		s.prefix = prefix
	}
}

// WithErrorHandler registers a callback invoked when a subscription encounters
// a non-recoverable Redis error (e.g., connection drop, protocol error). The
// callback receives the topic and the error. The subscription goroutine exits
// after invoking the handler. If no handler is set, the error is silently lost.
func WithErrorHandler(h func(topic string, err error)) RedisStoreOption {
	return func(s *RedisStore) {
		s.onError = h
	}
}

// NewRedisStore creates a new RedisStore backed by the given redis.Client.
// The client is NOT closed when RedisStore.Close() is called. The caller
// must manage the redis.Client lifecycle independently.
//
// If client is nil, NewRedisStore returns ErrNilClient.
func NewRedisStore(client *redis.Client, opts ...RedisStoreOption) (*RedisStore, error) {
	if client == nil {
		return nil, ErrNilClient
	}

	s := &RedisStore{
		client:     client,
		subClients: make(map[*redis.PubSub]chan *Message),
		closeCh:    make(chan struct{}),
	}

	for _, opt := range opts {
		opt(s)
	}

	return s, nil
}

func (s *RedisStore) channelName(topic string) string {
	if s.prefix != "" {
		return fmt.Sprintf("%s:%s", s.prefix, topic)
	}
	return topic
}

func (s *RedisStore) Publish(ctx context.Context, msg *Message) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	client := s.client
	s.mu.RUnlock()

	payload, err := encodePayload(msg.Payload)
	if err != nil {
		return fmt.Errorf("kexswiftbus: failed to marshal payload: %w", err)
	}

	return client.Publish(ctx, s.channelName(msg.Topic), payload).Err()
}

func encodePayload(payload interface{}) (string, error) {
	switch v := payload.(type) {
	case string:
		return v, nil
	case []byte:
		return string(v), nil
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
}

func (s *RedisStore) Subscribe(ctx context.Context, topic string) (<-chan *Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.closed {
		return nil, ErrStoreClosed
	}

	ch := make(chan *Message, 64)
	ps := s.client.Subscribe(ctx, s.channelName(topic))

	s.subClients[ps] = ch
	s.wg.Add(1)

	go s.consume(ctx, ps, ch, topic)

	return ch, nil
}

func (s *RedisStore) Unsubscribe(ctx context.Context, topic string, ch <-chan *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for ps, chh := range s.subClients {
		if chh == ch {
			ps.Close()
			close(chh)
			delete(s.subClients, ps)
			return nil
		}
	}
	return ErrTopicNotFound
}

func (s *RedisStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true

	for ps := range s.subClients {
		ps.Close()
	}
	s.subClients = nil
	close(s.closeCh)
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *RedisStore) consume(ctx context.Context, ps *redis.PubSub, ch chan *Message, topic string) {
	defer s.wg.Done()
	defer ps.Close()

	for {
		select {
		case <-s.closeCh:
			return
		case <-ctx.Done():
			return
		default:
		}

		// Use ReceiveTimeout instead of Channel() so network/Redis errors are
		// surfaced instead of being silently swallowed when the channel closes.
		msgi, err := ps.ReceiveTimeout(ctx, receiveTimeout)
		if err != nil {
			// Context cancellation / timeout are normal control paths.
			if errors.Is(err, context.Canceled) {
				return
			}
			if errors.Is(err, context.DeadlineExceeded) {
				continue
			}
			if s.onError != nil {
				s.onError(topic, err)
			}
			return
		}

		switch msg := msgi.(type) {
		case *redis.Message:
			m := &Message{
				Topic:   topic,
				Payload: msg.Payload,
			}
			select {
			case ch <- m:
			case <-s.closeCh:
				return
			case <-ctx.Done():
				return
			}
		case *redis.Subscription:
			// Subscription confirmation / unconfirmation; ignore.
		case error:
			if s.onError != nil {
				s.onError(topic, msg)
			}
		}
	}
}
