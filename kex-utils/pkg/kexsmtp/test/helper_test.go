package kexsmtp_test

import (
	"strings"
	"testing"

	"github.com/roidmc/kex-utils/pkg/kexsmtp"
)

func TestBuildMessageBody(t *testing.T) {
	msg := &kexsmtp.Message{
		From:        "sender@example.com",
		To:          []string{"recipient@example.com"},
		Cc:          []string{"cc@example.com"},
		Subject:     "Test Subject",
		Body:        "Hello World",
		ContentType: kexsmtp.ContentTypeTextPlain,
		ReplyTo:     "reply@example.com",
	}

	body, err := kexsmtp.BuildMessageBody("sender@example.com", msg, "<test-id@localhost>")
	if err != nil {
		t.Fatalf("BuildMessageBody() error = %v", err)
	}

	bodyStr := string(body)

	if !strings.Contains(bodyStr, "From: sender@example.com") {
		t.Error("body missing From header")
	}
	if !strings.Contains(bodyStr, "To: recipient@example.com") {
		t.Error("body missing To header")
	}
	if !strings.Contains(bodyStr, "Cc: cc@example.com") {
		t.Error("body missing Cc header")
	}
	if !strings.Contains(bodyStr, "Subject: Test Subject") {
		t.Error("body missing Subject header")
	}
	if !strings.Contains(bodyStr, "Reply-To: reply@example.com") {
		t.Error("body missing Reply-To header")
	}
	if !strings.Contains(bodyStr, "Hello World") {
		t.Error("body missing content")
	}
	if !strings.Contains(bodyStr, "Message-Id: <test-id@localhost>") {
		t.Error("body missing or wrong Message-Id")
	}
}

func TestBuildMessageBody_WithAttachments(t *testing.T) {
	msg := &kexsmtp.Message{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test with Attachment",
		Body:    "See attached",
		Attachments: []kexsmtp.Attachment{
			{
				Filename:    "test.txt",
				ContentType: "text/plain",
				Data:        []byte("attachment content"),
			},
		},
	}

	body, err := kexsmtp.BuildMessageBody("sender@example.com", msg, "<test-id@localhost>")
	if err != nil {
		t.Fatalf("BuildMessageBody() error = %v", err)
	}

	bodyStr := string(body)

	if !strings.Contains(bodyStr, "multipart/mixed") {
		t.Error("body should contain multipart/mixed content type")
	}
	if !strings.Contains(bodyStr, "filename=\"test.txt\"") {
		t.Error("body missing attachment filename")
	}
}

func TestBuildMessageBody_DefaultContentType(t *testing.T) {
	msg := &kexsmtp.Message{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "Test",
		Body:    "Hello",
	}

	body, err := kexsmtp.BuildMessageBody("sender@example.com", msg, "<test-id@localhost>")
	if err != nil {
		t.Fatalf("BuildMessageBody() error = %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "text/plain") {
		t.Error("body should default to text/plain content type")
	}
}

func TestBuildMessageBody_ChineseSubject(t *testing.T) {
	msg := &kexsmtp.Message{
		From:    "sender@example.com",
		To:      []string{"recipient@example.com"},
		Subject: "测试邮件",
		Body:    "Hello",
	}

	body, err := kexsmtp.BuildMessageBody("sender@example.com", msg, "<test-id@localhost>")
	if err != nil {
		t.Fatalf("BuildMessageBody() error = %v", err)
	}

	bodyStr := string(body)
	if !strings.Contains(bodyStr, "=?UTF-8?B?") {
		t.Error("Chinese subject should be RFC 2047 Base64 encoded")
	}
}
