package kexsmtp

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"mime/quotedprintable"
	"net"
	"net/smtp"
	"strings"
	"sync/atomic"
	"time"
)

type StandardSMTPConfig struct {
	Host      string
	Port      int
	Username  string
	Password  string
	From      string
	TLSMode   TLSMode
	LocalName string
}

type TLSMode int

const (
	TLSModeNone TLSMode = iota
	TLSModeSTARTTLS
	TLSModeTLS
)

type StandardSMTP struct {
	config StandardSMTPConfig
	addr   string
	pool   *ConnPool
	closed atomic.Bool
}

func NewStandardSMTP(config StandardSMTPConfig) (*StandardSMTP, error) {
	if config.Host == "" {
		return nil, ErrConnectionFailed
	}
	if config.Port == 0 {
		config.Port = 587
	}
	if config.From == "" {
		config.From = config.Username
	}

	return &StandardSMTP{
		config: config,
		addr:   fmt.Sprintf("%s:%d", config.Host, config.Port),
	}, nil
}

// NewStandardSMTPWithPool creates a StandardSMTP with connection pooling.
func NewStandardSMTPWithPool(config StandardSMTPConfig, maxIdle, maxActive int) (*StandardSMTP, error) {
	s, err := NewStandardSMTP(config)
	if err != nil {
		return nil, err
	}
	pool, err := NewConnPool(config, maxIdle, maxActive)
	if err != nil {
		return nil, fmt.Errorf("kexsmtp: failed to create connection pool: %w", err)
	}
	s.pool = pool
	return s, nil
}

func (s *StandardSMTP) Send(ctx context.Context, msg *Message) (*SendResult, error) {
	if s.closed.Load() {
		return nil, ErrMailerClosed
	}

	if err := ValidateMessage(msg); err != nil {
		return nil, err
	}

	from := msg.From
	if from == "" {
		from = s.config.From
	}

	messageID := GenerateMessageID(s.config.Host)

	body, err := BuildMessageBody(from, msg, messageID)
	if err != nil {
		return nil, fmt.Errorf("kexsmtp: failed to build message: %w", err)
	}

	const maxRetries = 3
	var lastErr error

	for attempt := 0; attempt < maxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.NewTimer(time.Duration(attempt*attempt) * time.Second)
			select {
			case <-ctx.Done():
				backoff.Stop()
				return nil, ctx.Err()
			case <-backoff.C:
			}
		}

		result, err := s.sendOnce(ctx, from, msg, body, messageID)
		if err == nil {
			return result, nil
		}

		lastErr = err
		if isNonRetryableError(err) {
			break
		}
	}

	return nil, fmt.Errorf("kexsmtp: send failed after %d attempts: %w", maxRetries, lastErr)
}

func (s *StandardSMTP) sendOnce(ctx context.Context, from string, msg *Message, body []byte, messageID string) (*SendResult, error) {
	var client *smtp.Client
	var err error

	if s.pool != nil {
		client, err = s.pool.Get(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get connection: %w", err)
		}
	} else {
		client, err = s.dial(ctx)
		if err != nil {
			return nil, err
		}
		defer client.Close()
	}

	if err := client.Mail(from); err != nil {
		if s.pool != nil {
			s.pool.Put(client, true)
		}
		return nil, fmt.Errorf("failed to set sender: %w", err)
	}

	allRecipients := collectRecipients(msg)

	for _, rcpt := range allRecipients {
		if err := client.Rcpt(rcpt); err != nil {
			if s.pool != nil {
				s.pool.Put(client, true)
			}
			return nil, fmt.Errorf("failed to set recipient: %w", err)
		}
	}

	w, err := client.Data()
	if err != nil {
		if s.pool != nil {
			s.pool.Put(client, true)
		}
		return nil, fmt.Errorf("failed to open data: %w", err)
	}

	if _, err := w.Write(body); err != nil {
		w.Close()
		if s.pool != nil {
			s.pool.Put(client, true)
		}
		return nil, fmt.Errorf("failed to write message: %w", err)
	}

	if err := w.Close(); err != nil {
		if s.pool != nil {
			s.pool.Put(client, true)
		}
		return nil, fmt.Errorf("failed to close data: %w", err)
	}

	if s.pool != nil {
		s.pool.Put(client, false)
	} else {
		client.Quit()
	}

	return &SendResult{
		MessageID: messageID,
		SentAt:    time.Now().UTC(),
	}, nil
}

func (s *StandardSMTP) dial(ctx context.Context) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	var client *smtp.Client

	switch s.config.TLSMode {
	case TLSModeTLS:
		tlsConfig := &tls.Config{ServerName: s.config.Host}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, fmt.Errorf("tls handshake failed: %w", err)
		}
		client, err = smtp.NewClient(tlsConn, s.config.Host)
	default:
		client, err = smtp.NewClient(conn, s.config.Host)
	}
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create smtp client: %w", err)
	}

	if s.config.TLSMode == TLSModeSTARTTLS {
		tlsConfig := &tls.Config{ServerName: s.config.Host}
		if err := client.StartTLS(tlsConfig); err != nil {
			client.Close()
			return nil, fmt.Errorf("starttls failed: %w", err)
		}
	}

	if s.config.Username != "" && s.config.Password != "" {
		auth := smtp.PlainAuth("", s.config.Username, s.config.Password, s.config.Host)
		if err := client.Auth(auth); err != nil {
			client.Close()
			return nil, fmt.Errorf("%w", ErrAuthFailed)
		}
	}

	return client, nil
}

func collectRecipients(msg *Message) []string {
	all := make([]string, 0, len(msg.To)+len(msg.Cc)+len(msg.Bcc))
	all = append(all, msg.To...)
	all = append(all, msg.Cc...)
	all = append(all, msg.Bcc...)
	return all
}

func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, ErrAuthFailed) {
		return true
	}
	if errors.Is(err, ErrInvalidMessage) {
		return true
	}
	return false
}

func (s *StandardSMTP) Ping(ctx context.Context) error {
	if s.closed.Load() {
		return ErrMailerClosed
	}

	dialer := &net.Dialer{Timeout: 10 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", s.addr)
	if err != nil {
		return fmt.Errorf("kexsmtp: ping failed: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, s.config.Host)
	if err != nil {
		return fmt.Errorf("kexsmtp: ping failed: %w", err)
	}
	defer client.Close()

	if err := client.Noop(); err != nil {
		return fmt.Errorf("kexsmtp: ping failed: %w", err)
	}

	client.Quit()
	return nil
}

func (s *StandardSMTP) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	if s.pool != nil {
		s.pool.Close()
	}
	return nil
}

// ValidateMessage checks if a message has the minimum required fields.
func ValidateMessage(msg *Message) error {
	if msg == nil {
		return ErrInvalidMessage
	}
	if len(msg.To) == 0 && len(msg.Cc) == 0 && len(msg.Bcc) == 0 {
		return ErrInvalidMessage
	}
	if msg.Subject == "" && msg.Body == "" && len(msg.Attachments) == 0 {
		return ErrInvalidMessage
	}
	return nil
}

// BuildMessageBody constructs the raw MIME message bytes.
// Exported for testing; prefer using Send() for production use.
func BuildMessageBody(from string, msg *Message, messageID string) ([]byte, error) {
	var buf bytes.Buffer
	var boundary string

	hasAttachments := len(msg.Attachments) > 0
	if hasAttachments {
		boundary = randomBoundary()
	}

	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", strings.Join(msg.To, ", "))
	if len(msg.Cc) > 0 {
		fmt.Fprintf(&buf, "Cc: %s\r\n", strings.Join(msg.Cc, ", "))
	}
	fmt.Fprintf(&buf, "Subject: %s\r\n", mimeEncodeHeader(msg.Subject))
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "Message-Id: %s\r\n", messageID)
	if msg.ReplyTo != "" {
		fmt.Fprintf(&buf, "Reply-To: %s\r\n", msg.ReplyTo)
	}

	contentType := string(msg.ContentType)
	if contentType == "" {
		contentType = string(ContentTypeTextPlain)
	}

	if hasAttachments {
		fmt.Fprintf(&buf, "Content-Type: multipart/mixed; boundary=\"%s\"\r\n", boundary)
		fmt.Fprintf(&buf, "\r\n")
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: %s; charset=\"utf-8\"\r\n", contentType)
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: quoted-printable\r\n")
		fmt.Fprintf(&buf, "\r\n")
	} else {
		fmt.Fprintf(&buf, "Content-Type: %s; charset=\"utf-8\"\r\n", contentType)
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: quoted-printable\r\n")
		fmt.Fprintf(&buf, "\r\n")
	}

	qpWriter := quotedprintable.NewWriter(&buf)
	if _, err := qpWriter.Write([]byte(msg.Body)); err != nil {
		return nil, fmt.Errorf("failed to encode body: %w", err)
	}
	if err := qpWriter.Close(); err != nil {
		return nil, fmt.Errorf("failed to close quoted-printable writer: %w", err)
	}
	fmt.Fprintf(&buf, "\r\n")

	for _, att := range msg.Attachments {
		fmt.Fprintf(&buf, "--%s\r\n", boundary)
		fmt.Fprintf(&buf, "Content-Type: %s\r\n", att.ContentType)
		fmt.Fprintf(&buf, "Content-Disposition: attachment; filename=\"%s\"\r\n", att.Filename)
		fmt.Fprintf(&buf, "Content-Transfer-Encoding: base64\r\n")
		fmt.Fprintf(&buf, "\r\n")

		encoded := make([]byte, base64.StdEncoding.EncodedLen(len(att.Data)))
		base64.StdEncoding.Encode(encoded, att.Data)
		for i := 0; i < len(encoded); i += 76 {
			end := i + 76
			if end > len(encoded) {
				end = len(encoded)
			}
			buf.Write(encoded[i:end])
			buf.WriteString("\r\n")
		}
	}

	if hasAttachments {
		fmt.Fprintf(&buf, "--%s--\r\n", boundary)
	}

	return buf.Bytes(), nil
}

func randomBoundary() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("boundary-%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("----=_Part_%s", hex.EncodeToString(b))
}

// mimeEncodeHeader applies RFC 2047 Base64 encoding for non-ASCII header values.
func mimeEncodeHeader(s string) string {
	for _, r := range s {
		if r > 127 {
			return fmt.Sprintf("=?UTF-8?B?%s?=", base64.StdEncoding.EncodeToString([]byte(s)))
		}
	}
	return s
}

// GenerateMessageID creates a unique RFC 2822 Message-ID.
func GenerateMessageID(host string) string {
	if host == "" {
		host = "localhost"
	}
	return fmt.Sprintf("<%d.%s@%s>", time.Now().UnixNano(), RandomString(8), host)
}

// RandomString generates a random hex string of the given length.
func RandomString(n int) string {
	b := make([]byte, (n+1)/2)
	if _, err := rand.Read(b); err != nil {
		// Fallback: use time-based seed per byte position
		for i := range b {
			b[i] = byte(time.Now().UnixNano() >> (uint(i) * 8))
		}
	}
	return hex.EncodeToString(b)[:n]
}
