package model

import (
	"time"

	"github.com/roidmc/quotagate/internal/types"
)

type AuditLog struct {
	ID         string          `gorm:"primaryKey;size:36" json:"id"`
	TenantID   string          `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	RequestID  string          `gorm:"size:36;index" json:"request_id"`
	Action     types.EventType `gorm:"size:64;not null;index" json:"action"`
	ActorID    string          `gorm:"size:36;index" json:"actor_id"`
	TargetID   string          `gorm:"size:36;index" json:"target_id"`
	TargetType string          `gorm:"size:32;not null" json:"target_type"`
	Result     string          `gorm:"size:16;index;default:success" json:"result"`
	Severity   string          `gorm:"size:16;index;default:info" json:"severity"`
	Message    string          `gorm:"size:256" json:"message"`
	Detail     string          `gorm:"type:text" json:"detail"`
	Before     string          `gorm:"type:text" json:"before"` // 变更前快照
	After      string          `gorm:"type:text" json:"after"`  // 变更后快照
	IP         string          `gorm:"size:45" json:"ip"`
	UserAgent  string          `gorm:"size:512" json:"user_agent"`
	Signature  string          `gorm:"size:128;not null" json:"signature"` // HMAC-SHA256 Signature
	CreatedAt  time.Time       `gorm:"autoCreateTime;index" json:"created_at"`
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

const (
	TargetTypeUser        = "user"
	TargetTypeTenant      = "tenant"
	TargetTypeSession     = "session"
	TargetTypeMFAMethod   = "mfa_method"
	TargetTypeOAuthClient = "oauth_client"
	TargetTypePolicy      = "policy"
	TargetTypeRole        = "role"
	TargetTypeIdentity    = "identity"
	// QuotaGate Event
	TargetTypeChannel     = "channel" // 渠道启用/禁用/故障
	TargetTypeToken       = "token"   // API Key 创建/吊销
	TargetTypeModel       = "model"   // 模型路由变更
	TargetTypeQuota       = "quota"   // 配额变更
	TargetTypeBilling     = "billing" // 预扣费/结算/退款
	TargetTypeRelay       = "relay"   // 请求转发成功/失败
)

const (
	ResultSuccess = "success"
	ResultFailure = "failure"
	ResultDenied  = "denied"
)

const (
	SeverityInfo     = "info"
	SeverityWarn     = "warn"
	SeverityError    = "error"
	SeverityCritical = "critical"
)
