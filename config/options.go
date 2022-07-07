package config

import "github.com/moderntv/cadre/config/source"

type options struct {
	sources []source.Source
}

func defaultOptions() *options {
	return &options{
		sources: []source.Source{},
	}
}

type Option func(opts *options) error

func WithSource(source source.Source) Option {
	return func(opts *options) error {
		opts.sources = append(opts.sources, source)

		return nil
	}
}
