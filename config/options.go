package config

import (
	"errors"

	"github.com/moderntv/cadre/config/source"
)

type options struct {
	sources []source.Source
	prefix  string
}

func defaultOptions() *options {
	return &options{
		sources: []source.Source{},
		prefix:  "",
	}
}

type Option func(opts *options) error

func WithPrefix(prefix string) Option {
	return func(opts *options) error {
		opts.prefix = prefix
		if opts.prefix == "" {
			return errors.New("no prefix setted up")
		}

		return nil
	}
}

func WithSource(source source.Source) Option {
	return func(opts *options) error {
		opts.sources = append(opts.sources, source)

		return nil
	}
}
