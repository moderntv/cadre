package static

type instance struct {
	serviceName string
	addr        string
}

func (this *instance) ServiceName() string {
	return this.serviceName
}

func (this *instance) Address() string {
	return this.addr
}
