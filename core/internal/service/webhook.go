package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/roidmc/quotagate/internal/event"
	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/repository"
	"github.com/roidmc/quotagate/internal/util/random"
	"github.com/roidmc/quotagate/pkg/kexswiftbus"
)

var (
	ErrWebhookInvalidURL     = errors.New("quotagate/service: invalid webhook URL")
	ErrWebhookNoEvents       = errors.New("quotagate/service: webhook must subscribe to at least one event")
	ErrWebhookConfigNotFound = errors.New("quotagate/service: webhook config not found")
)

type WebhookService struct {
	webhookRepo *repository.WebhookRepository
	bus         *event.EventBus
	dispatcher  *event.Dispatcher
	cancel      kexswiftbus.CancelFunc
}

func NewWebhookService(webhookRepo *repository.WebhookRepository, bus *event.EventBus) *WebhookService {
	svc := &WebhookService{
		webhookRepo: webhookRepo,
		bus:         bus,
		dispatcher:  event.NewDispatcher(30 * time.Second),
	}

	cancel, _ := bus.SubscribeEvent(event.Wildcard, svc.handleEvent)
	svc.cancel = cancel

	return svc
}

type CreateWebhookRequest struct {
	TenantID       string   `json:"tenant_id"`
	Name           string   `json:"name"`
	URL            string   `json:"url"`
	Secret         string   `json:"secret"`
	Events         []string `json:"events"`
	RetryCount     int      `json:"retry_count"`
	TimeoutSeconds int      `json:"timeout_seconds"`
}

type UpdateWebhookRequest struct {
	Name           *string   `json:"name"`
	URL            *string   `json:"url"`
	Secret         *string   `json:"secret"`
	Events         *[]string `json:"events"`
	Active         *bool     `json:"active"`
	RetryCount     *int      `json:"retry_count"`
	TimeoutSeconds *int      `json:"timeout_seconds"`
}

type WebhookResponse struct {
	ID             string   `json:"id"`
	TenantID       string   `json:"tenant_id"`
	Name           string   `json:"name"`
	URL            string   `json:"url"`
	Events         []string `json:"events"`
	Active         bool     `json:"active"`
	RetryCount     int      `json:"retry_count"`
	TimeoutSeconds int      `json:"timeout_seconds"`
	CreatedAt      string   `json:"created_at"`
	UpdatedAt      string   `json:"updated_at"`
}

func (s *WebhookService) Create(ctx context.Context, req *CreateWebhookRequest) (*WebhookResponse, error) {
	if req.URL == "" {
		return nil, ErrWebhookInvalidURL
	}
	if len(req.Events) == 0 {
		return nil, ErrWebhookNoEvents
	}

	eventsJSON := joinEvents(req.Events)
	retryCount := req.RetryCount
	if retryCount <= 0 {
		retryCount = 3
	}
	timeoutSeconds := req.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = 10
	}

	cfg := &model.WebhookConfig{
		ID:             random.MustUUIDString(),
		TenantID:       req.TenantID,
		Name:           req.Name,
		URL:            req.URL,
		Secret:         req.Secret,
		Events:         eventsJSON,
		Active:         true,
		RetryCount:     retryCount,
		TimeoutSeconds: timeoutSeconds,
	}

	if err := s.webhookRepo.Create(ctx, cfg); err != nil {
		return nil, fmt.Errorf("quotagate/service: failed to create webhook: %w", err)
	}

	return toWebhookResponse(cfg), nil
}

func (s *WebhookService) GetByID(ctx context.Context, id string) (*WebhookResponse, error) {
	cfg, err := s.webhookRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return nil, ErrWebhookConfigNotFound
		}
		return nil, fmt.Errorf("quotagate/service: failed to find webhook: %w", err)
	}
	return toWebhookResponse(cfg), nil
}

func (s *WebhookService) ListByTenant(ctx context.Context, tenantID string) ([]WebhookResponse, error) {
	configs, err := s.webhookRepo.FindByTenantID(ctx, tenantID)
	if err != nil {
		return nil, fmt.Errorf("quotagate/service: failed to list webhooks: %w", err)
	}

	responses := make([]WebhookResponse, len(configs))
	for i, cfg := range configs {
		responses[i] = *toWebhookResponse(&cfg)
	}
	return responses, nil
}

func (s *WebhookService) Update(ctx context.Context, id string, req *UpdateWebhookRequest) (*WebhookResponse, error) {
	cfg, err := s.webhookRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return nil, ErrWebhookConfigNotFound
		}
		return nil, fmt.Errorf("quotagate/service: failed to find webhook: %w", err)
	}

	if req.Name != nil {
		cfg.Name = *req.Name
	}
	if req.URL != nil {
		cfg.URL = *req.URL
	}
	if req.Secret != nil {
		cfg.Secret = *req.Secret
	}
	if req.Events != nil {
		cfg.Events = joinEvents(*req.Events)
	}
	if req.Active != nil {
		cfg.Active = *req.Active
	}
	if req.RetryCount != nil {
		cfg.RetryCount = *req.RetryCount
	}
	if req.TimeoutSeconds != nil {
		cfg.TimeoutSeconds = *req.TimeoutSeconds
	}

	if err := s.webhookRepo.Update(ctx, cfg); err != nil {
		return nil, fmt.Errorf("quotagate/service: failed to update webhook: %w", err)
	}

	return toWebhookResponse(cfg), nil
}

func (s *WebhookService) Delete(ctx context.Context, id string) error {
	if err := s.webhookRepo.Delete(ctx, id); err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return ErrWebhookConfigNotFound
		}
		return fmt.Errorf("quotagate/service: failed to delete webhook: %w", err)
	}
	return nil
}

// RotateSecret generates a new random secret for the webhook endpoint and
// persists it. The new secret is returned exactly once; callers must store it
// securely because it cannot be retrieved again.
func (s *WebhookService) RotateSecret(ctx context.Context, id string) (string, error) {
	cfg, err := s.webhookRepo.FindByID(ctx, id)
	if err != nil {
		if errors.Is(err, repository.ErrWebhookNotFound) {
			return "", ErrWebhookConfigNotFound
		}
		return "", fmt.Errorf("quotagate/service: failed to find webhook: %w", err)
	}

	newSecret, err := random.APIKey(32)
	if err != nil {
		return "", fmt.Errorf("quotagate/service: failed to generate secret: %w", err)
	}

	cfg.Secret = newSecret
	if err := s.webhookRepo.Update(ctx, cfg); err != nil {
		return "", fmt.Errorf("quotagate/service: failed to update webhook secret: %w", err)
	}

	return newSecret, nil
}

func (s *WebhookService) SubscribeToBus(eventType event.EventType, handler event.EventHandler) (kexswiftbus.CancelFunc, error) {
	return s.bus.SubscribeEvent(eventType, handler)
}

func (s *WebhookService) handleEvent(evt event.Event) {
	// event.EventHandler does not carry a context; use background context for
	// the lookup. The actual dispatch uses its own per-request context.
	configs, err := s.webhookRepo.FindActiveByEvent(context.Background(), evt.Type)
	if err != nil {
		return
	}

	for _, cfg := range configs {
		go s.dispatchToWebhook(evt, cfg)
	}
}

func (s *WebhookService) dispatchToWebhook(evt event.Event, cfg model.WebhookConfig) {
	payload, _ := json.Marshal(evt)

	result := s.dispatcher.DispatchWithRetry(context.Background(), evt, cfg.URL, cfg.Secret, time.Duration(cfg.TimeoutSeconds)*time.Second, cfg.RetryCount)

	deliveryLog := &model.WebhookDeliveryLog{
		ID:              random.MustUUIDString(),
		WebhookConfigID: cfg.ID,
		EventID:         evt.ID,
		EventType:       evt.Type,
		RequestURL:      cfg.URL,
		RequestBody:     string(payload),
		ResponseStatus:  result.StatusCode,
		ResponseBody:    result.ResponseBody,
		DurationMs:      result.DurationMs,
		Success:         result.Success,
		Attempt:         1,
		Error:           result.Error,
	}

	if err := s.webhookRepo.CreateDeliveryLog(context.Background(), deliveryLog); err != nil {
		slog.Error("quotagate/service: failed to create delivery log", "webhook_id", cfg.ID, "error", err)
	}
}

func (s *WebhookService) GetDeliveryLogs(ctx context.Context, webhookConfigID string, limit, offset int) ([]model.WebhookDeliveryLog, error) {
	return s.webhookRepo.ListDeliveryLogs(ctx, webhookConfigID, limit, offset)
}

func toWebhookResponse(cfg *model.WebhookConfig) *WebhookResponse {
	return &WebhookResponse{
		ID:             cfg.ID,
		TenantID:       cfg.TenantID,
		Name:           cfg.Name,
		URL:            cfg.URL,
		Events:         splitEvents(cfg.Events),
		Active:         cfg.Active,
		RetryCount:     cfg.RetryCount,
		TimeoutSeconds: cfg.TimeoutSeconds,
		CreatedAt:      cfg.CreatedAt.Format(time.RFC3339),
		UpdatedAt:      cfg.UpdatedAt.Format(time.RFC3339),
	}
}

func joinEvents(events []string) string {
	data, _ := json.Marshal(events)
	return string(data)
}

func splitEvents(eventsJSON string) []string {
	var events []string
	if err := json.Unmarshal([]byte(eventsJSON), &events); err != nil {
		return nil
	}
	return events
}
