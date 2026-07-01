// QuotaGate-only model: billing invoice (not shared with KexCore IAM)

package model

import "time"

// Invoice aggregates transactions or orders into a billable document.
type Invoice struct {
	ID            string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID      string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID        string     `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	InvoiceNo     string     `gorm:"column:invoice_no;size:64;uniqueIndex;not null" json:"invoice_no"`
	Type          string     `gorm:"size:32;not null;default:'consumption'" json:"type"` // consumption / recharge / subscription
	PeriodStart   time.Time  `gorm:"column:period_start;not null" json:"period_start"`
	PeriodEnd     time.Time  `gorm:"column:period_end;not null" json:"period_end"`
	Amount        int64      `gorm:"not null" json:"amount"`
	Currency      string     `gorm:"size:8;not null" json:"currency"`
	Status        string     `gorm:"size:16;not null;default:'pending';index" json:"status"`
	PaidAt        *time.Time `gorm:"column:paid_at" json:"paid_at,omitempty"`
	PaymentMethod string     `gorm:"column:payment_method;size:32" json:"payment_method"`
	Metadata      string     `gorm:"type:text" json:"metadata"`
	CreatedAt     time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt     time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Invoice) TableName() string { return "billing_invoices" }

const (
	InvoiceTypeConsumption  = "consumption"
	InvoiceTypeRecharge     = "recharge"
	InvoiceTypeSubscription = "subscription"

	InvoiceStatusPending   = "pending"
	InvoiceStatusPaid      = "paid"
	InvoiceStatusCancelled = "cancelled"
	InvoiceStatusOverdue   = "overdue"
)
