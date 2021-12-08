package shard

import "google.golang.org/grpc/balancer"

const Name = "shard"

type ShardLBCtxKeyType string

const DefaultShardKeyName ShardLBCtxKeyType = "shard_key"

func init() {
	b, err := NewBuilder()
	if err != nil {
		panic(err)
	}

	balancer.Register(b)
}
