package file

import (
	"github.com/moderntv/cadre/registry"
)

type (
	servicesMap map[string]instances
	instances   []registry.Instance
)

type instance struct {
	serviceName string `mapstructure:"service_name"`
	addr        string `mapstructure:"addr"`
}

func (i *instance) ServiceName() string {
	return i.serviceName
}

func (i *instance) Address() string {
	return i.addr
}
