package kexswiftbus

import (
	"context"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"
)

const (
	Wildcard = "*"
)

type Handler func(msg *Message)

type CancelFunc func()

var handlerIDCounter atomic.Uint64

type subscription struct {
	id     uint64
	topic  string
	ch     <-chan *Message
	cancel chan struct{}
}

type Bus struct {
	store   Store
	subs    sync.Map
	closed  atomic.Bool
	closeCh chan struct{}
	wg      sync.WaitGroup
}

func NewBus(store Store) *Bus {
	return &Bus{
		store:   store,
		closeCh: make(chan struct{}),
	}
}

func (b *Bus) Subscribe(topic string, handler Handler) (CancelFunc, error) {
	if b.closed.Load() {
		return nil, ErrBusClosed
	}

	ch, err := b.store.Subscribe(context.Background(), topic)
	if err != nil {
		return nil, err
	}

	id := handlerIDCounter.Add(1)
	sub := &subscription{
		id:     id,
		topic:  topic,
		ch:     ch,
		cancel: make(chan struct{}),
	}
	b.subs.Store(id, sub)

	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		for {
			select {
			case msg, ok := <-ch:
				if !ok {
					return
				}
				b.invoke(handler, msg)
			case <-sub.cancel:
				return
			case <-b.closeCh:
				return
			}
		}
	}()

	once := sync.Once{}
	return func() {
		once.Do(func() {
			close(sub.cancel)
			b.subs.Delete(id)
			_ = b.store.Unsubscribe(context.Background(), sub.topic, sub.ch)
		})
	}, nil
}

func (b *Bus) Publish(msg *Message) bool {
	if b.closed.Load() {
		return false
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := b.store.Publish(ctx, msg); err != nil {
		return false
	}
	return true
}

func (b *Bus) PublishSync(msg *Message) error {
	if b.closed.Load() {
		return ErrBusClosed
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return b.store.Publish(ctx, msg)
}

func (b *Bus) Close() {
	if !b.closed.CompareAndSwap(false, true) {
		return
	}
	close(b.closeCh)
	_ = b.store.Close()
	b.wg.Wait()
}

func (b *Bus) Done() <-chan struct{} {
	return b.closeCh
}

func (b *Bus) invoke(handler Handler, msg *Message) {
	defer func() {
		if r := recover(); r != nil {
			slog.Error("kexswiftbus: handler panic", "topic", msg.Topic, "error", r)
		}
	}()
	handler(msg)
}
