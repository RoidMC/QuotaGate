package kexsmtp

import "errors"

var (
	ErrMailerClosed     = errors.New("kexsmtp: mailer is closed")
	ErrInvalidMessage   = errors.New("kexsmtp: invalid message")
	ErrSendFailed       = errors.New("kexsmtp: failed to send email")
	ErrProviderError    = errors.New("kexsmtp: provider error")
	ErrNilClient        = errors.New("kexsmtp: client is nil")
	ErrAuthFailed       = errors.New("kexsmtp: authentication failed")
	ErrConnectionFailed = errors.New("kexsmtp: connection failed")
)
