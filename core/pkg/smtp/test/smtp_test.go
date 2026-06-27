package smtp_test

import (
	"testing"

	"github.com/roidmc/quotagate/pkg/smtp"
)

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     *smtp.Message
		wantErr bool
	}{
		{
			name:    "nil message",
			msg:     nil,
			wantErr: true,
		},
		{
			name: "no recipients",
			msg: &smtp.Message{
				Subject: "Test",
				Body:    "Hello",
			},
			wantErr: true,
		},
		{
			name: "valid message",
			msg: &smtp.Message{
				To:      []string{"test@example.com"},
				Subject: "Test",
				Body:    "Hello",
			},
			wantErr: false,
		},
		{
			name: "only bcc",
			msg: &smtp.Message{
				Bcc:     []string{"test@example.com"},
				Subject: "Test",
				Body:    "Hello",
			},
			wantErr: false,
		},
		{
			name: "empty subject and body but with attachment",
			msg: &smtp.Message{
				To: []string{"test@example.com"},
				Attachments: []smtp.Attachment{
					{Filename: "test.txt", ContentType: "text/plain", Data: []byte("test")},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := smtp.ValidateMessage(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateMessageID(t *testing.T) {
	id1 := smtp.GenerateMessageID("example.com")
	id2 := smtp.GenerateMessageID("example.com")

	if id1 == id2 {
		t.Error("GenerateMessageID should produce unique IDs")
	}

	if id1 == "" {
		t.Error("GenerateMessageID should not return empty string")
	}

	id3 := smtp.GenerateMessageID("")
	if id3 == "" {
		t.Error("GenerateMessageID with empty host should not return empty string")
	}
}

func TestRandomString(t *testing.T) {
	s1 := smtp.RandomString(8)
	s2 := smtp.RandomString(8)

	if len(s1) != 8 || len(s2) != 8 {
		t.Error("RandomString should return correct length")
	}

	if s1 == s2 {
		t.Error("RandomString should produce different strings")
	}
}

func TestMailerInterface(t *testing.T) {
	var _ smtp.Mailer = (*smtp.StandardSMTP)(nil)
	var _ smtp.Mailer = (*smtp.SendGridMailer)(nil)
	var _ smtp.Mailer = (*smtp.SESMailer)(nil)
}
