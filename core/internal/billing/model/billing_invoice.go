// QuotaGate-only model: billing invoice (not shared with KexCore IAM)

package billingmodel

import "time"

// Invoice aggregates transactions or orders into a billable document.
// For enterprise customers requiring compliant invoicing (e.g. China fapiao,
// EU VAT invoices), the header carries tax fields and a status flow that
// supports draft -> issue -> send -> pay -> write_off / dispute / refund.
type Invoice struct {
	ID       string `gorm:"primaryKey;size:36" json:"id"`
	TenantID string `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID   string `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	// User snapshot at invoice time.  IAM deletes users hard, so this snapshot
	// preserves identity for audit even after the user record is gone.
	UserSnapshotName       string `gorm:"column:user_snapshot_name;size:128" json:"user_snapshot_name"`
	UserSnapshotIdentifier string `gorm:"column:user_snapshot_identifier;size:256" json:"user_snapshot_identifier"` // email / phone / SAML UID
	InvoiceNo              string `gorm:"column:invoice_no;size:64;uniqueIndex;not null" json:"invoice_no"`
	Type                   string `gorm:"size:32;not null;default:'consumption'" json:"type"` // consumption / recharge / subscription
	// Period covered by this invoice
	PeriodStart time.Time `gorm:"column:period_start;not null" json:"period_start"`
	PeriodEnd   time.Time `gorm:"column:period_end;not null" json:"period_end"`
	// Monetary amounts (smallest currency unit)
	Subtotal  int64   `gorm:"column:subtotal;not null;default:0" json:"subtotal"` // sum of line items before tax
	TaxRate   float64 `gorm:"column:tax_rate;not null;default:0" json:"tax_rate"` // e.g. 0.13 for 13% VAT
	TaxAmount int64   `gorm:"column:tax_amount;not null;default:0" json:"tax_amount"`
	Amount    int64   `gorm:"not null;default:0" json:"amount"` // Subtotal + TaxAmount (final billed amount)
	Currency  string  `gorm:"size:8;not null;default:'CNY'" json:"currency"`
	// Status flow: drafted -> issued -> sent -> viewed -> paid -> (written_off | disputed | cancelled)
	// For simple deployments only pending/paid are used; the rest support enterprise workflows.
	Status        string     `gorm:"size:16;not null;default:'pending';index" json:"status"`
	PaidAt        *time.Time `gorm:"column:paid_at" json:"paid_at,omitempty"`
	IssuedAt      *time.Time `gorm:"column:issued_at" json:"issued_at,omitempty"` // when the invoice was issued to the customer
	SentAt        *time.Time `gorm:"column:sent_at" json:"sent_at,omitempty"`     // when the invoice was emailed / pushed
	ViewedAt      *time.Time `gorm:"column:viewed_at" json:"viewed_at,omitempty"` // first time the customer opened it
	PaymentMethod string     `gorm:"column:payment_method;size:32" json:"payment_method"`
	// Refund / credit note linkage: if this invoice is a credit note (Type="refund"),
	// RelatedInvoiceID points to the original invoice it refunds.
	RelatedInvoiceID *string `gorm:"column:related_invoice_id;size:36;index" json:"related_invoice_id,omitempty"`
	// Customer billing fields required for compliant invoices (China fapiao / EU VAT)
	CustomerName    string `gorm:"column:customer_name;size:255" json:"customer_name"`
	CustomerTaxID   string `gorm:"column:customer_tax_id;size:64" json:"customer_tax_id"` // VAT / unified social credit code
	CustomerAddress string `gorm:"column:customer_address;size:512" json:"customer_address"`
	CustomerPhone   string `gorm:"column:customer_phone;size:64" json:"customer_phone"`
	// Free-form extension (PDF download URL, template id, provider payload, etc.)
	Metadata  string    `gorm:"type:text" json:"metadata"`
	CreatedAt time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Invoice) TableName() string { return "billing_invoices" }

func (Invoice) TenantAware() bool { return true }

const (
	InvoiceTypeConsumption  = "consumption"
	InvoiceTypeRecharge     = "recharge"
	InvoiceTypeSubscription = "subscription"
	InvoiceTypeRefund       = "refund" // credit note

	InvoiceStatusDrafted    = "drafted" // 草稿，未开具
	InvoiceStatusPending    = "pending" // 已开具待支付（兼容简单部署）
	InvoiceStatusIssued     = "issued"  // 已开具
	InvoiceStatusSent       = "sent"    // 已发送给客户
	InvoiceStatusViewed     = "viewed"  // 客户已查看
	InvoiceStatusPaid       = "paid"
	InvoiceStatusDisputed   = "disputed"    // 客户提出争议
	InvoiceStatusWrittenOff = "written_off" // 核销（无法收回）
	InvoiceStatusCancelled  = "cancelled"
	InvoiceStatusOverdue    = "overdue"
)
