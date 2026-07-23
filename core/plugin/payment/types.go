package payment

import (
	"context"

	"github.com/roidmc/kex-utils/pkg/kexpluginsdk"
)

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

// ProviderConfig is the runtime, per-(tenant, provider) configuration loaded
// from the database. It is passed to Factory.New to instantiate a provider for
// a single request.
type ProviderConfig struct {
	// TenantID is the tenant this config belongs to.
	TenantID string
	// Name is the provider identifier and must match the factory's Name.
	Name string
	// Extra carries provider-specific config (API keys, endpoints, ...).
	Extra map[string]string
}

// Factory builds a configured Provider for a single request. Factories
// self-register in init(); only New needs per-tenant config.
type Factory interface {
	kexpluginsdk.Factory
	// Type reports the provider family, e.g. "payment".
	Type() string
	// Version is the provider implementation version.
	Version() string
	// New returns a Provider configured with the given per-tenant config.
	New(cfg ProviderConfig) (Provider, error)
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
