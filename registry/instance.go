package registry

type Instance interface {
	ServiceName() string
	Address() string
}
