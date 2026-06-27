package event

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/roidmc/quotagate/internal/util/ssrf"
)

// DispatchErrorCode classifies the type of failure that occurred during a
// webhook dispatch. Callers can use this for programmatic error handling
// instead of string-matching on DispatchResult.Error.
type DispatchErrorCode string

const (
	// ErrCodeNone indicates no error (Success is true).
	ErrCodeNone DispatchErrorCode = ""
	// ErrCodeMarshal indicates the Event could not be JSON-encoded.
	ErrCodeMarshal DispatchErrorCode = "marshal"
	// ErrCodeSign indicates the payload signature computation failed.
	ErrCodeSign DispatchErrorCode = "sign"
	// ErrCodeRequest indicates the HTTP request could not be sent or
	// completed (network error, DNS failure, TLS error, etc.).
	ErrCodeRequest DispatchErrorCode = "request"
	// ErrCodeReadBody indicates the HTTP response was received but the
	// response body could not be fully read.
	ErrCodeReadBody DispatchErrorCode = "read_body"
	// ErrCodeContextCancelled indicates the context was cancelled during
	// retry backoff, so no further attempts were made.
	ErrCodeContextCancelled DispatchErrorCode = "context_cancelled"
	// ErrCodePayloadTooLarge indicates the serialized event payload exceeds
	// the configured maximum payload size.
	ErrCodePayloadTooLarge DispatchErrorCode = "payload_too_large"
	// ErrCodeEventValidation indicates the event failed field-level validation
	// (e.g. ID, Source, or Subject exceeds maximum length).
	ErrCodeEventValidation DispatchErrorCode = "event_validation"
)

// DispatchResult holds the outcome of a single webhook dispatch attempt.
// Success is true when the HTTP response status code is in the 2xx range.
// When Success is false, ErrorCode classifies the failure category and
// Error contains a human-readable description.
type DispatchResult struct {
	Success       bool
	StatusCode    int
	ResponseBody  string
	DurationMs    int64
	Error         string
	ErrorCode     DispatchErrorCode
	Attempts      int
	rawRetryAfter string
}

// BackoffFunc computes the backoff duration before the next retry attempt.
//
// Parameters:
//   - attempt: zero-based attempt index (0 = first retry, 1 = second retry, ...).
//   - result:  the DispatchResult of the previous attempt.
//
// Return a duration of 0 or less to skip sleeping (proceed immediately).
type BackoffFunc func(attempt int, result *DispatchResult) time.Duration

// ResultHandler is called after every dispatch attempt (successful or not).
// It receives the DispatchResult and the attempt number (1-based).
// Useful for logging, metrics, or custom alerting.
//
// If the handler panics, the panic is recovered and logged; it does not
// affect the dispatch flow.
type ResultHandler func(result *DispatchResult, attempt int)

// DispatcherOption configures a Dispatcher via the functional options pattern.
// Use With* functions to construct options.
type DispatcherOption func(*Dispatcher)

// WithHTTPClient injects a custom http.Client into the Dispatcher.
// Use this to configure proxy, TLS, transport settings, or connection pooling
// at the Dispatcher level.
//
// The SSRF redirect policy is always enforced, even when a custom client is
// provided. This prevents accidental bypass of SSRF protections.
//
// If not provided, a default client with the timeout passed to NewDispatcher
// is used.
func WithHTTPClient(client *http.Client) DispatcherOption {
	return func(d *Dispatcher) {
		if client != nil {
			client.CheckRedirect = d.ssrfPolicy.CheckRedirect()
		}
		d.client = client
	}
}

// WithUserAgent sets a custom User-Agent header for all outgoing requests.
//
// If not provided, defaults to "quotagate-Webhook/1.0".
func WithUserAgent(userAgent string) DispatcherOption {
	return func(d *Dispatcher) {
		d.userAgent = userAgent
	}
}

// WithDefaultHeader adds a default header that is sent with every request.
// These headers are applied before the request-specific headers (signature,
// content-type), so they can be overridden per-request if needed.
//
// Call this multiple times to add multiple headers.
func WithDefaultHeader(key, value string) DispatcherOption {
	return func(d *Dispatcher) {
		if d.defaultHeaders == nil {
			d.defaultHeaders = make(map[string]string)
		}
		d.defaultHeaders[key] = value
	}
}

// WithBackoffFunc sets a custom backoff strategy for retries.
//
// The function receives the zero-based attempt index and the previous
// DispatchResult. Return a duration <= 0 to skip backoff and retry immediately.
//
// If not provided, the default exponential backoff with full jitter
// (capped at 30 seconds) is used.
func WithBackoffFunc(fn BackoffFunc) DispatcherOption {
	return func(d *Dispatcher) {
		d.backoffFunc = fn
	}
}

// WithOnResult sets a callback that is invoked after every dispatch attempt
// (both successful and failed attempts, including retries).
//
// This is useful for logging, metrics collection, or external alerting.
// If the handler panics, the panic is recovered and logged; it will not
// affect the dispatch flow.
//
// Only one handler can be set; calling WithOnResult multiple times
// overwrites the previous handler.
func WithOnResult(handler ResultHandler) DispatcherOption {
	return func(d *Dispatcher) {
		d.onResult = handler
	}
}

// WithMaxRespBodySize sets the maximum response body size (in bytes) read
// from webhook endpoints. Responses exceeding this limit are truncated to
// prevent memory exhaustion.
//
// If not provided or set to <= 0, defaults to 1 MB (defaultMaxRespBodySize).
func WithMaxRespBodySize(size int64) DispatcherOption {
	return func(d *Dispatcher) {
		d.maxRespBodySize = size
	}
}

// WithMaxPayloadSize sets the maximum serialized event payload size in bytes.
// Events whose JSON-serialized payload exceeds this limit are rejected before
// any HTTP request is made.
//
// If not provided or set to <= 0, defaults to 1 MB (defaultMaxPayloadSize).
func WithMaxPayloadSize(size int64) DispatcherOption {
	return func(d *Dispatcher) {
		d.maxPayloadSize = size
	}
}

// WithSecret sets the default signing secret for all dispatches.
//
// When a dispatch is made with an empty secret argument, this default
// secret is used. An empty default secret (the zero value) skips signing.
func WithSecret(secret string) DispatcherOption {
	return func(d *Dispatcher) {
		d.defaultSecret = secret
	}
}

// WithTimeout sets the default HTTP timeout for dispatches.
//
// When a dispatch is made with a zero timeout argument, this default
// timeout is used. If this is also zero, the Dispatcher-level client
// timeout (from NewDispatcher) is used.
func WithTimeout(timeout time.Duration) DispatcherOption {
	return func(d *Dispatcher) {
		d.defaultTimeout = timeout
	}
}

// WithSSRFPolicy sets a custom SSRF policy for the Dispatcher.
//
// Use this to relax or tighten SSRF restrictions (e.g. allow loopback
// addresses in test environments). If not provided, the default policy
// (blocks private, loopback, and link-local IPs) is used.
func WithSSRFPolicy(policy *ssrf.Policy) DispatcherOption {
	return func(d *Dispatcher) {
		d.ssrfPolicy = policy
		if d.client != nil {
			d.client.CheckRedirect = policy.CheckRedirect()
		}
	}
}

// Dispatcher sends Events to external HTTP endpoints (webhooks).
//
// It supports:
//   - HMAC payload signing with configurable hash algorithms (SHA-256, SHA-384,
//     SHA-512, SM3) via the Signer abstraction.
//   - Per-request HTTP timeout overriding the dispatcher-level default.
//   - Retry with configurable backoff strategy via DispatchWithRetry.
//     Only 5xx responses and transport errors are retried; 4xx responses
//     are treated as deterministic failures.
//   - Custom HTTP headers via WithDefaultHeader.
//   - Custom http.Client via WithHTTPClient.
//   - Custom User-Agent via WithUserAgent.
//   - Result callbacks via WithOnResult for logging and metrics.
//   - Custom backoff strategy via WithBackoffFunc.
//
// Create a Dispatcher with NewDispatcher (default SHA-256 signer) or
// NewDispatcherWithSigner (custom hash algorithm, e.g. SM3),
// optionally passing DispatcherOption values.
type Dispatcher struct {
	client          *http.Client
	signer          *Signer
	rand            *rand.Rand
	userAgent       string
	defaultHeaders  map[string]string
	backoffFunc     BackoffFunc
	onResult        ResultHandler
	maxRespBodySize int64
	maxPayloadSize  int64
	ssrfPolicy      *ssrf.Policy
	defaultSecret   string
	defaultTimeout  time.Duration
}

// NewDispatcher creates a Dispatcher with the given HTTP timeout and the
// default HMAC-SHA256 signer.
//
// The Dispatcher maintains a shared http.Client with connection pooling.
// For per-request timeout overrides, use the timeout parameter
// on Dispatch / DispatchWithRetry instead of creating multiple Dispatchers.
//
// Optional DispatcherOption values can be passed to customize the Dispatcher.
func NewDispatcher(timeout time.Duration, opts ...DispatcherOption) *Dispatcher {
	policy := ssrf.DefaultPolicy()
	d := &Dispatcher{
		client: &http.Client{
			Timeout:       timeout,
			CheckRedirect: policy.CheckRedirect(),
		},
		signer:          DefaultSigner,
		rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
		userAgent:       defaultUserAgent,
		maxRespBodySize: defaultMaxRespBodySize,
		maxPayloadSize:  defaultMaxPayloadSize,
		ssrfPolicy:      policy,
	}

	for _, opt := range opts {
		opt(d)
	}

	if d.client == nil {
		d.client = &http.Client{
			Timeout:       timeout,
			CheckRedirect: policy.CheckRedirect(),
		}
	}

	if d.maxRespBodySize <= 0 {
		d.maxRespBodySize = defaultMaxRespBodySize
	}

	if d.maxPayloadSize <= 0 {
		d.maxPayloadSize = defaultMaxPayloadSize
	}

	return d
}

// NewDispatcherWithSigner creates a Dispatcher with the given HTTP timeout
// and a custom Signer (e.g. using SM3 for Chinese national cryptography).
//
// See NewSigner for creating a Signer with a specific hash algorithm.
//
// Optional DispatcherOption values can be passed to customize the Dispatcher.
func NewDispatcherWithSigner(timeout time.Duration, signer *Signer, opts ...DispatcherOption) *Dispatcher {
	policy := ssrf.DefaultPolicy()
	d := &Dispatcher{
		client: &http.Client{
			Timeout:       timeout,
			CheckRedirect: policy.CheckRedirect(),
		},
		signer:          signer,
		rand:            rand.New(rand.NewSource(time.Now().UnixNano())),
		userAgent:       defaultUserAgent,
		maxRespBodySize: defaultMaxRespBodySize,
		maxPayloadSize:  defaultMaxPayloadSize,
		ssrfPolicy:      policy,
	}

	for _, opt := range opts {
		opt(d)
	}

	if d.client == nil {
		d.client = &http.Client{
			Timeout:       timeout,
			CheckRedirect: policy.CheckRedirect(),
		}
	}

	if d.maxRespBodySize <= 0 {
		d.maxRespBodySize = defaultMaxRespBodySize
	}

	if d.maxPayloadSize <= 0 {
		d.maxPayloadSize = defaultMaxPayloadSize
	}

	return d
}

// Signer returns the Signer used by this Dispatcher.
// Useful for verifying signatures or inspecting the hash algorithm.
func (d *Dispatcher) Signer() *Signer {
	return d.signer
}

const (
	// maxBackoff is the upper bound for exponential backoff in DispatchWithRetry.
	// Even when the retry attempt would produce a longer backoff (e.g. attempt 10),
	// the sleep duration is capped at this value to prevent excessively long waits.
	maxBackoff = 30 * time.Second

	// defaultUserAgent is the User-Agent header value sent with every
	// webhook POST request. The version segment can be updated alongside
	// releases to aid server-side analytics.
	defaultUserAgent = "quotagate-Webhook/1.0"

	// defaultMaxRespBodySize is the default maximum response body size (1 MB)
	// read from webhook endpoints. Responses exceeding this limit are truncated
	// to prevent memory exhaustion from malicious or misconfigured servers.
	defaultMaxRespBodySize = 1 << 20

	// defaultMaxPayloadSize is the default maximum serialized event payload
	// size (1 MB). Events exceeding this limit are rejected before any HTTP
	// request is made, preventing memory exhaustion from oversized payloads.
	defaultMaxPayloadSize = 1 << 20

	// maxEventIDLen is the maximum length of an Event.ID field.
	maxEventIDLen = 256
	// maxEventSourceLen is the maximum length of an Event.Source field.
	maxEventSourceLen = 512
	// maxEventSubjectLen is the maximum length of an Event.Subject field.
	maxEventSubjectLen = 512
)

// defaultBackoff is the default BackoffFunc: exponential growth with
// full jitter, capped at maxBackoff (30 seconds).
//
// The base backoff follows: 1s, 2s, 4s, 8s, ... capped at maxBackoff.
// Full jitter randomises the final value uniformly in [0, cappedBackoff],
// which provides the lowest total completion time bound under contention
// (see AWS architecture blog: "Exponential Backoff And Jitter").
func (d *Dispatcher) defaultBackoff(attempt int, _ *DispatchResult) time.Duration {
	base := time.Duration(1<<uint(attempt)) * time.Second
	if base > maxBackoff {
		base = maxBackoff
	}
	jitter := d.rand.Int63n(int64(base))
	return time.Duration(jitter)
}

// Dispatch sends the given Event to the specified URL as a JSON payload
// via HTTP POST.
//
// Parameters:
//   - ctx:     context for cancellation and deadline propagation.
//     If timeout > 0, a derived context with that timeout is used
//     automatically; ctx itself is still respected for cancellation.
//   - event:   the Event to deliver.
//   - url:     the webhook endpoint URL.
//   - secret:  signing secret; if non-empty, the payload is signed
//     using the Dispatcher's Signer and the X-Webhook-Signature header
//     is set. An empty string uses the Dispatcher's default secret
//     (see WithSecret); if the default is also empty, signing is skipped.
//   - timeout: per-request timeout; if > 0 it overrides the Dispatcher-level
//     default timeout (see WithTimeout). A zero value uses the Dispatcher default.
//
// Returns a DispatchResult. When Success is false, the ErrorCode field
// classifies the failure and Error provides a human-readable description.
// The StatusCode field is always populated from the HTTP response,
// even when the response body cannot be read.
func (d *Dispatcher) Dispatch(ctx context.Context, event Event, url string, secret string, timeout time.Duration) *DispatchResult {
	start := time.Now()

	if err := d.ssrfPolicy.ValidateURL(url); err != nil {
		return &DispatchResult{
			Success:    false,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("quotagate/event: %v", err),
			ErrorCode:  ErrCodeRequest,
		}
	}

	if len(event.ID) > maxEventIDLen {
		return &DispatchResult{
			Success:    false,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("quotagate/event: event ID exceeds max length (%d)", maxEventIDLen),
			ErrorCode:  ErrCodeEventValidation,
		}
	}
	if len(event.Source) > maxEventSourceLen {
		return &DispatchResult{
			Success:    false,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("quotagate/event: event Source exceeds max length (%d)", maxEventSourceLen),
			ErrorCode:  ErrCodeEventValidation,
		}
	}
	if len(event.Subject) > maxEventSubjectLen {
		return &DispatchResult{
			Success:    false,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("quotagate/event: event Subject exceeds max length (%d)", maxEventSubjectLen),
			ErrorCode:  ErrCodeEventValidation,
		}
	}

	payload, err := json.Marshal(event)
	if err != nil {
		slog.Error("[quotagate/event] marshal failed", "error", err)
		return &DispatchResult{
			Success:    false,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("quotagate/event: failed to marshal event: %v", err),
			ErrorCode:  ErrCodeMarshal,
		}
	}

	if int64(len(payload)) > d.maxPayloadSize {
		return &DispatchResult{
			Success:    false,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("quotagate/event: payload size %d exceeds max %d", len(payload), d.maxPayloadSize),
			ErrorCode:  ErrCodePayloadTooLarge,
		}
	}

	// Resolve effective secret and timeout.
	if secret == "" {
		secret = d.defaultSecret
	}
	if timeout <= 0 {
		timeout = d.defaultTimeout
	}

	parentCtx := ctx
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payload))
	if err != nil {
		return &DispatchResult{
			Success:    false,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("quotagate/event: failed to create request: %v", err),
			ErrorCode:  ErrCodeRequest,
		}
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", d.userAgent)

	for k, v := range d.defaultHeaders {
		if req.Header.Get(k) == "" {
			req.Header.Set(k, v)
		}
	}

	req.Header.Set("X-Idempotency-Key", event.ID)

	if secret != "" {
		signResult, err := d.signer.SignPayload(payload, secret, time.Now())
		if err != nil {
			slog.Error("sign failed", "package", "event", "error", err)
			return &DispatchResult{
				Success:    false,
				DurationMs: time.Since(start).Milliseconds(),
				Error:      fmt.Sprintf("quotagate/event: failed to sign payload: %v", err),
				ErrorCode:  ErrCodeSign,
			}
		}
		req.Header.Set(SignatureHeader, signResult.Header)
	}

	resp, err := d.client.Do(req)
	if err != nil {
		errorCode := ErrCodeRequest
		if parentCtx.Err() != nil {
			errorCode = ErrCodeContextCancelled
		}
		return &DispatchResult{
			Success:    false,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("quotagate/event: request failed: %v", err),
			ErrorCode:  errorCode,
		}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, d.maxRespBodySize))
	if err != nil {
		return &DispatchResult{
			Success:    false,
			StatusCode: resp.StatusCode,
			DurationMs: time.Since(start).Milliseconds(),
			Error:      fmt.Sprintf("quotagate/event: failed to read response body: %v", err),
			ErrorCode:  ErrCodeReadBody,
		}
	}

	duration := time.Since(start).Milliseconds()

	success := resp.StatusCode >= 200 && resp.StatusCode < 300

	result := &DispatchResult{
		Success:       success,
		StatusCode:    resp.StatusCode,
		ResponseBody:  string(body),
		DurationMs:    duration,
		ErrorCode:     ErrCodeNone,
		rawRetryAfter: resp.Header.Get("Retry-After"),
	}
	return result
}

// invokeOnResult calls the onResult handler if set, recovering from panics.
func (d *Dispatcher) invokeOnResult(result *DispatchResult, attempt int) {
	if d.onResult == nil {
		return
	}
	func() {
		defer func() {
			if r := recover(); r != nil {
				slog.Error("[quotagate/event] onResult handler panicked", "error", r)
			}
		}()
		d.onResult(result, attempt)
	}()
}

// isRetryable determines whether a failed dispatch result should be retried.
//
// Retry policy:
//   - 429 (Too Many Requests) is treated as a transient rate-limit error and
//     IS retried, respecting the Retry-After header when present.
//   - Other 4xx responses (400-428, 430-499) are deterministic client errors
//     and are NOT retried.
//   - 5xx responses are transient server errors and ARE retried.
//   - Transport errors (StatusCode == 0, e.g. DNS failure, connection refused,
//     TLS error) ARE retried.
func isRetryable(result *DispatchResult) bool {
	if result.Success {
		return false
	}
	if result.StatusCode == http.StatusTooManyRequests {
		return true
	}
	if result.StatusCode >= 400 && result.StatusCode < 500 {
		return false
	}
	return true
}

// parseRetryAfter parses the Retry-After response header.
// It accepts both a delay in seconds (e.g. "120") and an HTTP-date
// (e.g. "Fri, 16 May 2025 12:00:00 GMT"). Returns 0 on parse failure.
func parseRetryAfter(header string, now time.Time) time.Duration {
	if header == "" {
		return 0
	}
	if seconds, err := strconv.Atoi(header); err == nil {
		if seconds > 0 {
			return time.Duration(seconds) * time.Second
		}
		return 0
	}
	if t, err := http.ParseTime(header); err == nil {
		d := t.Sub(now)
		if d > 0 {
			return d
		}
	}
	return 0
}

// DispatchWithRetry sends an Event with retry support.
//
// Retry policy:
//   - 429 (Too Many Requests) is retried, respecting the Retry-After header.
//   - Other 4xx responses are treated as deterministic client errors and
//     returned immediately without further attempts.
//   - 5xx responses and transport errors are retried.
//   - Backoff uses the custom BackoffFunc if set via WithBackoffFunc,
//     otherwise falls back to exponential backoff with full jitter,
//     capped at maxBackoff (30 seconds).
//   - The backoff wait respects context cancellation: if ctx is done
//     during a backoff, the method returns immediately with
//     ErrCodeContextCancelled.
//
// Retry semantics:
//   - maxRetries = N means at most N retry attempts after the initial attempt,
//     for a total of N+1 requests.
//   - maxRetries = 0 means no retries (single attempt only).
//
// The result of the last attempt (successful or not) is always returned.
// The Attempts field on the result indicates how many HTTP requests were made.
func (d *Dispatcher) DispatchWithRetry(ctx context.Context, event Event, url string, secret string, timeout time.Duration, maxRetries int) *DispatchResult {
	if maxRetries < 0 {
		maxRetries = 0
	}

	var lastResult *DispatchResult
	totalAttempts := 0

	for attempt := 0; attempt <= maxRetries; attempt++ {
		select {
		case <-ctx.Done():
			return &DispatchResult{
				Success:    false,
				DurationMs: 0,
				Error:      fmt.Sprintf("quotagate/event: context cancelled before attempt %d: %v", attempt+1, ctx.Err()),
				ErrorCode:  ErrCodeContextCancelled,
				Attempts:   totalAttempts,
			}
		default:
		}

		lastResult = d.Dispatch(ctx, event, url, secret, timeout)
		totalAttempts++

		lastResult.Attempts = totalAttempts
		d.invokeOnResult(lastResult, attempt+1)

		if lastResult.Success {
			return lastResult
		}

		if lastResult.ErrorCode == ErrCodeContextCancelled && ctx.Err() != nil {
			return lastResult
		}

		if !isRetryable(lastResult) {
			slog.Info("[quotagate/event] non-retryable error (status %d), not retrying", lastResult.StatusCode)
			return lastResult
		}

		if attempt < maxRetries {
			backoff := d.computeBackoff(attempt, lastResult)

			if lastResult.StatusCode == http.StatusTooManyRequests {
				if ra := parseRetryAfter(lastResult.rawRetryAfter, time.Now()); ra > 0 {
					backoff = ra
					slog.Info("[quotagate/event] attempt %d got 429, using Retry-After %v", attempt+1, "retryAfter", backoff)
				}
			}

			slog.Info("[quotagate/event] attempt %d failed (status %d), retrying in %v", attempt+1, lastResult.StatusCode, backoff)

			select {
			case <-ctx.Done():
				return &DispatchResult{
					Success:    false,
					DurationMs: 0,
					Error:      fmt.Sprintf("quotagate/event: context cancelled during retry backoff: %v", ctx.Err()),
					ErrorCode:  ErrCodeContextCancelled,
					Attempts:   totalAttempts,
				}
			case <-time.After(backoff):
			}
		}
	}

	slog.Info("[quotagate/event] all %d attempts exhausted, last status %d", totalAttempts, lastResult.StatusCode)
	return lastResult
}

// computeBackoff determines the backoff duration for the given attempt.
// If a custom BackoffFunc was set via WithBackoffFunc, it is used;
// otherwise the default exponential backoff with full jitter is applied.
func (d *Dispatcher) computeBackoff(attempt int, result *DispatchResult) time.Duration {
	if d.backoffFunc != nil {
		return d.backoffFunc(attempt, result)
	}
	return d.defaultBackoff(attempt, result)
}
