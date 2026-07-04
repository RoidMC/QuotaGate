package repository

import (
	"context"
	"log/slog"
	"time"

	"github.com/roidmc/quotagate/internal/model"
	"github.com/roidmc/quotagate/internal/types"
	"github.com/roidmc/quotagate/internal/util/audit"
	"github.com/roidmc/quotagate/internal/util/random"
	"gorm.io/gorm"
)

type AuditLogRepository struct {
	db      *gorm.DB
	signKey string
}

type AuditFilter struct {
	ActorID    string
	Action     string
	TargetType string
	TargetID   string
	Result     string
	Severity   string
	StartTime  *time.Time
	EndTime    *time.Time
}

func NewAuditLogRepository(db *gorm.DB, signKey string) *AuditLogRepository {
	return &AuditLogRepository{db: db, signKey: signKey}
}

func (r *AuditLogRepository) AutoMigrate() error {
	return r.db.AutoMigrate(&model.AuditLog{})
}

func (r *AuditLogRepository) Create(ctx context.Context, entry *model.AuditLog) error {
	if err := r.db.WithContext(ctx).Create(entry).Error; err != nil {
		slog.Warn("failed to create audit log", "action", entry.Action, "actor_id", entry.ActorID, "error", err)
		return err
	}
	return nil
}

func (r *AuditLogRepository) BatchCreate(ctx context.Context, entries []*model.AuditLog) error {
	if len(entries) == 0 {
		return nil
	}
	if err := r.db.WithContext(ctx).CreateInBatches(entries, 100).Error; err != nil {
		slog.Warn("failed to batch create audit logs", "count", len(entries), "error", err)
		return err
	}
	return nil
}

func (r *AuditLogRepository) ListByFilter(ctx context.Context, tenantID string, filter AuditFilter, limit, offset int) ([]model.AuditLog, error) {
	var logs []model.AuditLog
	query := r.db.WithContext(ctx).Where("tenant_id = ?", tenantID)

	if filter.ActorID != "" {
		query = query.Where("actor_id = ?", filter.ActorID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.TargetType != "" {
		query = query.Where("target_type = ?", filter.TargetType)
	}
	if filter.TargetID != "" {
		query = query.Where("target_id = ?", filter.TargetID)
	}
	if filter.Result != "" {
		query = query.Where("result = ?", filter.Result)
	}
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", filter.EndTime)
	}

	result := query.Order("created_at DESC").
		Limit(limit).Offset(offset).
		Find(&logs)
	if result.Error != nil {
		return nil, result.Error
	}

	for i := range logs {
		if err := r.verifyLogSignature(&logs[i]); err != nil {
			slog.Warn("audit log signature verification failed", "id", logs[i].ID, "error", err)
			logs[i].Result = "unknown"
		}
	}

	return logs, nil
}

func (r *AuditLogRepository) verifyLogSignature(log *model.AuditLog) error {
	return audit.VerifySignature(audit.AuditLogInput{
		Action:     string(log.Action),
		ActorID:    log.ActorID,
		TargetID:   log.TargetID,
		TargetType: log.TargetType,
		Result:     log.Result,
		TenantID:   log.TenantID,
		RequestID:  log.RequestID,
		Message:    log.Message,
		Timestamp:  log.CreatedAt,
	}, r.signKey, log.Signature)
}

func (r *AuditLogRepository) CountByFilter(ctx context.Context, tenantID string, filter AuditFilter) (int64, error) {
	var count int64
	query := r.db.WithContext(ctx).Model(&model.AuditLog{}).Where("tenant_id = ?", tenantID)

	if filter.ActorID != "" {
		query = query.Where("actor_id = ?", filter.ActorID)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.TargetType != "" {
		query = query.Where("target_type = ?", filter.TargetType)
	}
	if filter.TargetID != "" {
		query = query.Where("target_id = ?", filter.TargetID)
	}
	if filter.Result != "" {
		query = query.Where("result = ?", filter.Result)
	}
	if filter.Severity != "" {
		query = query.Where("severity = ?", filter.Severity)
	}
	if filter.StartTime != nil {
		query = query.Where("created_at >= ?", filter.StartTime)
	}
	if filter.EndTime != nil {
		query = query.Where("created_at <= ?", filter.EndTime)
	}

	if err := query.Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// NewEntry creates a new AuditLog entry with HMAC-SHA256 signature.
// Options (currently only WithBeforeAfter) can be passed to include snapshots.
func (r *AuditLogRepository) NewEntry(action types.EventType, actorID, targetID, targetType, result, severity, tenantID, requestID, ip, userAgent, message, detail string, opts ...NewEntryOption) *model.AuditLog {
	now := time.Now()

	s := audit.SanitizeAuditLog(audit.AuditLogInput{
		Action:     string(action),
		ActorID:    actorID,
		TargetID:   targetID,
		TargetType: targetType,
		Result:     result,
		Severity:   severity,
		TenantID:   tenantID,
		RequestID:  requestID,
		IP:         ip,
		UserAgent:  userAgent,
		Message:    message,
		Detail:     detail,
	})

	signature := audit.ComputeSignature(audit.AuditLogInput{
		Action:     s.Action,
		ActorID:    s.ActorID,
		TargetID:   s.TargetID,
		TargetType: s.TargetType,
		Result:     s.Result,
		TenantID:   s.TenantID,
		RequestID:  s.RequestID,
		Message:    s.Message,
		Timestamp:  now,
	}, r.signKey)

	entry := &model.AuditLog{
		ID:         random.MustUUIDString(),
		TenantID:   s.TenantID,
		RequestID:  s.RequestID,
		Action:     types.EventType(s.Action),
		ActorID:    s.ActorID,
		TargetID:   s.TargetID,
		TargetType: s.TargetType,
		Result:     s.Result,
		Severity:   s.Severity,
		Message:    s.Message,
		Detail:     s.Detail,
		IP:         s.IP,
		UserAgent:  s.UserAgent,
		Signature:  signature,
		CreatedAt:  now,
	}

	for _, opt := range opts {
		opt(entry)
	}

	entry.Before = audit.SanitizeAuditLog(audit.AuditLogInput{Before: entry.Before}).Before
	entry.After = audit.SanitizeAuditLog(audit.AuditLogInput{After: entry.After}).After

	return entry
}

// NewEntryOption customizes an audit log entry created by NewEntry.
type NewEntryOption func(*model.AuditLog)

// WithBeforeAfter attaches before/after snapshots (useful for data mutations).
func WithBeforeAfter(before, after string) NewEntryOption {
	return func(entry *model.AuditLog) {
		entry.Before = before
		entry.After = after
	}
}
