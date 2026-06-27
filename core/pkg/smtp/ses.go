package smtp

import (
	"context"
	"fmt"
	"sync/atomic"
)

// SESSMTPConfig 是 SES SMTP 接口的配置
// 凭证需要在 SES 控制台生成 "SMTP credentials"，不是 IAM Access Key
type SESSMTPConfig struct {
	Username string // SES SMTP Username（非 IAM Access Key ID）
	Password string // SES SMTP Password（非 IAM Secret Access Key）
	Region   string // AWS Region，如 us-east-1
	From     string
}

// NewSESSMTP 创建一个通过 SES SMTP 接口发送邮件的 Mailer
// 零外部依赖，复用 StandardSMTP
func NewSESSMTP(config SESSMTPConfig) (*StandardSMTP, error) {
	if config.Username == "" || config.Password == "" {
		return nil, ErrAuthFailed
	}
	if config.Region == "" {
		config.Region = "us-east-1"
	}
	if config.From == "" {
		return nil, ErrInvalidMessage
	}

	host := fmt.Sprintf("email-smtp.%s.amazonaws.com", config.Region)

	return NewStandardSMTP(StandardSMTPConfig{
		Host:     host,
		Port:     465,
		Username: config.Username,
		Password: config.Password,
		From:     config.From,
		TLSMode:  TLSModeTLS,
	})
}

// SESConfig 保留用于 API 方式（需要 AWS SDK）
type SESConfig struct {
	AccessKeyID     string
	SecretAccessKey string
	Region          string
	From            string
}

// SESMailer 是 SES API 的占位实现，需要 AWS SDK 才能使用
type SESMailer struct {
	config SESConfig
	client interface{}
	closed atomic.Bool
}

func NewSES(config SESConfig) (*SESMailer, error) {
	return nil, fmt.Errorf("kexsmtp: SES API adapter requires AWS SDK, use NewSESSMTP() instead: %w", ErrProviderError)
}

func (s *SESMailer) Send(ctx context.Context, msg *Message) (*SendResult, error) {
	return nil, ErrProviderError
}

func (s *SESMailer) Ping(ctx context.Context) error {
	return ErrProviderError
}

func (s *SESMailer) Close() error {
	return nil
}
