package kexsmtp_test

import (
	"context"
	"testing"

	"github.com/roidmc/kex-utils/pkg/kexsmtp"
)

func TestNewSESSMTP(t *testing.T) {
	mailer, err := kexsmtp.NewSESSMTP(kexsmtp.SESSMTPConfig{
		Username: "SMTP_USERNAME",
		Password: "SMTP_PASSWORD",
		Region:   "us-east-1",
		From:     "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("NewSESSMTP() error = %v", err)
	}
	if mailer == nil {
		t.Fatal("NewSESSMTP() returned nil")
	}
	defer mailer.Close()
}

func TestNewSESSMTP_DefaultRegion(t *testing.T) {
	mailer, err := kexsmtp.NewSESSMTP(kexsmtp.SESSMTPConfig{
		Username: "SMTP_USERNAME",
		Password: "SMTP_PASSWORD",
		From:     "noreply@example.com",
	})
	if err != nil {
		t.Fatalf("NewSESSMTP() error = %v", err)
	}
	defer mailer.Close()
}

func TestNewSESSMTP_MissingCredentials(t *testing.T) {
	_, err := kexsmtp.NewSESSMTP(kexsmtp.SESSMTPConfig{
		Region: "us-east-1",
		From:   "noreply@example.com",
	})
	if err == nil {
		t.Fatal("NewSESSMTP() with empty credentials should error")
	}
}

func TestNewSESSMTP_MissingFrom(t *testing.T) {
	_, err := kexsmtp.NewSESSMTP(kexsmtp.SESSMTPConfig{
		Username: "SMTP_USERNAME",
		Password: "SMTP_PASSWORD",
		Region:   "us-east-1",
	})
	if err == nil {
		t.Fatal("NewSESSMTP() with empty from should error")
	}
}

func TestNewSES_API_Unimplemented(t *testing.T) {
	_, err := kexsmtp.NewSES(kexsmtp.SESConfig{
		AccessKeyID:     "AKIA...",
		SecretAccessKey: "secret",
	})
	if err == nil {
		t.Fatal("NewSES() should return error (requires AWS SDK)")
	}
}

func TestSESMailer_Send_Unimplemented(t *testing.T) {
	// SESMailer is a placeholder, Send/Ping should return errors
	mailer := &kexsmtp.SESMailer{}
	ctx := context.Background()

	_, err := mailer.Send(ctx, &kexsmtp.Message{
		To:      []string{"test@example.com"},
		Subject: "Test",
		Body:    "Hello",
	})
	if err == nil {
		t.Error("SESMailer.Send() should return error")
	}
}
