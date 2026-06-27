package kexswiftbus

import (
	"time"
)

type Message struct {
	ID        string            `json:"id"`
	Topic     string            `json:"topic"`
	Payload   interface{}       `json:"payload"`
	Timestamp time.Time         `json:"timestamp"`
	Metadata  map[string]string `json:"metadata,omitempty"`
}

func NewMessage(topic string, payload interface{}) *Message {
	return &Message{
		Topic:     topic,
		Payload:   payload,
		Timestamp: time.Now(),
		Metadata:  make(map[string]string),
	}
}
