// QuotaGate-only model: billing subscription order (not shared with KexCore IAM)

package billingmodel

import "time"

// SubscriptionOrder is the payment order for purchasing a subscription plan.
// It bridges the payment gateway (Stripe / Creem / ePay / balance) and the
// resulting UserSubscription. A completed order triggers CreateUserSubscriptionFromPlanTx.
//
// Lifecycle:
//
//	pending -> success  (payment confirmed via webhook or balance pay)
//	pending -> expired  (timeout without payment)
//	pending -> failed   (payment gateway returned failure)
type SubscriptionOrder struct {
	ID              string     `gorm:"primaryKey;size:36" json:"id"`
	TenantID        string     `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID          string     `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	PlanID          string     `gorm:"column:plan_id;size:36;index;not null" json:"plan_id"`
	Amount          int64      `gorm:"not null;default:0" json:"amount"` // smallest currency unit
	Currency        string     `gorm:"size:8;not null;default:'CNY'" json:"currency"`
	TradeNo         string     `gorm:"column:trade_no;size:128;uniqueIndex;not null" json:"trade_no"`      // idempotent trade number shared with payment gateway
	PaymentMethod   string     `gorm:"column:payment_method;size:32" json:"payment_method"`                // balance / stripe / creem / epay / waffo
	PaymentProvider string     `gorm:"column:payment_provider;size:32;default:''" json:"payment_provider"` // raw provider key for cross-gateway callback verification
	Status          string     `gorm:"size:16;not null;default:'pending';index" json:"status"`
	ProviderPayload string     `gorm:"column:provider_payload;type:text" json:"provider_payload"` // raw provider callback payload for audit
	CreateTime      time.Time  `gorm:"column:create_time;not null" json:"create_time"`
	CompleteTime    *time.Time `gorm:"column:complete_time" json:"complete_time,omitempty"`
	CreatedAt       time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt       time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
}

func (SubscriptionOrder) TableName() string { return "billing_subscription_orders" }

func (SubscriptionOrder) TenantAware() bool { return true }

const (
	SubscriptionOrderStatusPending = "pending"
	SubscriptionOrderStatusSuccess = "success"
	SubscriptionOrderStatusExpired = "expired"
	SubscriptionOrderStatusFailed  = "failed"

	SubscriptionPaymentMethodBalance = "balance"
	SubscriptionPaymentMethodStripe  = "stripe"
	SubscriptionPaymentMethodCreem   = "creem"
	SubscriptionPaymentMethodEpay    = "epay"
	SubscriptionPaymentMethodWaffo   = "waffo"

	SubscriptionPaymentProviderBalance = "balance"
	SubscriptionPaymentProviderStripe  = "stripe"
	SubscriptionPaymentProviderCreem   = "creem"
	SubscriptionPaymentProviderEpay    = "epay"
	SubscriptionPaymentProviderWaffo   = "waffo"
)
