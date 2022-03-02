package shard

import (
	"fmt"

	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
)

func NewBuilder() (balancer.Builder, error) {
	options := defaultBuilderOptions()
	pickerBuilder, err := newPickerBuilder(options)
	if err != nil {
		return nil, fmt.Errorf("cannot create picker builder: %v", err)
	}

	return base.NewBalancerBuilder(Name, pickerBuilder, base.Config{HealthCheck: true}), nil
}

func NewNamedBuilder(name string) (balancer.Builder, error) {
	options := defaultBuilderOptions()
	pickerBuilder, err := newPickerBuilder(options)
	if err != nil {
		return nil, fmt.Errorf("cannot create picker builder: %v", err)
	}

	return base.NewBalancerBuilder(name, pickerBuilder, base.Config{HealthCheck: true}), nil
}
