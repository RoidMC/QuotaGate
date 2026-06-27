package smtp

import (
	"context"
	"time"
)

type ContentType string

const (
	ContentTypeTextPlain ContentType = "text/plain"
	ContentTypeTextHTML  ContentType = "text/html"
)

type Attachment struct {
	Filename    string
	ContentType string
	Data        []byte
}

type Message struct {
	From        string
	To          []string
	Cc          []string
	Bcc         []string
	Subject     string
	Body        string
	ContentType ContentType
	Attachments []Attachment
	ReplyTo     string
}

type SendResult struct {
	MessageID string
	SentAt    time.Time
}

type Mailer interface {
	Send(ctx context.Context, msg *Message) (*SendResult, error)
	Ping(ctx context.Context) error
	Close() error
}
