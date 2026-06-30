package payment

import "context"

// Plugin interface is embedded by all payment plugins.
type Plugin interface {
	Name() string
	Version() string
	Type() string
}

// Provider is the core payment plugin interface.
type Provider interface {
	Plugin
	SupportedMethods() []string
	CreatePayment(ctx context.Context, req *PaymentRequest) (*PaymentResult, error)
	HandleCallback(ctx context.Context, payload []byte) (*CallbackResult, error)
	QueryOrder(ctx context.Context, tradeNo string) (*OrderStatus, error)
}

type PaymentRequest struct {
	UserID    int
	Amount    int64
	Currency  string
	Method    string
	TradeNo   string
	ReturnURL string
	NotifyURL string
	Metadata  map[string]string
	Subject   string
	Body      string
}

type PaymentResult struct {
	TradeNo          string
	ProviderTradeNo  string
	PayURL           string
	Params           map[string]string
	ExpiresAt        int64
	RequiresRedirect bool
}

type CallbackResult struct {
	TradeNo         string
	ProviderTradeNo string
	Status          string
	Amount          float64
	Currency        string
	PaymentMethod   string
	Signed          bool
	Raw             map[string]string
	Error           string
}

type OrderStatus struct {
	TradeNo         string
	ProviderTradeNo string
	Status          string
	Amount          float64
	Currency        string
	PaymentMethod   string
}
