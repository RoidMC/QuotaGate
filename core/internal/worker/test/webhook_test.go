package worker_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/roidmc/kex-utils/pkg/kexrandom"
	"github.com/roidmc/kex-utils/pkg/kexssrf"
	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/repository"
	"github.com/roidmc/quotagate/internal/testutil/testdb"
	"github.com/roidmc/quotagate/internal/worker"
	"gorm.io/gorm"
)

func setupWorkerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db := testdb.OpenRaw(t)
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("failed to get sql db: %v", err)
	}
	sqlDB.SetMaxOpenConns(1)

	repo := repository.NewWebhookRepository(db)
	if err := repo.AutoMigrate(); err != nil {
		t.Fatalf("failed to migrate: %v", err)
	}
	return db
}

func testSSRFPolicy() *kexssrf.Policy {
	p := kexssrf.DefaultPolicy()
	p.AllowLoopback = true
	return p
}

func newTestDispatcher(t *testing.T) *event.Dispatcher {
	t.Helper()
	return event.NewDispatcher(5*time.Second, event.WithSSRFPolicy(testSSRFPolicy()))
}

// waitForOutboxStatus is the canonical "did processing finish" signal for these
// tests. Wait on the durable DB outbox status, never on w.Metrics():
//
//   - The worker increments ProcessedCount at the *start* of processEntry, so a
//     metrics-based waiter would return the moment an entry is claimed — before
//     the HTTP dispatch even happens. It cannot tell Completed from Dead either.
//   - SuccessCount/FailureCount are incremented *after* the DB terminal status is
//     persisted (see webhook.go processEntry). Therefore any assertion on
//     w.Metrics() MUST follow w.Stop(), which drains in-flight processEntry calls
//     and guarantees the counters are settled. Never read Metrics() between this
//     waiter and Stop().
func waitForOutboxStatus(t *testing.T, db *gorm.DB, id string, want string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var outbox model.WebhookOutbox
		if err := db.Where("id = ?", id).First(&outbox).Error; err != nil {
			time.Sleep(10 * time.Millisecond)
			continue
		}
		if outbox.Status == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for outbox %s status %s", id, want)
}

func waitForNoPending(t *testing.T, db *gorm.DB, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		var count int64
		db.Model(&model.WebhookOutbox{}).Where("status IN ?", []string{model.OutboxStatusPending, model.OutboxStatusProcessing}).Count(&count)
		if count == 0 {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("timeout waiting for pending/processing outbox entries to be processed")
}

func TestWebhookWorkerDispatchesPendingEntry(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(10*time.Millisecond),
		worker.WithMaxInterval(100*time.Millisecond),
	)

	w.Start()

	evt := event.Event{ID: "evt-1", Type: "test", Data: map[string]string{"key": "val"}}
	payload, _ := json.Marshal(evt)

	entry := model.WebhookOutbox{
		ID:             kexrandom.MustUUIDString(),
		EventType:      "test",
		EventID:        "evt-1",
		TenantID:       "tenant-1",
		WebhookID:      "wh-1",
		URL:            server.URL,
		Secret:         "",
		Payload:        string(payload),
		Status:         model.OutboxStatusPending,
		Attempt:        0,
		MaxAttempts:    3,
		TimeoutSeconds: 5,
		NextAttemptAt:  time.Now(),
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	waitForOutboxStatus(t, db, entry.ID, model.OutboxStatusCompleted, 3*time.Second)
	w.Stop()

	if n := atomic.LoadInt32(&callCount); n != 1 {
		t.Errorf("expected 1 HTTP call, got %d", n)
	}

	metrics := w.Metrics()
	if metrics.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", metrics.SuccessCount)
	}
}

func TestWebhookWorkerMarksEntryDeadAfterMaxAttempts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo,
		event.NewDispatcher(100*time.Millisecond, event.WithSSRFPolicy(testSSRFPolicy())),
		worker.WithMinInterval(10*time.Millisecond),
		worker.WithMaxInterval(100*time.Millisecond),
	)

	w.Start()

	evt := event.Event{ID: "evt-fail", Type: "test"}
	payload, _ := json.Marshal(evt)

	entry := model.WebhookOutbox{
		ID:             kexrandom.MustUUIDString(),
		EventType:      "test",
		EventID:        "evt-fail",
		TenantID:       "tenant-1",
		WebhookID:      "wh-1",
		URL:            server.URL,
		Secret:         "",
		Payload:        string(payload),
		Status:         model.OutboxStatusPending,
		Attempt:        0,
		MaxAttempts:    1,
		TimeoutSeconds: 5,
		NextAttemptAt:  time.Now(),
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	waitForOutboxStatus(t, db, entry.ID, model.OutboxStatusDead, 3*time.Second)
	w.Stop()

	metrics := w.Metrics()
	if metrics.FailureCount != 1 {
		t.Errorf("expected 1 failure, got %d", metrics.FailureCount)
	}
}

func TestWebhookWorkerRetriesUntilSuccess(t *testing.T) {
	var attemptCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&attemptCount, 1)
		if n < 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(10*time.Millisecond),
		worker.WithMaxInterval(100*time.Millisecond),
	)

	w.Start()

	evt := event.Event{ID: "evt-retry", Type: "test"}
	payload, _ := json.Marshal(evt)

	entry := model.WebhookOutbox{
		ID:             kexrandom.MustUUIDString(),
		EventType:      "test",
		EventID:        "evt-retry",
		TenantID:       "tenant-1",
		WebhookID:      "wh-1",
		URL:            server.URL,
		Secret:         "",
		Payload:        string(payload),
		Status:         model.OutboxStatusPending,
		Attempt:        0,
		MaxAttempts:    3,
		TimeoutSeconds: 5,
		NextAttemptAt:  time.Now(),
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	waitForOutboxStatus(t, db, entry.ID, model.OutboxStatusCompleted, 5*time.Second)
	w.Stop()

	metrics := w.Metrics()
	if metrics.SuccessCount != 1 {
		t.Errorf("expected 1 success, got %d", metrics.SuccessCount)
	}
}

func TestWebhookWorkerDispatchesMultipleEntries(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(10*time.Millisecond),
		worker.WithMaxInterval(100*time.Millisecond),
	)

	w.Start()

	for i := 0; i < 3; i++ {
		evt := event.Event{ID: fmt.Sprintf("evt-batch-%d", i), Type: "test"}
		payload, _ := json.Marshal(evt)
		entry := model.WebhookOutbox{
			ID:             kexrandom.MustUUIDString(),
			EventType:      "test",
			EventID:        evt.ID,
			TenantID:       "tenant-1",
			WebhookID:      "wh-1",
			URL:            server.URL,
			Secret:         "",
			Payload:        string(payload),
			Status:         model.OutboxStatusPending,
			Attempt:        0,
			MaxAttempts:    3,
			TimeoutSeconds: 5,
			NextAttemptAt:  time.Now(),
		}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatalf("failed to create entry: %d: %v", i, err)
		}
	}

	waitForNoPending(t, db, 3*time.Second)
	w.Stop()

	var completedCount int64
	db.Model(&model.WebhookOutbox{}).Where("status = ?", model.OutboxStatusCompleted).Count(&completedCount)
	if completedCount != 3 {
		t.Errorf("expected 3 completed entries, got %d", completedCount)
	}

	metrics := w.Metrics()
	if metrics.SuccessCount != 3 {
		t.Errorf("expected 3 successes, got %d", metrics.SuccessCount)
	}
}

func TestWebhookWorkerCreatesDeliveryLog(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(10*time.Millisecond),
		worker.WithMaxInterval(100*time.Millisecond),
	)

	w.Start()

	evt := event.Event{ID: "evt-log", Type: "test"}
	payload, _ := json.Marshal(evt)

	entry := model.WebhookOutbox{
		ID:             kexrandom.MustUUIDString(),
		EventType:      "test",
		EventID:        "evt-log",
		TenantID:       "tenant-1",
		WebhookID:      "wh-1",
		URL:            server.URL,
		Secret:         "",
		Payload:        string(payload),
		Status:         model.OutboxStatusPending,
		Attempt:        0,
		MaxAttempts:    3,
		TimeoutSeconds: 5,
		NextAttemptAt:  time.Now(),
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	waitForOutboxStatus(t, db, entry.ID, model.OutboxStatusCompleted, 3*time.Second)
	w.Stop()

	var logCount int64
	db.Model(&model.WebhookDeliveryLog{}).Count(&logCount)
	if logCount != 1 {
		t.Errorf("expected 1 delivery log, got %d", logCount)
	}
}

func TestWebhookWorkerSignsPayloadWithSecret(t *testing.T) {
	var receivedSig string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedSig = r.Header.Get(event.SignatureHeader)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(10*time.Millisecond),
		worker.WithMaxInterval(100*time.Millisecond),
	)

	w.Start()

	evt := event.Event{ID: "evt-secret", Type: "test"}
	payload, _ := json.Marshal(evt)

	entry := model.WebhookOutbox{
		ID:             kexrandom.MustUUIDString(),
		EventType:      "test",
		EventID:        "evt-secret",
		TenantID:       "tenant-1",
		WebhookID:      "wh-1",
		URL:            server.URL,
		Secret:         "my-secret-key",
		Payload:        string(payload),
		Status:         model.OutboxStatusPending,
		Attempt:        0,
		MaxAttempts:    3,
		TimeoutSeconds: 5,
		NextAttemptAt:  time.Now(),
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	waitForOutboxStatus(t, db, entry.ID, model.OutboxStatusCompleted, 3*time.Second)
	w.Stop()

	if receivedSig == "" {
		t.Fatal("expected signature header when secret is set")
	}
	if err := event.VerifySignature(payload, "my-secret-key", receivedSig, event.DefaultTolerance); err != nil {
		t.Errorf("signature verification failed: %v", err)
	}
}

func TestWebhookWorkerInvalidPayloadMarkedDead(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(10*time.Millisecond),
		worker.WithMaxInterval(100*time.Millisecond),
	)

	w.Start()

	entry := model.WebhookOutbox{
		ID:             kexrandom.MustUUIDString(),
		EventType:      "test",
		EventID:        "evt-bad",
		TenantID:       "tenant-1",
		WebhookID:      "wh-1",
		URL:            "https://example.com/webhook",
		Secret:         "",
		Payload:        "invalid json{{{",
		Status:         model.OutboxStatusPending,
		Attempt:        0,
		MaxAttempts:    3,
		TimeoutSeconds: 5,
		NextAttemptAt:  time.Now(),
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("failed to create entry: %v", err)
	}

	waitForOutboxStatus(t, db, entry.ID, model.OutboxStatusDead, 3*time.Second)
	w.Stop()
}

func TestWebhookWorkerStartStop(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(100*time.Millisecond),
		worker.WithMaxInterval(1*time.Second),
	)

	w.Start()
	w.Stop()
}

func TestWebhookWorkerAdaptivePolling(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(50*time.Millisecond),
		worker.WithMaxInterval(500*time.Millisecond),
	)

	w.Start()
	time.Sleep(200 * time.Millisecond)

	metrics := w.Metrics()
	t.Logf("metrics after idle: processed=%d success=%d failure=%d dead_letter=%d",
		metrics.ProcessedCount, metrics.SuccessCount, metrics.FailureCount, metrics.DeadLetterCount)

	w.Stop()
}

func TestWebhookWorkerConcurrentStartStop(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)

	for i := 0; i < 5; i++ {
		w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
			worker.WithMinInterval(50*time.Millisecond),
			worker.WithMaxInterval(100*time.Millisecond),
		)
		w.Start()
		time.Sleep(10 * time.Millisecond)
		w.Stop()
	}
}

func TestWebhookWorkerStopIdempotent(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t))

	w.Start()
	w.Stop()
	w.Stop()
}

func TestWebhookWorkerEmptyOutbox(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(50*time.Millisecond),
		worker.WithMaxInterval(100*time.Millisecond),
	)

	w.Start()
	time.Sleep(200 * time.Millisecond)
	w.Stop()

	metrics := w.Metrics()
	if metrics.ProcessedCount != 0 {
		t.Errorf("expected 0 processed entries for empty outbox, got %d", metrics.ProcessedCount)
	}
}

func TestWebhookWorkerMetricsZero(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t))

	metrics := w.Metrics()
	if metrics.SuccessCount != 0 {
		t.Errorf("expected 0 success, got %d", metrics.SuccessCount)
	}
	if metrics.FailureCount != 0 {
		t.Errorf("expected 0 failure, got %d", metrics.FailureCount)
	}
	if metrics.DeadLetterCount != 0 {
		t.Errorf("expected 0 dead letter, got %d", metrics.DeadLetterCount)
	}
	if metrics.ProcessedCount != 0 {
		t.Errorf("expected 0 processed, got %d", metrics.ProcessedCount)
	}
}

func TestWebhookWorkerReclaimsStaleProcessing(t *testing.T) {
	var callCount int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&callCount, 1)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(10*time.Millisecond),
		worker.WithMaxInterval(100*time.Millisecond),
	)

	evt := event.Event{ID: "evt-stale", Type: "test"}
	payload, _ := json.Marshal(evt)

	entry := model.WebhookOutbox{
		ID:             kexrandom.MustUUIDString(),
		EventType:      "test",
		EventID:        "evt-stale",
		TenantID:       "tenant-1",
		WebhookID:      "wh-1",
		URL:            server.URL,
		Secret:         "",
		Payload:        string(payload),
		Status:         model.OutboxStatusProcessing,
		Attempt:        1,
		MaxAttempts:    3,
		TimeoutSeconds: 5,
		NextAttemptAt:  time.Now(),
		UpdatedAt:      time.Now().Add(-10 * time.Minute),
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("failed to create stale processing entry: %v", err)
	}

	w.Start()
	waitForOutboxStatus(t, db, entry.ID, model.OutboxStatusCompleted, 3*time.Second)
	w.Stop()

	if n := atomic.LoadInt32(&callCount); n != 1 {
		t.Errorf("expected 1 HTTP call after reclaim, got %d", n)
	}
}

func TestWebhookWorkerStartPanicsOnDoubleStart(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t))

	w.Start()
	defer w.Stop()

	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic on double Start")
		}
	}()
	w.Start()
}

func TestWebhookWorkerOptions(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)
	w := worker.NewWebhookWorker(repo, newTestDispatcher(t),
		worker.WithMinInterval(1*time.Second),
		worker.WithMaxInterval(30*time.Second),
		worker.WithWorkers(2),
		worker.WithDLQCheckInterval(1*time.Minute),
		worker.WithDLQAlertThreshold(500),
	)

	w.Start()
	w.Stop()
}

func TestReplayDeadEntry(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)

	entry := model.WebhookOutbox{
		ID:             kexrandom.MustUUIDString(),
		EventType:      "test",
		EventID:        "evt-replay",
		TenantID:       "tenant-1",
		WebhookID:      "wh-1",
		URL:            "https://example.com/webhook",
		Secret:         "",
		Payload:        `{"type":"test"}`,
		Status:         model.OutboxStatusDead,
		Attempt:        3,
		MaxAttempts:    3,
		TimeoutSeconds: 5,
		NextAttemptAt:  time.Now(),
		LastError:      "connection refused",
	}
	if err := db.Create(&entry).Error; err != nil {
		t.Fatalf("failed to create dead entry: %v", err)
	}

	w := worker.NewWebhookWorker(repo, newTestDispatcher(t))
	if err := w.ReplayDeadEntry(context.Background(), entry.ID); err != nil {
		t.Fatalf("ReplayDeadEntry failed: %v", err)
	}

	var outbox model.WebhookOutbox
	if err := db.Where("id = ?", entry.ID).First(&outbox).Error; err != nil {
		t.Fatalf("failed to find entry: %v", err)
	}
	if outbox.Status != model.OutboxStatusPending {
		t.Errorf("expected status pending after replay, got %s", outbox.Status)
	}
	if outbox.Attempt != 0 {
		t.Errorf("expected attempt reset to 0, got %d", outbox.Attempt)
	}
	if outbox.LastError != "" {
		t.Errorf("expected last_error cleared, got %q", outbox.LastError)
	}
}

func TestReplayDeadBatch(t *testing.T) {
	db := setupWorkerTestDB(t)
	repo := repository.NewWebhookRepository(db)

	for i := 0; i < 3; i++ {
		entry := model.WebhookOutbox{
			ID:             kexrandom.MustUUIDString(),
			EventType:      "test",
			EventID:        fmt.Sprintf("evt-batch-replay-%d", i),
			TenantID:       "tenant-1",
			WebhookID:      "wh-1",
			URL:            "https://example.com/webhook",
			Secret:         "",
			Payload:        `{"type":"test"}`,
			Status:         model.OutboxStatusDead,
			Attempt:        3,
			MaxAttempts:    3,
			TimeoutSeconds: 5,
			NextAttemptAt:  time.Now(),
			LastError:      "timeout",
		}
		if err := db.Create(&entry).Error; err != nil {
			t.Fatalf("failed to create dead entry: %v", err)
		}
	}

	w := worker.NewWebhookWorker(repo, newTestDispatcher(t))
	replayed, err := w.ReplayDeadBatch(context.Background(), 10)
	if err != nil {
		t.Fatalf("ReplayDeadBatch failed: %v", err)
	}
	if replayed != 3 {
		t.Errorf("expected 3 replayed, got %d", replayed)
	}

	var pendingCount int64
	db.Model(&model.WebhookOutbox{}).Where("status = ?", model.OutboxStatusPending).Count(&pendingCount)
	if pendingCount != 3 {
		t.Errorf("expected 3 pending entries after replay, got %d", pendingCount)
	}
}
