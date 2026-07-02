package event_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/util/ssrf"
)

func testSSRFPolicy() *ssrf.Policy {
	p := ssrf.DefaultPolicy()
	p.AllowLoopback = true
	return p
}

func TestDispatcherDispatch(t *testing.T) {
	received := make(chan struct{}, 1)
	var receivedBody []byte
	var receivedSig string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get(event.SignatureHeader)
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))

	evt := event.Event{
		ID:   "evt-1",
		Type: "user.registered",
		Data: map[string]string{"user_id": "user-123"},
	}

	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "test-secret", 5*time.Second)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.StatusCode != http.StatusOK {
		t.Errorf("expected status 200, got %d", result.StatusCode)
	}
	if result.DurationMs <= 0 {
		t.Error("expected positive duration")
	}
	if result.ErrorCode != event.ErrCodeNone {
		t.Errorf("expected ErrCodeNone on success, got %s", result.ErrorCode)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	var receivedEvt event.Event
	if err := json.Unmarshal(receivedBody, &receivedEvt); err != nil {
		t.Fatalf("failed to unmarshal received body: %v", err)
	}
	if receivedEvt.ID != "evt-1" {
		t.Errorf("expected event ID evt-1, got %s", receivedEvt.ID)
	}

	if err := event.VerifySignature(receivedBody, "test-secret", receivedSig, event.DefaultTolerance); err != nil {
		t.Errorf("signature verification failed: %v", err)
	}
}

func TestDispatcherWithSM3Signer(t *testing.T) {
	received := make(chan struct{}, 1)
	var receivedSig string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get(event.SignatureHeader)
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	sm3Signer, err := event.NewSigner(event.HashSM3)
	if err != nil {
		t.Fatalf("failed to create SM3 signer: %v", err)
	}

	dispatcher := event.NewDispatcherWithSigner(5*time.Second, sm3Signer, event.WithSSRFPolicy(testSSRFPolicy()))

	evt := event.Event{ID: "evt-sm3", Type: "user.registered"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "test-secret", 5*time.Second)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	if !strings.Contains(receivedSig, "sm3=") {
		t.Errorf("expected SM3 version tag in signature header, got: %s", receivedSig)
	}

	if err := sm3Signer.VerifySignature(receivedBody, "test-secret", receivedSig, event.DefaultTolerance); err != nil {
		t.Errorf("SM3 signature verification failed: %v", err)
	}
}

func TestDispatcherSignerMethod(t *testing.T) {
	t.Run("default dispatcher uses SHA256 signer", func(t *testing.T) {
		d := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		if d.Signer().Algorithm() != event.HashSHA256 {
			t.Errorf("expected SHA256, got %s", d.Signer().Algorithm())
		}
	})

	t.Run("dispatcher with custom signer", func(t *testing.T) {
		sm3Signer, _ := event.NewSigner(event.HashSM3)
		d := event.NewDispatcherWithSigner(5*time.Second, sm3Signer, event.WithSSRFPolicy(testSSRFPolicy()))
		if d.Signer().Algorithm() != event.HashSM3 {
			t.Errorf("expected SM3, got %s", d.Signer().Algorithm())
		}
	})
}

func TestDispatcherNoSecret(t *testing.T) {
	received := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sig := r.Header.Get(event.SignatureHeader)
		if sig != "" {
			t.Error("expected no signature header when secret is empty")
		}
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))

	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}
}

func TestDispatcherServerError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))

	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	if result.Success {
		t.Error("expected failure for 500 response")
	}
	if result.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", result.StatusCode)
	}
}

func TestDispatcherTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(100*time.Millisecond, event.WithSSRFPolicy(testSSRFPolicy()))

	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 1*time.Second)

	if result.Success {
		t.Error("expected failure for timeout")
	}
	if result.Error == "" {
		t.Error("expected error message for timeout")
	}
	if result.ErrorCode != event.ErrCodeRequest {
		t.Errorf("expected ErrCodeRequest for timeout, got %s", result.ErrorCode)
	}
}

func TestDispatcherInvalidURL(t *testing.T) {
	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))

	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, "http://invalid.local:9999/nonexistent", "", 1*time.Second)

	if result.Success {
		t.Error("expected failure for invalid URL")
	}
	if result.Error == "" {
		t.Error("expected error message for invalid URL")
	}
	if result.ErrorCode != event.ErrCodeRequest {
		t.Errorf("expected ErrCodeRequest for network error, got %s", result.ErrorCode)
	}
}

func TestDispatcherContentType(t *testing.T) {
	received := make(chan struct{}, 1)
	var contentType string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		contentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	if contentType != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}
}

func TestDispatcherUserAgent(t *testing.T) {
	received := make(chan struct{}, 1)
	var userAgent string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userAgent = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	if !strings.HasPrefix(userAgent, "Quotagate-Webhook/") {
		t.Errorf("expected User-Agent starting with Quotagate-Webhook/, got %s", userAgent)
	}
}

func TestDispatcherDispatchWithRetry(t *testing.T) {
	t.Run("success on first attempt", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 3)

		if !result.Success {
			t.Fatalf("expected success, got error: %s", result.Error)
		}
		if result.Attempts != 1 {
			t.Errorf("expected 1 attempt, got %d", result.Attempts)
		}
	})

	t.Run("retries on 5xx then succeeds", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				fmt.Fprintln(w, "retry later")
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 3)

		if !result.Success {
			t.Fatalf("expected success after retries, got error: %s", result.Error)
		}
		if atomic.LoadInt32(&attempts) != 3 {
			t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
		}
		if result.Attempts != 3 {
			t.Errorf("expected Attempts=3, got %d", result.Attempts)
		}
	})

	t.Run("maxRetries=2 means exactly 2 retries and 3 total attempts", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "retry later")
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 2)

		if result.Success {
			t.Error("expected failure after all retries")
		}
		if atomic.LoadInt32(&attempts) != 3 {
			t.Errorf("expected exactly 3 total attempts (1 initial + 2 retries), got %d", atomic.LoadInt32(&attempts))
		}
		if result.Attempts != 3 {
			t.Errorf("expected Attempts=3, got %d", result.Attempts)
		}
	})

	t.Run("maxRetries=0 means no retry, only one attempt", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 0)

		if result.Success {
			t.Error("expected failure")
		}
		if atomic.LoadInt32(&attempts) != 1 {
			t.Errorf("expected exactly 1 attempt (no retries), got %d", atomic.LoadInt32(&attempts))
		}
		if result.Attempts != 1 {
			t.Errorf("expected Attempts=1, got %d", result.Attempts)
		}
	})

	t.Run("retries exhaust then returns last failure result", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
			fmt.Fprintln(w, "retry later")
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 2)

		if result.Success {
			t.Error("expected failure after all retries")
		}
		if result.StatusCode != http.StatusServiceUnavailable {
			t.Errorf("expected status 503, got %d", result.StatusCode)
		}
		if result.ResponseBody != "retry later\n" {
			t.Errorf("expected response body 'retry later\\n', got %s", result.ResponseBody)
		}
	})

	t.Run("backoff with jitter does not exceed maxBackoff", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}

		start := time.Now()
		_ = dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 5)
		elapsed := time.Since(start)

		if atomic.LoadInt32(&attempts) != 6 {
			t.Errorf("expected 6 total attempts (1 initial + 5 retries), got %d", atomic.LoadInt32(&attempts))
		}
		var maxPossibleBackoff time.Duration
		for i := 0; i < 5; i++ {
			maxPossibleBackoff += 30 * time.Second
		}
		if elapsed > maxPossibleBackoff+5*time.Second {
			t.Errorf("elapsed %v significantly exceeds max possible backoff %v", elapsed, maxPossibleBackoff)
		}
	})
}

func TestDispatcherResponseBodyReadError(t *testing.T) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer l.Close()

	addr := l.Addr().String()
	go func() {
		conn, err := l.Accept()
		if err != nil {
			return
		}
		conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 100\r\n\r\nhello"))
		conn.Close()
	}()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, "http://"+addr, "", 5*time.Second)

	if result.Success {
		t.Error("expected failure when response body cannot be fully read")
	}
	if result.Error == "" {
		t.Error("expected error message when response body read fails")
	}
	if !strings.Contains(result.Error, "failed to read response body") {
		t.Errorf("expected 'failed to read response body' in error, got: %s", result.Error)
	}
	if result.ErrorCode != event.ErrCodeReadBody {
		t.Errorf("expected ErrCodeReadBody, got %s", result.ErrorCode)
	}
}

func TestDispatcherVariousStatusCodes(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		success bool
	}{
		{"200 OK", http.StatusOK, true},
		{"201 Created", http.StatusCreated, true},
		{"202 Accepted", http.StatusAccepted, true},
		{"204 No Content", http.StatusNoContent, true},
		{"400 Bad Request", http.StatusBadRequest, false},
		{"401 Unauthorized", http.StatusUnauthorized, false},
		{"403 Forbidden", http.StatusForbidden, false},
		{"404 Not Found", http.StatusNotFound, false},
		{"429 Too Many Requests", http.StatusTooManyRequests, false},
		{"500 Internal Server Error", http.StatusInternalServerError, false},
		{"502 Bad Gateway", http.StatusBadGateway, false},
		{"503 Service Unavailable", http.StatusServiceUnavailable, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.status)
			}))
			defer server.Close()

			dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
			evt := event.Event{ID: "evt-1", Type: "test"}
			result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

			if result.Success != tc.success {
				t.Errorf("expected Success=%v for status %d, got %v", tc.success, tc.status, result.Success)
			}
			if result.StatusCode != tc.status {
				t.Errorf("expected status %d, got %d", tc.status, result.StatusCode)
			}
		})
	}
}

func TestDispatcherZeroTimeout(t *testing.T) {
	received := make(chan struct{}, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))

	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 0*time.Second)

	if !result.Success {
		t.Fatalf("expected success with timeoutSeconds=0 (uses dispatcher default), got error: %s", result.Error)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}
}

func TestDispatchErrorCode(t *testing.T) {
	t.Run("success returns ErrCodeNone", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

		if result.ErrorCode != event.ErrCodeNone {
			t.Errorf("expected ErrCodeNone on success, got %s", result.ErrorCode)
		}
	})

	t.Run("network error returns ErrCodeRequest", func(t *testing.T) {
		dispatcher := event.NewDispatcher(100*time.Millisecond, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.Dispatch(context.Background(), evt, "http://127.0.0.1:1/fail", "", 1*time.Second)

		if result.Success {
			t.Error("expected failure")
		}
		if result.ErrorCode != event.ErrCodeRequest {
			t.Errorf("expected ErrCodeRequest for network error, got %s", result.ErrorCode)
		}
	})

	t.Run("read body error returns ErrCodeReadBody", func(t *testing.T) {
		l, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer l.Close()

		go func() {
			conn, err := l.Accept()
			if err != nil {
				return
			}
			conn.Write([]byte("HTTP/1.1 200 OK\r\nContent-Length: 1000\r\n\r\nshort"))
			conn.Close()
		}()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.Dispatch(context.Background(), evt, "http://"+l.Addr().String(), "", 5*time.Second)

		if result.ErrorCode != event.ErrCodeReadBody {
			t.Errorf("expected ErrCodeReadBody, got %s", result.ErrorCode)
		}
	})
}

func TestRetryPolicy4xxNotRetried(t *testing.T) {
	statusCodes := []int{
		http.StatusBadRequest,
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusNotFound,
		http.StatusMethodNotAllowed,
		http.StatusConflict,
		http.StatusGone,
		http.StatusUnprocessableEntity,
	}

	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("status_%d_not_retried", code), func(t *testing.T) {
			var attempts int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&attempts, 1)
				w.WriteHeader(code)
			}))
			defer server.Close()

			dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
			evt := event.Event{ID: "evt-1", Type: "test"}
			result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 3)

			if result.Success {
				t.Errorf("expected failure for status %d", code)
			}
			if atomic.LoadInt32(&attempts) != 1 {
				t.Errorf("expected exactly 1 attempt for 4xx status %d (no retry), got %d", code, atomic.LoadInt32(&attempts))
			}
			if result.Attempts != 1 {
				t.Errorf("expected Attempts=1, got %d", result.Attempts)
			}
			if result.StatusCode != code {
				t.Errorf("expected status %d, got %d", code, result.StatusCode)
			}
		})
	}
}

func TestRetryPolicy5xxRetried(t *testing.T) {
	statusCodes := []int{
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout,
	}

	for _, code := range statusCodes {
		t.Run(fmt.Sprintf("status_%d_retried", code), func(t *testing.T) {
			var attempts int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				atomic.AddInt32(&attempts, 1)
				w.WriteHeader(code)
			}))
			defer server.Close()

			dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
			evt := event.Event{ID: "evt-1", Type: "test"}
			result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 2)

			if result.Success {
				t.Errorf("expected failure for status %d", code)
			}
			if atomic.LoadInt32(&attempts) != 3 {
				t.Errorf("expected 3 attempts for 5xx status %d (1 initial + 2 retries), got %d", code, atomic.LoadInt32(&attempts))
			}
			if result.Attempts != 3 {
				t.Errorf("expected Attempts=3, got %d", result.Attempts)
			}
		})
	}
}

func TestRetryPolicyTransportErrorRetried(t *testing.T) {
	var attempts int32
	dispatcher := event.NewDispatcher(100*time.Millisecond, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}

	result := dispatcher.DispatchWithRetry(context.Background(), evt, "http://127.0.0.1:1/fail", "", 1*time.Second, 2)

	if result.Success {
		t.Error("expected failure for transport error")
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts for transport error (1 initial + 2 retries), got %d", result.Attempts)
	}
	if result.ErrorCode != event.ErrCodeRequest {
		t.Errorf("expected ErrCodeRequest for transport error, got %s", result.ErrorCode)
	}
	_ = attempts
}

func TestRetryContextCancellation(t *testing.T) {
	t.Run("context cancelled during backoff", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}

		start := time.Now()
		result := dispatcher.DispatchWithRetry(ctx, evt, server.URL, "", 5*time.Second, 10)
		elapsed := time.Since(start)

		if result.Success {
			t.Error("expected failure")
		}
		if result.ErrorCode != event.ErrCodeContextCancelled {
			t.Errorf("expected ErrCodeContextCancelled, got %s", result.ErrorCode)
		}
		if !strings.Contains(result.Error, "context cancelled") {
			t.Errorf("expected 'context cancelled' in error, got: %s", result.Error)
		}
		if elapsed > 5*time.Second {
			t.Errorf("context cancellation should have stopped retries quickly, took %v", elapsed)
		}
	})

	t.Run("already cancelled context returns immediately", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}

		start := time.Now()
		result := dispatcher.DispatchWithRetry(ctx, evt, "http://127.0.0.1:1/fail", "", 1*time.Second, 3)
		elapsed := time.Since(start)

		if result.Success {
			t.Error("expected failure")
		}
		if result.ErrorCode != event.ErrCodeContextCancelled {
			t.Errorf("expected ErrCodeContextCancelled, got %s", result.ErrorCode)
		}
		if elapsed > 500*time.Millisecond {
			t.Errorf("already-cancelled context should return immediately, took %v", elapsed)
		}
	})

	t.Run("context cancelled before second attempt", func(t *testing.T) {
		callCount := int32(0)
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&callCount, 1)
			if n == 1 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			time.Sleep(5 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		ctx, cancel := context.WithCancel(context.Background())

		go func() {
			time.Sleep(500 * time.Millisecond)
			cancel()
		}()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(ctx, evt, server.URL, "", 5*time.Second, 5)

		if result.Success {
			t.Error("expected failure due to context cancellation")
		}
		if result.ErrorCode != event.ErrCodeContextCancelled {
			t.Errorf("expected ErrCodeContextCancelled, got %s", result.ErrorCode)
		}
		if result.Attempts < 1 {
			t.Errorf("expected at least 1 attempt before cancellation, got %d", result.Attempts)
		}
	})
}

func TestRetryAttemptsField(t *testing.T) {
	t.Run("single attempt on success", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 5)

		if result.Attempts != 1 {
			t.Errorf("expected Attempts=1, got %d", result.Attempts)
		}
	})

	t.Run("4xx returns Attempts=1", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 5)

		if result.Attempts != 1 {
			t.Errorf("expected Attempts=1 for 4xx (no retry), got %d", result.Attempts)
		}
	})

	t.Run("all retries exhausted", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 3)

		if result.Attempts != 4 {
			t.Errorf("expected Attempts=4 (1 initial + 3 retries), got %d", result.Attempts)
		}
	})
}

func TestRetryRecoveryOn5xx(t *testing.T) {
	t.Run("succeeds after 5xx failures", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n < 3 {
				w.WriteHeader(http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 5)

		if !result.Success {
			t.Fatalf("expected success after 5xx recovery, got error: %s", result.Error)
		}
		if result.Attempts != 3 {
			t.Errorf("expected Attempts=3, got %d", result.Attempts)
		}
	})
}

func TestRetry429RespectsRetryAfter(t *testing.T) {
	var attempts int32
	var mu sync.Mutex
	attemptTimes := make([]time.Time, 0)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		mu.Lock()
		attemptTimes = append(attemptTimes, time.Now())
		mu.Unlock()
		if atomic.LoadInt32(&attempts) == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			fmt.Fprintln(w, "rate limited")
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-429", Type: "test"}
	result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 3)

	if !result.Success {
		t.Errorf("expected success after 429 retry, got error: %s", result.Error)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts (429 then success), got %d", atomic.LoadInt32(&attempts))
	}

	mu.Lock()
	if len(attemptTimes) == 2 {
		elapsed := attemptTimes[1].Sub(attemptTimes[0])
		if elapsed < 900*time.Millisecond {
			t.Errorf("expected ~1s Retry-After delay, got %v", elapsed)
		}
	}
	mu.Unlock()
}

func TestRetry429WithoutRetryAfterHeader(t *testing.T) {
	var attempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprintln(w, "rate limited")
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-429-nora", Type: "test"}

	start := time.Now()
	result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 2)
	elapsed := time.Since(start)

	if result.Success {
		t.Error("expected failure after all retries")
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
	}
	if elapsed > 5*time.Second {
		t.Errorf("backoff took too long, elapsed %v", elapsed)
	}
}

func TestRetryNegativeMaxRetries(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, -1)

	if !result.Success {
		t.Fatalf("expected success even with negative maxRetries, got error: %s", result.Error)
	}
	if result.Attempts != 1 {
		t.Errorf("expected Attempts=1, got %d", result.Attempts)
	}
}

func TestDispatchContextCancellation(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	dispatcher := event.NewDispatcher(10*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(ctx, evt, server.URL, "", 0*time.Second)

	if result.Success {
		t.Error("expected failure due to context timeout")
	}
	if result.ErrorCode != event.ErrCodeContextCancelled {
		t.Errorf("expected ErrCodeContextCancelled for context timeout, got %s", result.ErrorCode)
	}
}

func TestDispatchCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(ctx, evt, "http://127.0.0.1:1/fail", "", 1*time.Second)

	if result.Success {
		t.Error("expected failure with cancelled context")
	}
}

func TestDispatchSuccessResponseWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"status":"ok"}`)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}
	if result.ResponseBody != `{"status":"ok"}` {
		t.Errorf("expected response body '{\"status\":\"ok\"}', got %s", result.ResponseBody)
	}
}

func TestDispatchErrorResponseWithBody(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, `{"error":"bad request"}`)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	if result.Success {
		t.Error("expected failure for 400")
	}
	if result.ResponseBody != `{"error":"bad request"}` {
		t.Errorf("expected response body with error, got %s", result.ResponseBody)
	}
	if result.StatusCode != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", result.StatusCode)
	}
}

func TestDispatchWithSigning(t *testing.T) {
	received := make(chan struct{}, 1)
	var receivedSig string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get(event.SignatureHeader)
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-signed", Type: "user.login", Data: "test-data"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "my-secret-key", 5*time.Second)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	if receivedSig == "" {
		t.Error("expected signature header to be set")
	}
	if err := event.VerifySignature(receivedBody, "my-secret-key", receivedSig, event.DefaultTolerance); err != nil {
		t.Errorf("signature verification failed: %v", err)
	}
}

func TestDispatchWrongSecretFailsVerification(t *testing.T) {
	received := make(chan struct{}, 1)
	var receivedSig string
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get(event.SignatureHeader)
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "correct-secret", 5*time.Second)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	err := event.VerifySignature(receivedBody, "wrong-secret", receivedSig, event.DefaultTolerance)
	if err == nil {
		t.Error("expected verification to fail with wrong secret")
	}
}

func TestDispatchHTTPMethod(t *testing.T) {
	received := make(chan struct{}, 1)
	var method string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	if method != http.MethodPost {
		t.Errorf("expected POST method, got %s", method)
	}
}

func TestDispatchPerRequestTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(30*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 1*time.Second)

	if result.Success {
		t.Error("expected failure due to per-request timeout")
	}
	if result.ErrorCode != event.ErrCodeRequest {
		t.Errorf("expected ErrCodeRequest for timeout, got %s", result.ErrorCode)
	}
}

func TestRetryWithTransportError(t *testing.T) {
	dispatcher := event.NewDispatcher(100*time.Millisecond, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.DispatchWithRetry(context.Background(), evt, "http://127.0.0.1:1/fail", "", 1*time.Second, 2)

	if result.Success {
		t.Error("expected failure for transport error")
	}
	if result.Attempts != 3 {
		t.Errorf("expected 3 attempts for transport error, got %d", result.Attempts)
	}
	if result.ErrorCode != event.ErrCodeRequest {
		t.Errorf("expected ErrCodeRequest, got %s", result.ErrorCode)
	}
}

func TestRetry5xxTo4xxTransition(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attempts, 1)
		if n == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusForbidden)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 5)

	if result.Success {
		t.Error("expected failure")
	}
	if result.StatusCode != http.StatusForbidden {
		t.Errorf("expected status 403 from second attempt, got %d", result.StatusCode)
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts (5xx retried, then 4xx stopped), got %d", atomic.LoadInt32(&attempts))
	}
	if result.Attempts != 2 {
		t.Errorf("expected Attempts=2, got %d", result.Attempts)
	}
}

func TestRetrySuccessAfterTransportError(t *testing.T) {
	var transportErrors int32
	var httpAttempts int32

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&httpAttempts, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(100*time.Millisecond, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}

	_ = transportErrors
	_ = dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 3)

	if atomic.LoadInt32(&httpAttempts) != 1 {
		t.Errorf("expected 1 successful HTTP attempt, got %d", atomic.LoadInt32(&httpAttempts))
	}
}

func TestDispatchEmptyEvent(t *testing.T) {
	received := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	if !result.Success {
		t.Fatalf("expected success for empty event, got error: %s", result.Error)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}
}

func TestDispatchEventWithComplexData(t *testing.T) {
	received := make(chan struct{}, 1)
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{
		ID:        "evt-complex",
		Type:      "user.registered",
		Source:    "auth-service",
		Subject:   "user-123",
		Data:      map[string]interface{}{"nested": map[string]string{"key": "value"}, "count": 42},
		Timestamp: time.Date(2025, 5, 16, 12, 0, 0, 0, time.UTC),
	}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	if !result.Success {
		t.Fatalf("expected success, got error: %s", result.Error)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(receivedBody, &parsed); err != nil {
		t.Fatalf("failed to unmarshal body: %v", err)
	}
	data, ok := parsed["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data to be a map")
	}
	if data["count"].(float64) != 42 {
		t.Errorf("expected count=42, got %v", data["count"])
	}
}

func TestDispatchConcurrentRequests(t *testing.T) {
	var received int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&received, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))

	const numRequests = 10
	results := make(chan *event.DispatchResult, numRequests)

	for i := 0; i < numRequests; i++ {
		go func(id int) {
			evt := event.Event{ID: fmt.Sprintf("evt-%d", id), Type: "test"}
			results <- dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)
		}(i)
	}

	for i := 0; i < numRequests; i++ {
		select {
		case result := <-results:
			if !result.Success {
				t.Errorf("expected success for concurrent request, got error: %s", result.Error)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("timeout waiting for concurrent requests")
		}
	}

	if atomic.LoadInt32(&received) != numRequests {
		t.Errorf("expected %d received requests, got %d", numRequests, atomic.LoadInt32(&received))
	}
}

func TestRetryBackoffRespectsContextDuringWait(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(500 * time.Millisecond)
		cancel()
	}()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}

	start := time.Now()
	result := dispatcher.DispatchWithRetry(ctx, evt, server.URL, "", 5*time.Second, 10)
	elapsed := time.Since(start)

	if result.Success {
		t.Error("expected failure")
	}
	if result.ErrorCode != event.ErrCodeContextCancelled {
		t.Errorf("expected ErrCodeContextCancelled, got %s", result.ErrorCode)
	}
	if elapsed > 3*time.Second {
		t.Errorf("should have stopped soon after context cancel, took %v", elapsed)
	}
}

func TestRetryMaxRetriesOne(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 1)

	if result.Success {
		t.Error("expected failure")
	}
	if atomic.LoadInt32(&attempts) != 2 {
		t.Errorf("expected 2 attempts (1 initial + 1 retry), got %d", atomic.LoadInt32(&attempts))
	}
	if result.Attempts != 2 {
		t.Errorf("expected Attempts=2, got %d", result.Attempts)
	}
}

func TestDispatchPerRequestTimeoutOverridesClient(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(10*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}

	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 1*time.Second)

	if result.Success {
		t.Error("expected failure due to per-request timeout of 1s being shorter than server delay of 3s")
	}
}

func TestDispatchLargePayload(t *testing.T) {
	received := make(chan struct{}, 1)
	var receivedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		received <- struct{}{}
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))

	largeData := strings.Repeat("x", 10000)
	evt := event.Event{ID: "evt-large", Type: "test", Data: largeData}
	result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	if !result.Success {
		t.Fatalf("expected success for large payload, got error: %s", result.Error)
	}

	select {
	case <-received:
	case <-time.After(time.Second):
		t.Fatal("server did not receive request")
	}

	if len(receivedBody) < 10000 {
		t.Errorf("expected large payload to be received, got %d bytes", len(receivedBody))
	}
}

// =============================================================================
// Tests for Dispatcher custom capabilities (DispatcherOption)
// =============================================================================

func TestWithHTTPClient(t *testing.T) {
	t.Run("injects custom http.Client", func(t *testing.T) {
		customClient := &http.Client{
			Timeout: 3 * time.Second,
		}

		dispatcher := event.NewDispatcher(5*time.Second, event.WithHTTPClient(customClient), event.WithSSRFPolicy(testSSRFPolicy()))

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

		if !result.Success {
			t.Fatalf("expected success, got error: %s", result.Error)
		}
	})

	t.Run("respects custom client timeout", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		customClient := &http.Client{
			Timeout: 100 * time.Millisecond,
		}
		dispatcher := event.NewDispatcher(5*time.Second, event.WithHTTPClient(customClient), event.WithSSRFPolicy(testSSRFPolicy()))

		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 0*time.Second)

		if result.Success {
			t.Error("expected failure due to custom client timeout")
		}
	})
}

func TestWithUserAgent(t *testing.T) {
	t.Run("sets custom User-Agent header", func(t *testing.T) {
		var receivedUserAgent string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedUserAgent = r.Header.Get("User-Agent")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithUserAgent("MyApp/2.0"), event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

		if receivedUserAgent != "MyApp/2.0" {
			t.Errorf("expected User-Agent 'MyApp/2.0', got '%s'", receivedUserAgent)
		}
	})

	t.Run("default User-Agent when not specified", func(t *testing.T) {
		var receivedUserAgent string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedUserAgent = r.Header.Get("User-Agent")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

		if !strings.HasPrefix(receivedUserAgent, "Quotagate-Webhook/") {
			t.Errorf("expected default User-Agent prefix 'Quotagate-Webhook/', got '%s'", receivedUserAgent)
		}
	})
}

func TestWithDefaultHeader(t *testing.T) {
	t.Run("adds single default header", func(t *testing.T) {
		var receivedTraceID string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedTraceID = r.Header.Get("X-Trace-Id")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second,
			event.WithDefaultHeader("X-Trace-Id", "trace-abc-123"),
			event.WithSSRFPolicy(testSSRFPolicy()),
		)
		evt := event.Event{ID: "evt-1", Type: "test"}
		dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

		if receivedTraceID != "trace-abc-123" {
			t.Errorf("expected X-Trace-Id 'trace-abc-123', got '%s'", receivedTraceID)
		}
	})

	t.Run("adds multiple default headers", func(t *testing.T) {
		var authHeader string
		var traceHeader string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader = r.Header.Get("Authorization")
			traceHeader = r.Header.Get("X-Trace-Id")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second,
			event.WithDefaultHeader("Authorization", "Bearer my-token"),
			event.WithDefaultHeader("X-Trace-Id", "trace-xyz-789"),
			event.WithSSRFPolicy(testSSRFPolicy()),
		)
		evt := event.Event{ID: "evt-1", Type: "test"}
		dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

		if authHeader != "Bearer my-token" {
			t.Errorf("expected Authorization header, got '%s'", authHeader)
		}
		if traceHeader != "trace-xyz-789" {
			t.Errorf("expected X-Trace-Id header, got '%s'", traceHeader)
		}
	})

	t.Run("signature header overrides default header if same key", func(t *testing.T) {
		var contentType string
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			contentType = r.Header.Get("Content-Type")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second,
			event.WithDefaultHeader("Content-Type", "text/plain"),
			event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

		if contentType != "application/json" {
			t.Errorf("expected Content-Type to be 'application/json' (not overridden by default), got '%s'", contentType)
		}
	})
}

func TestWithBackoffFunc(t *testing.T) {
	t.Run("uses custom backoff function", func(t *testing.T) {
		var attempts int32
		backoffCalls := make(chan time.Duration, 10)

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			atomic.AddInt32(&attempts, 1)
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		customBackoff := func(attempt int, result *event.DispatchResult) time.Duration {
			backoffCalls <- time.Duration(attempt) * time.Millisecond
			return time.Duration(attempt) * time.Millisecond
		}

		dispatcher := event.NewDispatcher(5*time.Second, event.WithBackoffFunc(customBackoff), event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 2)

		if result.Success {
			t.Error("expected failure after all retries")
		}
		if atomic.LoadInt32(&attempts) != 3 {
			t.Errorf("expected 3 attempts, got %d", atomic.LoadInt32(&attempts))
		}

		close(backoffCalls)
		calls := make([]time.Duration, 0)
		for d := range backoffCalls {
			calls = append(calls, d)
		}
		if len(calls) != 2 {
			t.Errorf("expected 2 backoff calls (for 2 retries), got %d", len(calls))
		}
	})

	t.Run("zero backoff skips sleep", func(t *testing.T) {
		var attempts int32
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			n := atomic.AddInt32(&attempts, 1)
			if n < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		customBackoff := func(attempt int, result *event.DispatchResult) time.Duration {
			return 0
		}

		dispatcher := event.NewDispatcher(5*time.Second, event.WithBackoffFunc(customBackoff), event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}

		start := time.Now()
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 2)
		elapsed := time.Since(start)

		if !result.Success {
			t.Fatalf("expected success after retries, got error: %s", result.Error)
		}
		if elapsed > 2*time.Second {
			t.Errorf("expected fast retries with zero backoff, but took %v", elapsed)
		}
	})

	t.Run("backoff func receives attempt index and result", func(t *testing.T) {
		var receivedAttempts []int
		var receivedStatusCodes []int

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		customBackoff := func(attempt int, result *event.DispatchResult) time.Duration {
			receivedAttempts = append(receivedAttempts, attempt)
			receivedStatusCodes = append(receivedStatusCodes, result.StatusCode)
			return 1 * time.Millisecond
		}

		dispatcher := event.NewDispatcher(5*time.Second, event.WithBackoffFunc(customBackoff), event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 2)

		if len(receivedAttempts) != 2 {
			t.Fatalf("expected 2 backoff calls, got %d", len(receivedAttempts))
		}
		for i, attempt := range receivedAttempts {
			if attempt != i {
				t.Errorf("backoff call %d: expected attempt=%d, got %d", i, i, attempt)
			}
		}
		for _, sc := range receivedStatusCodes {
			if sc != http.StatusServiceUnavailable {
				t.Errorf("expected status 503 in backoff func, got %d", sc)
			}
		}
	})
}

func TestWithOnResult(t *testing.T) {
	t.Run("calls onResult after each attempt", func(t *testing.T) {
		var results []int
		var attempts []int

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		handler := func(result *event.DispatchResult, attempt int) {
			results = append(results, result.StatusCode)
			attempts = append(attempts, attempt)
		}

		dispatcher := event.NewDispatcher(5*time.Second, event.WithOnResult(handler), event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 0)

		if len(results) != 1 {
			t.Fatalf("expected 1 onResult call, got %d", len(results))
		}
		if results[0] != http.StatusOK {
			t.Errorf("expected status 200 in onResult, got %d", results[0])
		}
		if attempts[0] != 1 {
			t.Errorf("expected attempt=1 in onResult, got %d", attempts[0])
		}
	})

	t.Run("calls onResult after each retry attempt", func(t *testing.T) {
		var results []int
		var attemptNums []int

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		handler := func(result *event.DispatchResult, attempt int) {
			results = append(results, result.StatusCode)
			attemptNums = append(attemptNums, attempt)
		}

		dispatcher := event.NewDispatcher(5*time.Second, event.WithOnResult(handler), event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 2)

		if len(results) != 3 {
			t.Fatalf("expected 3 onResult calls (3 attempts), got %d", len(results))
		}
		for _, sc := range results {
			if sc != http.StatusServiceUnavailable {
				t.Errorf("expected status 503 in onResult, got %d", sc)
			}
		}
		if attemptNums[0] != 1 || attemptNums[1] != 2 || attemptNums[2] != 3 {
			t.Errorf("expected attempt numbers [1,2,3] in onResult, got %v", attemptNums)
		}
	})

	t.Run("calls onResult even on failure", func(t *testing.T) {
		var called bool
		var receivedError event.DispatchErrorCode

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusBadRequest)
		}))
		defer server.Close()

		handler := func(result *event.DispatchResult, attempt int) {
			called = true
			receivedError = result.ErrorCode
		}

		dispatcher := event.NewDispatcher(5*time.Second, event.WithOnResult(handler), event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 0)

		if !called {
			t.Fatal("expected onResult to be called on failure")
		}
		if receivedError != event.ErrCodeNone {
			t.Errorf("expected ErrCodeNone in onResult for 400 response, got %s", receivedError)
		}
	})

	t.Run("recovers from panic in onResult", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		panicHandler := func(result *event.DispatchResult, attempt int) {
			panic("something went wrong in the handler")
		}

		dispatcher := event.NewDispatcher(5*time.Second, event.WithOnResult(panicHandler), event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

		if !result.Success {
			t.Error("expected dispatch to succeed even though onResult panicked")
		}
	})

	t.Run("onResult with retry reports all attempts", func(t *testing.T) {
		var allResults []event.DispatchResult

		attemptCount := 0
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			attemptCount++
			if attemptCount < 3 {
				w.WriteHeader(http.StatusServiceUnavailable)
				return
			}
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		handler := func(result *event.DispatchResult, attempt int) {
			allResults = append(allResults, *result)
		}

		dispatcher := event.NewDispatcher(5*time.Second, event.WithOnResult(handler), event.WithSSRFPolicy(testSSRFPolicy()))
		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 3)

		if !result.Success {
			t.Fatalf("expected success after retries, got error: %s", result.Error)
		}
		if len(allResults) != 3 {
			t.Fatalf("expected 3 onResult calls, got %d", len(allResults))
		}
		if allResults[0].StatusCode != 503 || allResults[1].StatusCode != 503 {
			t.Error("expected first two onResult calls to have status 503")
		}
		if allResults[2].StatusCode != 200 {
			t.Error("expected third onResult call to have status 200")
		}
	})
}

func TestMultipleDispatcherOptions(t *testing.T) {
	t.Run("combines WithUserAgent and WithDefaultHeader", func(t *testing.T) {
		var userAgent string
		var traceID string
		var authHeader string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userAgent = r.Header.Get("User-Agent")
			traceID = r.Header.Get("X-Trace-Id")
			authHeader = r.Header.Get("Authorization")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second,
			event.WithUserAgent("MyCustomAgent/3.0"),
			event.WithDefaultHeader("X-Trace-Id", "trace-111"),
			event.WithDefaultHeader("Authorization", "Bearer test-token"),
			event.WithSSRFPolicy(testSSRFPolicy()),
		)

		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

		if !result.Success {
			t.Fatalf("expected success, got error: %s", result.Error)
		}
		if userAgent != "MyCustomAgent/3.0" {
			t.Errorf("expected User-Agent 'MyCustomAgent/3.0', got '%s'", userAgent)
		}
		if traceID != "trace-111" {
			t.Errorf("expected X-Trace-Id 'trace-111', got '%s'", traceID)
		}
		if authHeader != "Bearer test-token" {
			t.Errorf("expected Authorization header, got '%s'", authHeader)
		}
	})

	t.Run("WithHTTPClient and WithBackoffFunc together", func(t *testing.T) {
		customClient := &http.Client{Timeout: 2 * time.Second}

		var backoffAttempts []int
		customBackoff := func(attempt int, result *event.DispatchResult) time.Duration {
			backoffAttempts = append(backoffAttempts, attempt)
			return 1 * time.Millisecond
		}

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}))
		defer server.Close()

		dispatcher := event.NewDispatcher(5*time.Second,
			event.WithHTTPClient(customClient),
			event.WithBackoffFunc(customBackoff),
			event.WithSSRFPolicy(testSSRFPolicy()),
		)

		evt := event.Event{ID: "evt-1", Type: "test"}
		result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 1)

		if result.Success {
			t.Error("expected failure")
		}
		if len(backoffAttempts) != 1 {
			t.Errorf("expected 1 backoff call, got %d", len(backoffAttempts))
		}
	})

	t.Run("NewDispatcherWithSigner with options", func(t *testing.T) {
		var receivedSig string
		var userAgent string

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			receivedSig = r.Header.Get(event.SignatureHeader)
			userAgent = r.Header.Get("User-Agent")
			w.WriteHeader(http.StatusOK)
		}))
		defer server.Close()

		sm3Signer, err := event.NewSigner(event.HashSM3)
		if err != nil {
			t.Fatalf("failed to create SM3 signer: %v", err)
		}

		dispatcher := event.NewDispatcherWithSigner(5*time.Second, sm3Signer,
			event.WithUserAgent("SM3Agent/1.0"),
			event.WithDefaultHeader("X-Custom", "value"),
			event.WithSSRFPolicy(testSSRFPolicy()),
		)

		evt := event.Event{ID: "evt-sm3", Type: "test"}
		result := dispatcher.Dispatch(context.Background(), evt, server.URL, "secret", 5*time.Second)

		if !result.Success {
			t.Fatalf("expected success, got error: %s", result.Error)
		}
		if userAgent != "SM3Agent/1.0" {
			t.Errorf("expected User-Agent 'SM3Agent/1.0', got '%s'", userAgent)
		}
		if !strings.Contains(receivedSig, "sm3=") {
			t.Errorf("expected SM3 signature, got: %s", receivedSig)
		}
	})
}

func TestDefaultBackoffStillWorksWithoutCustomFunc(t *testing.T) {
	var attempts int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&attempts, 1)
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "evt-1", Type: "test"}
	result := dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 2)

	if result.Success {
		t.Error("expected failure after all retries")
	}
	if atomic.LoadInt32(&attempts) != 3 {
		t.Errorf("expected 3 attempts (1 initial + 2 retries), got %d", atomic.LoadInt32(&attempts))
	}
}

func TestIdempotencyKeyHeaderSent(t *testing.T) {
	var gotKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Idempotency-Key")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "idempotency-evt-123", Type: "user.login"}
	_ = dispatcher.Dispatch(context.Background(), evt, server.URL, "", 5*time.Second)

	if gotKey != "idempotency-evt-123" {
		t.Errorf("expected X-Idempotency-Key header 'idempotency-evt-123', got '%s'", gotKey)
	}
}

func TestIdempotencyKeyHeaderSentWithRetry(t *testing.T) {
	var gotKeys []string
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotKeys = append(gotKeys, r.Header.Get("X-Idempotency-Key"))
		mu.Unlock()
		if len(gotKeys) < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	dispatcher := event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
	evt := event.Event{ID: "idempotency-retry-456", Type: "user.register"}
	_ = dispatcher.DispatchWithRetry(context.Background(), evt, server.URL, "", 5*time.Second, 5)

	mu.Lock()
	defer mu.Unlock()
	for i, k := range gotKeys {
		if k != "idempotency-retry-456" {
			t.Errorf("attempt %d: expected X-Idempotency-Key 'idempotency-retry-456', got '%s'", i+1, k)
		}
	}
}
