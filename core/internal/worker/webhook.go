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
	defaultMinInterval       = 500 * time.Millisecond
	defaultMaxInterval       = 10 * time.Second
	defaultBatchSize         = 50
	defaultDLQCheckInterval  = 5 * time.Minute
	defaultDLQAlertThreshold = 100
)

type WebhookWorkerMetrics struct {
	SuccessCount    int64
	FailureCount    int64
	DeadLetterCount int64
	ProcessedCount  int64
}

type WebhookWorker struct {
	repo       *repository.WebhookRepository
	dispatcher *event.Dispatcher

	minInterval       time.Duration
	maxInterval       time.Duration
	currentInterval   time.Duration
	batchSize         int
	dlqCheckInterval  time.Duration
	dlqAlertThreshold int
	lastDLQCheck      time.Time

	successCount    int64
	failureCount    int64
	deadLetterCount int64
	processedCount  int64

	ctx    context.Context
	cancel context.CancelFunc
	stopCh chan struct{}
	once   sync.Once
	wg     sync.WaitGroup
}

type WebhookWorkerOption func(*WebhookWorker)

func WithMinInterval(d time.Duration) WebhookWorkerOption {
	return func(w *WebhookWorker) { w.minInterval = d }
}

func WithMaxInterval(d time.Duration) WebhookWorkerOption {
	return func(w *WebhookWorker) { w.maxInterval = d }
}

func WithBatchSize(n int) WebhookWorkerOption {
	return func(w *WebhookWorker) { w.batchSize = n }
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
		minInterval:       defaultMinInterval,
		maxInterval:       defaultMaxInterval,
		currentInterval:   defaultMinInterval,
		batchSize:         defaultBatchSize,
		dlqCheckInterval:  defaultDLQCheckInterval,
		dlqAlertThreshold: defaultDLQAlertThreshold,
		lastDLQCheck:      time.Now(),
		ctx:               ctx,
		cancel:            cancel,
		stopCh:            make(chan struct{}),
	}

	for _, opt := range opts {
		opt(w)
	}

	return w
}

func (w *WebhookWorker) Start() {
	w.wg.Add(1)
	go func() {
		defer w.wg.Done()
		ticker := time.NewTicker(w.currentInterval)
		defer ticker.Stop()

		for {
			select {
			case <-w.ctx.Done():
				return
			case <-w.stopCh:
				return
			case <-ticker.C:
				foundWork := w.processBatch(w.ctx)
				if foundWork {
					w.currentInterval = w.minInterval
				} else {
					w.currentInterval *= 2
					if w.currentInterval > w.maxInterval {
						w.currentInterval = w.maxInterval
					}
				}
				ticker.Reset(w.currentInterval)
			}
		}
	}()
}

func (w *WebhookWorker) Stop() {
	w.once.Do(func() {
		w.cancel()
		close(w.stopCh)
	})
	w.wg.Wait()
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

func (w *WebhookWorker) processBatch(ctx context.Context) bool {
	entries, err := w.repo.ClaimPendingOutbox(ctx, w.batchSize)
	if err != nil {
		slog.Error("quotagate/worker: claim pending outbox failed", "error", err)
		return false
	}

	for _, entry := range entries {
		select {
		case <-ctx.Done():
			return len(entries) > 0
		default:
			w.processEntry(ctx, entry)
		}
	}

	w.checkDLQ(ctx)

	return len(entries) > 0
}

func (w *WebhookWorker) checkDLQ(ctx context.Context) {
	if time.Since(w.lastDLQCheck) < w.dlqCheckInterval {
		return
	}
	w.lastDLQCheck = time.Now()

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
