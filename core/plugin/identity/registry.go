package identity

func init() {
	defaultRegistry = NewRegistry()
}

var defaultRegistry *Registry

func DefaultRegistry() *Registry {
	return defaultRegistry
}
