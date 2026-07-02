// QuotaGate-only model: billing transaction ledger (not shared with KexCore IAM)

package model

import "time"

// Transaction records every balance/frozen changing movement for audit and reconciliation.
// It is append-only; corrections are written as new reversal transactions.
type Transaction struct {
	ID            string  `gorm:"primaryKey;size:36" json:"id"`
	TenantID      string  `gorm:"column:tenant_id;size:36;index;not null;default:''" json:"tenant_id"`
	UserID        string  `gorm:"column:user_id;size:36;index;not null" json:"user_id"`
	WalletID      string  `gorm:"column:wallet_id;size:36;index;not null" json:"wallet_id"`
	Type          string  `gorm:"size:32;not null;index" json:"type"`
	Direction     string  `gorm:"size:16;not null" json:"direction"` // credit / debit
	Amount        int64   `gorm:"not null" json:"amount"`
	BalanceBefore int64   `gorm:"column:balance_before;not null" json:"balance_before"`
	BalanceAfter  int64   `gorm:"column:balance_after;not null" json:"balance_after"`
	FrozenBefore  int64   `gorm:"column:frozen_before;not null" json:"frozen_before"`
	FrozenAfter   int64   `gorm:"column:frozen_after;not null" json:"frozen_after"`
	FrozenChange  int64   `gorm:"column:frozen_change;not null;default:0" json:"frozen_change"` // +lock / -release / -settle
	Currency      string  `gorm:"size:8;not null" json:"currency"`
	RequestID     string  `gorm:"column:request_id;size:64;index" json:"request_id"`
	WALID         *int64  `gorm:"column:wal_id;index" json:"wal_id,omitempty"`
	OrderID       *string `gorm:"column:order_id;size:36;index" json:"order_id,omitempty"`
	ReferenceID   string  `gorm:"column:reference_id;size:128;index" json:"reference_id"` // external payment / order / invoice id
	Description   string  `gorm:"size:255" json:"description"`
	Status        string  `gorm:"size:16;not null;default:'completed';index" json:"status"`
	OperatorID    string  `gorm:"column:operator_id;size:36;index" json:"operator_id"` // user / admin / system / task
	OperatorType  string  `gorm:"column:operator_type;size:32;not null;default:'system'" json:"operator_type"`
	// Operator snapshot at action time.  IAM deletes users hard, so this snapshot
	// preserves who performed the operation even after the user record is gone.
	OperatorName       string    `gorm:"column:operator_name;size:128" json:"operator_name"`
	OperatorIdentifier string    `gorm:"column:operator_identifier;size:256" json:"operator_identifier"` // email / phone / SAML UID
	CreatedAt          time.Time `gorm:"autoCreateTime;index" json:"created_at"`
	UpdatedAt          time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

func (Transaction) TableName() string { return "billing_transactions" }

const (
	TransactionTypeRecharge     = "recharge"
	TransactionTypeConsume      = "consume"
	TransactionTypeRefund       = "refund"
	TransactionTypePreConsume   = "pre_consume"
	TransactionTypePreRelease   = "pre_release"
	TransactionTypeCompensation = "compensation"
	TransactionTypeAdjustment   = "adjustment"

	TransactionDirectionCredit = "credit"
	TransactionDirectionDebit  = "debit"

	TransactionStatusCompleted = "completed"
	TransactionStatusPending   = "pending"
	TransactionStatusReversed  = "reversed"

	TransactionOperatorSystem = "system"
	TransactionOperatorUser   = "user"
	TransactionOperatorAdmin  = "admin"
	TransactionOperatorTask   = "task"
)
