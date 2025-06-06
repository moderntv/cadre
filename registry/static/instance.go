package static

type instance struct {
	serviceName string
	addr        string
}

func (i *instance) ServiceName() string {
	return i.serviceName
}

func (i *instance) Address() string {
	return i.addr
}
