package consul

import "github.com/moderntv/cadre/registry"

var _ registry.Instance = &instance{}

type instance struct {
	serviceName string `mapstructure:"service_name"`
	addr        string `mapstructure:"addr"`
}

func (this instance) ServiceName() string {
	return this.serviceName
}

func (this instance) Address() string {
	return this.addr
}
