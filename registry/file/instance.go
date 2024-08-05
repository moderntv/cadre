package file

import (
	"github.com/moderntv/cadre/registry"
)

type instance struct {
	serviceName string `mapstructure:"service_name"`
	addr        string `mapstructure:"addr"`
}

func (this *instance) ServiceName() string {
	return this.serviceName
}

func (this *instance) Address() string {
	return this.addr
}

type (
	servicesMap map[string]instances
	instances   []registry.Instance
)
