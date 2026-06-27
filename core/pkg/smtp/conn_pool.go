package smtp

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"sync/atomic"
	"time"
)

type ConnPool struct {
	config      StandardSMTPConfig
	addr        string
	pool        chan *smtp.Client
	activeCount int64
	maxIdle     int
	maxActive   int
	closed      atomic.Bool
}

func NewConnPool(config StandardSMTPConfig, maxIdle, maxActive int) (*ConnPool, error) {
	if maxIdle <= 0 {
		maxIdle = 5
	}
	if maxActive <= 0 {
		maxActive = 20
	}

	p := &ConnPool{
		config:    config,
		addr:      fmt.Sprintf("%s:%d", config.Host, config.Port),
		pool:      make(chan *smtp.Client, maxIdle),
		maxIdle:   maxIdle,
		maxActive: maxActive,
	}

	// Pre-heat connections
	for i := 0; i < maxIdle/2; i++ {
		client, err := p.dialNew(context.Background())
		if err != nil {
			continue
		}
		p.pool <- client
		atomic.AddInt64(&p.activeCount, 1)
	}
	return p, nil
}

// Get 从连接池获取一个 SMTP 客户端，池满时会阻塞等待
func (p *ConnPool) Get(ctx context.Context) (*smtp.Client, error) {
	if p.closed.Load() {
		return nil, ErrMailerClosed
	}

	// 尝试从池中获取
	select {
	case client := <-p.pool:
		// 检查连接是否存活
		if err := client.Noop(); err != nil {
			atomic.AddInt64(&p.activeCount, -1)
			client.Quit()
			return p.dialNew(ctx)
		}
		return client, nil
	default:
	}

	// 池为空，尝试新建连接
	if atomic.LoadInt64(&p.activeCount) < int64(p.maxActive) {
		return p.dialNew(ctx)
	}

	// 已达上限，阻塞等待归还
	select {
	case client := <-p.pool:
		if err := client.Noop(); err != nil {
			atomic.AddInt64(&p.activeCount, -1)
			client.Quit()
			return p.dialNew(ctx)
		}
		return client, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// Put 归还连接到池中，forceClose=true 时强制关闭（如发送失败）
func (p *ConnPool) Put(client *smtp.Client, forceClose bool) {
	if client == nil {
		return
	}

	if forceClose || p.closed.Load() {
		client.Quit()
		atomic.AddInt64(&p.activeCount, -1)
		return
	}

	select {
	case p.pool <- client:
	default:
		client.Quit()
		atomic.AddInt64(&p.activeCount, -1)
	}
}

func (p *ConnPool) Close() error {
	if !p.closed.CompareAndSwap(false, true) {
		return nil
	}

	close(p.pool)
	for client := range p.pool {
		client.Quit()
		atomic.AddInt64(&p.activeCount, -1)
	}
	return nil
}

// dialNew 建立新的 SMTP 连接（含 TLS/STARTTLS 和认证）
func (p *ConnPool) dialNew(ctx context.Context) (*smtp.Client, error) {
	dialer := &net.Dialer{Timeout: 30 * time.Second}
	conn, err := dialer.DialContext(ctx, "tcp", p.addr)
	if err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	var client *smtp.Client

	// TLS mode process
	switch p.config.TLSMode {
	case TLSModeTLS:
		tlsConfig := &tls.Config{ServerName: p.config.Host}
		tlsConn := tls.Client(conn, tlsConfig)
		if err := tlsConn.HandshakeContext(ctx); err != nil {
			conn.Close()
			return nil, fmt.Errorf("tls handshake failed: %w", err)
		}
		client, err = smtp.NewClient(tlsConn, p.config.Host)
	default:
		client, err = smtp.NewClient(conn, p.config.Host)
	}

	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("failed to create smtp client: %w", err)
	}

	// STARTTLS
	if p.config.TLSMode == TLSModeSTARTTLS {
		tlsConfig := &tls.Config{ServerName: p.config.Host}
		if err := client.StartTLS(tlsConfig); err != nil {
			client.Close()
			return nil, fmt.Errorf("starttls failed: %w", err)
		}
	}

	// Authenticate
	if p.config.Username != "" && p.config.Password != "" {
		auth := smtp.PlainAuth("", p.config.Username, p.config.Password, p.config.Host)
		if err := client.Auth(auth); err != nil {
			client.Close()
			return nil, fmt.Errorf("%w", ErrAuthFailed)
		}
	}

	atomic.AddInt64(&p.activeCount, 1)
	return client, nil
}
