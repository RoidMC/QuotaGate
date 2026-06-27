package kexswiftbus

import "context"

type Store interface {
	Publish(ctx context.Context, msg *Message) error
	Subscribe(ctx context.Context, topic string) (<-chan *Message, error)
	Unsubscribe(ctx context.Context, topic string, ch <-chan *Message) error
	Close() error
}
