package worker

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"sync/atomic"
	"time"

	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/repository"
	kexrandom "github.com/roidmc/quotagate/internal/util/random"
)

const (
	defaultWorkers           = 10
	defaultMinInterval       = 500 * time.Millisecond
	defaultMaxInterval       = 10 * time.Second
	defaultClaimLimit        = 1
	defaultDLQCheckInterval  = 5 * time.Minute
	defaultDLQAlertThreshold = 100
)

type WebhookWorkerMetrics struct {
	SuccessCount    int64
	FailureCount    int64
	DeadLetterCount int64
	ProcessedCount  int64
}

// WebhookWorker dispatches outbox entries via a pool of goroutines.
//
// Known trade-offs (intentionally not implemented — ROI too low for this scope):
//
//   - No per-endpoint fair scheduling: workers claim by created_at (FIFO), so a
//     high-volume endpoint can delay others. Fair scheduling needs a per-webhook
//     dispatch queue, which adds complexity disproportionate to the benefit.
//   - No shutdown timeout: Stop() waits for all in-flight dispatches to finish.
//     A stuck endpoint blocks Stop until its HTTP timeout fires. Accept the
//     wait or call cancel() directly for a hard stop.
//   - No backoff jitter across workers: all workers start in sync and could
//     poll in lockstep when idle. Dispatch duration variance desynchronises
//     them within a few cycles, so explicit jitter wasn't added.
//   - No multi-instance lease coordination beyond FOR UPDATE SKIP LOCKED:
//     multiple process instances rely on the database for claim safety, but
//     there is no distributed lease renewal. A crashed instance's entries are
//     recovered after processingLeaseTimeout (5 min), not immediately.
type WebhookWorker struct {
	repo       *repository.WebhookRepository
	dispatcher *event.Dispatcher

	workers           int
	minInterval       time.Duration
	maxInterval       time.Duration
	dlqCheckInterval  time.Duration
	dlqAlertThreshold int

	successCount    int64
	failureCount    int64
	deadLetterCount int64
	processedCount  int64

	ctx     context.Context
	cancel  context.CancelFunc
	stopCh  chan struct{}
	once    sync.Once
	wg      sync.WaitGroup
	started atomic.Bool
}

type WebhookWorkerOption func(*WebhookWorker)

func WithWorkers(n int) WebhookWorkerOption {
	return func(w *WebhookWorker) {
		if n > 0 {
			w.workers = n
		}
	}
}

func WithMinInterval(d time.Duration) WebhookWorkerOption {
	return func(w *WebhookWorker) { w.minInterval = d }
}

func WithMaxInterval(d time.Duration) WebhookWorkerOption {
	return func(w *WebhookWorker) { w.maxInterval = d }
}

func WithDLQCheckInterval(d time.Duration) WebhookWorkerOption {
	return func(w *WebhookWorker) { w.dlqCheckInterval = d }
}

func WithDLQAlertThreshold(n int) WebhookWorkerOption {
	return func(w *WebhookWorker) { w.dlqAlertThreshold = n }
}

func NewWebhookWorker(repo *repository.WebhookRepository, dispatcher *event.Dispatcher, opts ...WebhookWorkerOption) *WebhookWorker {
	ctx, cancel := context.WithCancel(context.Background())
	w := &WebhookWorker{
		repo:              repo,
		dispatcher:        dispatcher,
		workers:           defaultWorkers,
		minInterval:       defaultMinInterval,
		maxInterval:       defaultMaxInterval,
		dlqCheckInterval:  defaultDLQCheckInterval,
		dlqAlertThreshold: defaultDLQAlertThreshold,
		ctx:               ctx,
		cancel:            cancel,
		stopCh:            make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

// Start launches the configured number of worker goroutines. Each goroutine
// claims one pending outbox entry at a time and dispatches it independently,
// so a slow endpoint cannot block the whole worker pool.
//
// Start must be called at most once. Calling Start twice panics — this
// indicates a caller bug (e.g. starting the same worker in multiple places).
// To restart after Stop, create a new WebhookWorker.
func (w *WebhookWorker) Start() {
	if !w.started.CompareAndSwap(false, true) {
		panic("quotagate/worker: Start called more than once")
	}
	for i := 0; i < w.workers; i++ {
		w.wg.Add(1)
		go w.runWorker(i)
	}
}

// Stop performs a graceful shutdown: it signals all workers to stop claiming
// new entries, waits for in-flight dispatches and persistence operations to
// complete, and only then cancels the internal context.
func (w *WebhookWorker) Stop() {
	w.once.Do(func() {
		close(w.stopCh)
	})
	w.wg.Wait()
	w.cancel()
}

func (w *WebhookWorker) Metrics() WebhookWorkerMetrics {
	return WebhookWorkerMetrics{
		SuccessCount:    atomic.LoadInt64(&w.successCount),
		FailureCount:    atomic.LoadInt64(&w.failureCount),
		DeadLetterCount: atomic.LoadInt64(&w.deadLetterCount),
		ProcessedCount:  atomic.LoadInt64(&w.processedCount),
	}
}

// ReplayDeadEntry resets a single dead-letter outbox entry to pending so the
// worker will redeliver it. Use this for manual DLQ replay from an admin API.
func (w *WebhookWorker) ReplayDeadEntry(ctx context.Context, id string) error {
	return w.repo.ReplayDeadOutbox(ctx, id)
}

// ReplayDeadBatch resets up to limit dead-letter entries to pending and returns
// the number of entries replayed. A limit <= 0 means no limit.
func (w *WebhookWorker) ReplayDeadBatch(ctx context.Context, limit int) (int64, error) {
	if limit <= 0 {
		limit = 1000
	}

	entries, err := w.repo.ListDeadOutbox(ctx, limit, 0)
	if err != nil {
		return 0, err
	}

	var replayed int64
	for _, entry := range entries {
		if err := w.repo.ReplayDeadOutbox(ctx, entry.ID); err != nil {
			slog.Error("quotagate/worker: failed to replay dead entry", "entry_id", entry.ID, "error", err)
			continue
		}
		replayed++
	}

	return replayed, nil
}

func (w *WebhookWorker) runWorker(id int) {
	defer w.wg.Done()

	currentInterval := w.minInterval
	ticker := time.NewTicker(currentInterval)
	defer ticker.Stop()

	var lastDLQCheck time.Time

	for {
		select {
		case <-w.ctx.Done():
			return
		case <-w.stopCh:
			return
		case <-ticker.C:
			foundWork := w.claimAndProcess()
			if foundWork {
				currentInterval = w.minInterval
			} else {
				currentInterval *= 2
				if currentInterval > w.maxInterval {
					currentInterval = w.maxInterval
				}
				if time.Since(lastDLQCheck) >= w.dlqCheckInterval {
					w.checkDLQ(w.ctx)
					lastDLQCheck = time.Now()
				}
			}
			ticker.Reset(currentInterval)
		}
	}
}

// claimAndProcess pulls a single pending outbox entry and processes it.
// It returns true when an entry was found and claimed.
func (w *WebhookWorker) claimAndProcess() bool {
	entries, err := w.repo.ClaimPendingOutbox(w.ctx, defaultClaimLimit)
	if err != nil {
		slog.Error("quotagate/worker: claim pending outbox failed", "error", err)
		return false
	}
	if len(entries) == 0 {
		return false
	}

	w.processEntry(w.ctx, entries[0])
	return true
}

func (w *WebhookWorker) checkDLQ(ctx context.Context) {
	count, err := w.repo.CountDeadOutbox(ctx)
	if err != nil {
		slog.Error("quotagate/worker: count dead outbox failed", "error", err)
		return
	}

	atomic.StoreInt64(&w.deadLetterCount, count)

	if count > int64(w.dlqAlertThreshold) {
		slog.Warn("quotagate/worker: dead letters in outbox exceed threshold", "count", count, "threshold", w.dlqAlertThreshold)
	}
}

func (w *WebhookWorker) processEntry(ctx context.Context, entry model.WebhookOutbox) {
	atomic.AddInt64(&w.processedCount, 1)

	var evt event.Event
	if err := json.Unmarshal([]byte(entry.Payload), &evt); err != nil {
		slog.Error("quotagate/worker: unmarshal outbox payload failed", "entry_id", entry.ID, "error", err)
		if markErr := w.repo.MarkOutboxDead(ctx, entry.ID, "invalid payload: "+err.Error()); markErr != nil {
			slog.Error("quotagate/worker: mark outbox dead failed", "entry_id", entry.ID, "error", markErr)
		}
		atomic.AddInt64(&w.failureCount, 1)
		return
	}

	result := w.dispatcher.DispatchWithRetry(
		ctx, evt, entry.URL, entry.Secret,
		time.Duration(entry.TimeoutSeconds)*time.Second, entry.MaxAttempts-entry.Attempt,
	)

	deliveryLog := &model.WebhookDeliveryLog{
		ID:              kexrandom.MustUUIDString(),
		WebhookConfigID: entry.WebhookID,
		EventID:         entry.EventID,
		EventType:       entry.EventType,
		RequestURL:      entry.URL,
		RequestBody:     entry.Payload,
		ResponseStatus:  result.StatusCode,
		ResponseBody:    result.ResponseBody,
		DurationMs:      result.DurationMs,
		Success:         result.Success,
		Attempt:         entry.Attempt + result.Attempts,
		Error:           result.Error,
	}

	if logErr := w.repo.CreateDeliveryLog(ctx, deliveryLog); logErr != nil {
		slog.Error("quotagate/worker: create delivery log failed", "entry_id", entry.ID, "error", logErr)
	}

	if result.Success {
		if err := w.repo.MarkOutboxCompleted(ctx, entry.ID); err != nil {
			slog.Error("quotagate/worker: mark outbox completed failed", "entry_id", entry.ID, "error", err)
		}
		atomic.AddInt64(&w.successCount, 1)
		return
	}

	if err := w.repo.MarkOutboxFailed(ctx, entry.ID, result.Error); err != nil {
		slog.Error("quotagate/worker: mark outbox failed failed", "entry_id", entry.ID, "error", err)
	}
	atomic.AddInt64(&w.failureCount, 1)
}
