package smtp

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"time"
)

type SendGridConfig struct {
	APIKey   string
	From     string
	FromName string
}

type sendGridPersonalization struct {
	To      []sendGridEmail `json:"to"`
	Cc      []sendGridEmail `json:"cc,omitempty"`
	Bcc     []sendGridEmail `json:"bcc,omitempty"`
	Subject string          `json:"subject,omitempty"`
}

type sendGridEmail struct {
	Email string `json:"email"`
	Name  string `json:"name,omitempty"`
}

type sendGridContent struct {
	Type  string `json:"type"`
	Value string `json:"value"`
}

type sendGridAttachment struct {
	Content     string `json:"content"`
	Filename    string `json:"filename"`
	Type        string `json:"type"`
	Disposition string `json:"disposition"`
}

type sendGridMail struct {
	Personalizations []sendGridPersonalization `json:"personalizations"`
	From             sendGridEmail             `json:"from"`
	ReplyTo          *sendGridEmail            `json:"reply_to,omitempty"`
	Subject          string                    `json:"subject,omitempty"`
	Content          []sendGridContent         `json:"content,omitempty"`
	Attachments      []sendGridAttachment      `json:"attachments,omitempty"`
}

type SendGridMailer struct {
	config SendGridConfig
	client *http.Client
	closed atomic.Bool
}

func NewSendGrid(config SendGridConfig) (*SendGridMailer, error) {
	if config.APIKey == "" {
		return nil, ErrAuthFailed
	}
	return &SendGridMailer{
		config: config,
		client: &http.Client{Timeout: 30 * time.Second},
	}, nil
}

func (s *SendGridMailer) Send(ctx context.Context, msg *Message) (*SendResult, error) {
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

	sgMail := sendGridMail{
		From: sendGridEmail{
			Email: from,
			Name:  s.config.FromName,
		},
		Subject: msg.Subject,
	}

	if msg.ReplyTo != "" {
		sgMail.ReplyTo = &sendGridEmail{Email: msg.ReplyTo}
	}

	personalization := sendGridPersonalization{
		Subject: msg.Subject,
	}
	for _, to := range msg.To {
		personalization.To = append(personalization.To, sendGridEmail{Email: to})
	}
	for _, cc := range msg.Cc {
		personalization.Cc = append(personalization.Cc, sendGridEmail{Email: cc})
	}
	for _, bcc := range msg.Bcc {
		personalization.Bcc = append(personalization.Bcc, sendGridEmail{Email: bcc})
	}
	sgMail.Personalizations = append(sgMail.Personalizations, personalization)

	contentType := string(msg.ContentType)
	if contentType == "" {
		contentType = string(ContentTypeTextPlain)
	}
	sgMail.Content = append(sgMail.Content, sendGridContent{
		Type:  contentType,
		Value: msg.Body,
	})

	for _, att := range msg.Attachments {
		sgMail.Attachments = append(sgMail.Attachments, sendGridAttachment{
			Content:     base64.StdEncoding.EncodeToString(att.Data),
			Filename:    att.Filename,
			Type:        att.ContentType,
			Disposition: "attachment",
		})
	}

	payload, err := json.Marshal(sgMail)
	if err != nil {
		return nil, fmt.Errorf("kexsmtp: failed to marshal sendgrid payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.sendgrid.com/v3/mail/send", bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("kexsmtp: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("kexsmtp: sendgrid request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return nil, fmt.Errorf("kexsmtp: sendgrid returned status %d: %w", resp.StatusCode, ErrProviderError)
	}

	messageID := resp.Header.Get("X-Message-Id")
	if messageID == "" {
		messageID = GenerateMessageID("sendgrid.net")
	}

	return &SendResult{
		MessageID: messageID,
		SentAt:    time.Now().UTC(),
	}, nil
}

func (s *SendGridMailer) Ping(ctx context.Context) error {
	if s.closed.Load() {
		return ErrMailerClosed
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://api.sendgrid.com/v3/user/profile", nil)
	if err != nil {
		return fmt.Errorf("kexsmtp: ping failed: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("kexsmtp: ping failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("kexsmtp: ping returned status %d: %w", resp.StatusCode, ErrProviderError)
	}

	return nil
}

func (s *SendGridMailer) Close() error {
	if !s.closed.CompareAndSwap(false, true) {
		return nil
	}
	return nil
}
