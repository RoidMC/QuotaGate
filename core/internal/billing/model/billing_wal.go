// QuotaGate-only model: billing WAL (not shared with KexCore IAM)

package billingmodel

import "time"

// WALRow represents a billing write-ahead log row.
// WAL rows are never physically deleted because Order and Transaction reference
// them during reconciliation.  Deletion is performed by setting Status to
// "cancelled".
type WALRow struct {
	ID        int64  `json:"id" gorm:"primaryKey;autoIncrement"`
	RequestID string `json:"request_id" gorm:"type:varchar(64);uniqueIndex;not null"`
	TenantID  string `json:"tenant_id" gorm:"type:varchar(36);index;not null;default:''"`
	UserID    string `json:"user_id" gorm:"type:varchar(36);index;not null"`
	// User snapshot at billing time.  IAM deletes users hard, so this snapshot
	// preserves identity for audit even after the user record is gone.
	UserSnapshotName       string     `json:"user_snapshot_name" gorm:"type:varchar(128)"`
	UserSnapshotIdentifier string     `json:"user_snapshot_identifier" gorm:"type:varchar(256)"` // email / phone / SAML UID
	SubscriptionID         *string    `json:"subscription_id,omitempty" gorm:"column:subscription_id;size:36;index"`
	PreConsumed            int64      `json:"pre_consumed" gorm:"not null"`
	ActualConsumed         int64      `json:"actual_consumed" gorm:"default:0"`
	Status                 string     `json:"status" gorm:"type:varchar(32);not null;default:'pending';index"`
	BillingSource          string     `json:"billing_source" gorm:"type:varchar(32);not null"`
	PaymentMethod          string     `json:"payment_method" gorm:"type:varchar(32)"`
	CreatedAt              time.Time  `json:"created_at"`
	UpdatedAt              time.Time  `json:"updated_at"`
	SettledAt              *time.Time `json:"settled_at,omitempty"`
	RefundedAt             *time.Time `json:"refunded_at,omitempty"`
	ErrorMessage           string     `json:"error_message" gorm:"type:text"`
}

func (WALRow) TableName() string { return "billing_wal" }

func (WALRow) TenantAware() bool { return true }

const (
	WALStatusPending   = "pending"
	WALStatusSettled   = "settled"
	WALStatusRefunded  = "refunded"
	WALStatusFailed    = "failed"
	WALStatusCancelled = "cancelled" // logical deletion; never physically dropped

	WALSourceWallet       = "wallet"
	WALSourceSubscription = "subscription"
)
