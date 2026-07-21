package captcha

// defaultRegistry is populated by init()s of real provider packages
// (standard, china, mock, ...). A var initializer is used so the registry
// exists before any provider init() runs and registers into it.
var defaultRegistry = NewRegistry()

// DefaultRegistry returns the registry populated by real provider init()s.
// Importing a provider package (e.g. _ ".../plugin/captcha/standard") causes
// its factory to self-register.
func DefaultRegistry() *Registry {
	return defaultRegistry
}
