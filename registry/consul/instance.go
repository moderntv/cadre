package consul

import "github.com/moderntv/cadre/registry"

var _ registry.Instance = &instance{}

type instance struct {
	serviceName string `mapstructure:"service_name"`
	addr        string `mapstructure:"addr"`
}

func (i instance) ServiceName() string {
	return i.serviceName
}

func (i instance) Address() string {
	return i.addr
}
