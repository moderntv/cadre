package shard

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

func NewBuilder() (balancer.Builder, error) {
	return NewNamedBuilder(Name)
}

func NewNamedBuilder(name string) (balancer.Builder, error) {
	options := defaultBuilderOptions()
	pickerBuilder := newPickerBuilder(options)
	return base.NewBalancerBuilder(name, pickerBuilder, base.Config{HealthCheck: true}), nil
}
