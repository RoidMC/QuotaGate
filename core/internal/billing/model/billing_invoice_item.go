// QuotaGate-only model: billing invoice line item (not shared with KexCore IAM)

package billingmodel

import "time"

// InvoiceItem is a single line on an invoice, linking the invoice to the
// underlying business record(s) it aggregates. Without this, an invoice only
// carries a total Amount and enterprises cannot reconcile which orders or
// transactions were billed.
//
// InvoiceItem rows are never physically deleted because they are part of a
// billing record.  Logical deletion is performed by setting Status to
// "cancelled".
//
// SourceType / SourceID form a polymorphic link:
//   - "order"       -> SourceID = Order.ID
//   - "transaction" -> SourceID = Transaction.ID
//   - "subscription"-> SourceID = Subscription.ID
//   - "adjustment"  -> SourceID = "" (manual line, Description required)
type InvoiceItem struct {
	ID          string    `gorm:"primaryKey;size:36" json:"id"`
	TenantID    string    `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	InvoiceID   string    `gorm:"column:invoice_id;size:36;index;not null" json:"invoice_id"`
	InvoiceNo   string    `gorm:"column:invoice_no;size:64" json:"invoice_no"` // denormalized for querying
	UserID      string    `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	SourceType  string    `gorm:"column:source_type;size:16;not null;index" json:"source_type"` // order / transaction / subscription / adjustment
	SourceID    string    `gorm:"column:source_id;size:36;index" json:"source_id"`              // empty for manual adjustment lines
	Description string    `gorm:"size:255" json:"description"`
	Quantity    int64     `gorm:"not null;default:1" json:"quantity"`                     // number of units (tokens / requests / months)
	UnitPrice   int64     `gorm:"column:unit_price;not null;default:0" json:"unit_price"` // smallest currency unit per unit
	Amount      int64     `gorm:"not null;default:0" json:"amount"`                       // Quantity * UnitPrice before tax
	TaxRate     float64   `gorm:"column:tax_rate;not null;default:0" json:"tax_rate"`     // e.g. 0.13 for 13% VAT
	TaxAmount   int64     `gorm:"column:tax_amount;not null;default:0" json:"tax_amount"`
	TotalAmount int64     `gorm:"column:total_amount;not null;default:0" json:"total_amount"` // Amount + TaxAmount
	Status      string    `gorm:"size:16;not null;default:'active';index" json:"status"`      // active / cancelled
	CreatedAt   time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt   time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (InvoiceItem) TableName() string { return "billing_invoice_items" }

func (InvoiceItem) TenantAware() bool { return true }

const (
	InvoiceItemStatusActive    = "active"
	InvoiceItemStatusCancelled = "cancelled" // logical deletion; never physically dropped

	InvoiceItemSourceOrder        = "order"
	InvoiceItemSourceTransaction  = "transaction"
	InvoiceItemSourceSubscription = "subscription"
	InvoiceItemSourceAdjustment   = "adjustment"
)
