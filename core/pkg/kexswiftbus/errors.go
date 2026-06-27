package kexswiftbus

import "errors"

var (
	ErrBusClosed     = errors.New("kexswiftbus: bus is closed")
	ErrStoreClosed   = errors.New("kexswiftbus: store is closed")
	ErrNilClient     = errors.New("kexswiftbus: client is nil")
	ErrTopicNotFound = errors.New("kexswiftbus: topic not found")
	ErrInvalidTopic  = errors.New("kexswiftbus: invalid topic name")
)
