package smtp_test

import (
	"context"
	"testing"
	"time"

	"github.com/roidmc/quotagate/pkg/smtp"
)

func TestNewStandardSMTP(t *testing.T) {
	mailer, err := smtp.NewStandardSMTP(smtp.StandardSMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	if err != nil {
		t.Fatalf("NewStandardSMTP() error = %v", err)
	}
	if mailer == nil {
		t.Fatal("NewStandardSMTP() returned nil")
	}
	defer mailer.Close()

	_, err = smtp.NewStandardSMTP(smtp.StandardSMTPConfig{
		Host: "",
	})
	if err == nil {
		t.Fatal("NewStandardSMTP() with empty host should error")
	}

	mailer2, err := smtp.NewStandardSMTP(smtp.StandardSMTPConfig{
		Host:     "smtp.example.com",
		Username: "user@example.com",
	})
	if err != nil {
		t.Fatalf("NewStandardSMTP() error = %v", err)
	}
	defer mailer2.Close()
}

func TestStandardSMTP_DefaultPort(t *testing.T) {
	mailer, err := smtp.NewStandardSMTP(smtp.StandardSMTPConfig{
		Host: "smtp.example.com",
	})
	if err != nil {
		t.Fatalf("NewStandardSMTP() error = %v", err)
	}
	defer mailer.Close()
}

func TestStandardSMTP_Close(t *testing.T) {
	mailer, _ := smtp.NewStandardSMTP(smtp.StandardSMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
	})

	if err := mailer.Close(); err != nil {
		t.Fatalf("Close() error = %v", err)
	}

	if err := mailer.Close(); err != nil {
		t.Fatalf("Close() second time error = %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	_, err := mailer.Send(ctx, &smtp.Message{
		To:      []string{"test@example.com"},
		Subject: "Test",
		Body:    "Hello",
	})
	if err != smtp.ErrMailerClosed {
		t.Fatalf("Send() after Close() error = %v, want ErrMailerClosed", err)
	}
}

func TestStandardSMTP_Ping_Closed(t *testing.T) {
	mailer, _ := smtp.NewStandardSMTP(smtp.StandardSMTPConfig{
		Host: "smtp.example.com",
		Port: 587,
	})
	mailer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := mailer.Ping(ctx)
	if err != smtp.ErrMailerClosed {
		t.Fatalf("Ping() after Close() error = %v, want ErrMailerClosed", err)
	}
}
