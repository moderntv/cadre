package registry

type Service interface {
	Name() string
}

// Instance describes single instance. The instance is unique by service is provides and address (IP:port).
type Instance interface {
	ServiceName() string
	Address() string
}

// RegistryChangeType represents types of changes in registry.
type RegistryChangeType int

const (
	// Service registered to the service registry.
	RCTRegistered RegistryChangeType = iota
	// Service deregistered from the service registry.
	RCTDeregistered
)

// RegistryChange represents single change on Instance while doing changes in registry.
type RegistryChange struct {
	Instance Instance
	Type     RegistryChangeType
}

// Registry provides a way for services to register/deregister in services and resolve service name to an array of available (healthy) endpoints.
type Registry interface {
	Register(serviceInstance Instance) error
	Deregister(serviceInstance Instance) error
	Instances(service string) []Instance
	Watch(service string) (<-chan RegistryChange, func())
}

type service struct {
	name string
}

func (this *service) Name() string { return this.name }

type instance struct {
	serviceName string
	address     string
}

func (this *instance) getServiceName() string { return this.serviceName }
func (this *instance) getAddress() string     { return this.address }
