package kexswiftdb

import "errors"

var (
	ErrKeyNotFound  = errors.New("kexswiftdb: key not found")
	ErrStoreClosed  = errors.New("kexswiftdb: store is closed")
	ErrNilClient    = errors.New("kexswiftdb: redis client is nil")
	ErrCASConflict  = errors.New("kexswiftdb: cas conflict after retries")
)
