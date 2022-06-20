package shard

import (
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/grpclog"
)

type ShardLBCtxKeyType string

const (
	Name                                  = "shard"
	DefaultShardKeyName ShardLBCtxKeyType = "shard_key"
)

var logger = grpclog.Component("shard_lb")

func init() {
	b, err := NewBuilder()
	if err != nil {
		panic(err)
	}

	balancer.Register(b)
}
