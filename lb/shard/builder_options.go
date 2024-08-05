package shard

import (
	"context"
)

type (
	builderOption  func(*builderOptions) error
	builderOptions struct {
		shardKeyFunc func(context.Context) string
	}
)

func defaultBuilderOptions() builderOptions {
	return builderOptions{
		shardKeyFunc: func(ctx context.Context) string {
			key, ok := ctx.Value(DefaultShardKeyName).(string)
			if !ok {
				return "NOT_FOUND"
			}
			return key
		},
	}
}

func WithShardKeyFunc(shardKeyFunc func(context.Context) string) builderOption {
	return func(opts *builderOptions) error {
		opts.shardKeyFunc = shardKeyFunc

		return nil
	}
}
