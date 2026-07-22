package kexsmtp_test

import (
	"testing"

	"github.com/roidmc/kex-utils/pkg/kexsmtp"
)

func TestValidateMessage(t *testing.T) {
	tests := []struct {
		name    string
		msg     *kexsmtp.Message
		wantErr bool
	}{
		{
			name:    "nil message",
			msg:     nil,
			wantErr: true,
		},
		{
			name: "no recipients",
			msg: &kexsmtp.Message{
				Subject: "Test",
				Body:    "Hello",
			},
			wantErr: true,
		},
		{
			name: "valid message",
			msg: &kexsmtp.Message{
				To:      []string{"test@example.com"},
				Subject: "Test",
				Body:    "Hello",
			},
			wantErr: false,
		},
		{
			name: "only bcc",
			msg: &kexsmtp.Message{
				Bcc:     []string{"test@example.com"},
				Subject: "Test",
				Body:    "Hello",
			},
			wantErr: false,
		},
		{
			name: "empty subject and body but with attachment",
			msg: &kexsmtp.Message{
				To: []string{"test@example.com"},
				Attachments: []kexsmtp.Attachment{
					{Filename: "test.txt", ContentType: "text/plain", Data: []byte("test")},
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := kexsmtp.ValidateMessage(tt.msg)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateMessage() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestGenerateMessageID(t *testing.T) {
	id1 := kexsmtp.GenerateMessageID("example.com")
	id2 := kexsmtp.GenerateMessageID("example.com")

	if id1 == id2 {
		t.Error("GenerateMessageID should produce unique IDs")
	}

	if id1 == "" {
		t.Error("GenerateMessageID should not return empty string")
	}

	id3 := kexsmtp.GenerateMessageID("")
	if id3 == "" {
		t.Error("GenerateMessageID with empty host should not return empty string")
	}
}

func TestRandomString(t *testing.T) {
	s1 := kexsmtp.RandomString(8)
	s2 := kexsmtp.RandomString(8)

	if len(s1) != 8 || len(s2) != 8 {
		t.Error("RandomString should return correct length")
	}

	if s1 == s2 {
		t.Error("RandomString should produce different strings")
	}
}

func TestMailerInterface(t *testing.T) {
	var _ kexsmtp.Mailer = (*kexsmtp.StandardSMTP)(nil)
	var _ kexsmtp.Mailer = (*kexsmtp.SendGridMailer)(nil)
	var _ kexsmtp.Mailer = (*kexsmtp.SESMailer)(nil)
}
