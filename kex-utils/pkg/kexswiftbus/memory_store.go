package kexswiftbus

import (
	"context"
	"sync"
)

type MemoryStore struct {
	mu        sync.RWMutex
	subs      map[string][]chan *Message
	closed    bool
	closeCh   chan struct{}
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		subs:    make(map[string][]chan *Message),
		closeCh: make(chan struct{}),
	}
}

func (s *MemoryStore) Publish(ctx context.Context, msg *Message) error {
	s.mu.RLock()
	if s.closed {
		s.mu.RUnlock()
		return ErrStoreClosed
	}
	subs := make([]chan *Message, 0)
	for _, ch := range s.subs[msg.Topic] {
		subs = append(subs, ch)
	}
	for _, ch := range s.subs["*"] {
		subs = append(subs, ch)
	}
	s.mu.RUnlock()

	for _, ch := range subs {
		select {
		case ch <- msg:
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
	}
	return nil
}

func (s *MemoryStore) Subscribe(ctx context.Context, topic string) (<-chan *Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil, ErrStoreClosed
	}

	ch := make(chan *Message, 64)
	s.subs[topic] = append(s.subs[topic], ch)
	return ch, nil
}

func (s *MemoryStore) Unsubscribe(ctx context.Context, topic string, ch <-chan *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	subs := s.subs[topic]
	for i, c := range subs {
		if c == ch {
			close(c)
			s.subs[topic] = append(subs[:i], subs[i+1:]...)
			if len(s.subs[topic]) == 0 {
				delete(s.subs, topic)
			}
			return nil
		}
	}
	return ErrTopicNotFound
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	for _, subs := range s.subs {
		for _, ch := range subs {
			close(ch)
		}
	}
	s.subs = nil
	s.mu.Unlock()
	close(s.closeCh)
	return nil
}
