package smtp_test

import (
	"context"
	"testing"
	"time"

	"github.com/roidmc/quotagate/pkg/smtp"
)

func TestNewSendGrid(t *testing.T) {
	mailer, err := smtp.NewSendGrid(smtp.SendGridConfig{
		APIKey: "SG.test",
		From:   "test@example.com",
	})
	if err != nil {
		t.Fatalf("NewSendGrid() error = %v", err)
	}
	if mailer == nil {
		t.Fatal("NewSendGrid() returned nil")
	}
	defer mailer.Close()

	_, err = smtp.NewSendGrid(smtp.SendGridConfig{
		APIKey: "",
	})
	if err == nil {
		t.Fatal("NewSendGrid() with empty API key should error")
	}
}

func TestSendGridMailer_Close(t *testing.T) {
	mailer, _ := smtp.NewSendGrid(smtp.SendGridConfig{
		APIKey: "SG.test",
		From:   "test@example.com",
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

func TestSendGridMailer_Ping_Closed(t *testing.T) {
	mailer, _ := smtp.NewSendGrid(smtp.SendGridConfig{
		APIKey: "SG.test",
		From:   "test@example.com",
	})
	mailer.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	err := mailer.Ping(ctx)
	if err != smtp.ErrMailerClosed {
		t.Fatalf("Ping() after Close() error = %v, want ErrMailerClosed", err)
	}
}
