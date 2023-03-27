package config

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/moderntv/cadre/config/encoder/yaml"
	"github.com/moderntv/cadre/config/source/environment"
	"github.com/moderntv/cadre/config/source/file"
	"github.com/spf13/viper"
	"github.com/stretchr/testify/assert"
)

type BaseConfig struct {
	Type     string `mapstructure:"type"`
	FilePath string `mapstructure:"file_path"`
}

func (b *BaseConfig) PostLoad() error {
	return nil
}

func (b *BaseConfig) Merge(c any) error {
	m := c.(*BaseConfig)
	b.Type = m.Type
	b.FilePath = m.FilePath
	return nil
}

func TestManager(t *testing.T) {
	os.Setenv("CADRE_TYPE", "string")
	e, err := environment.NewSource("CADRE", viper.New())
	if err != nil {
		panic(err)
	}

	m, err := NewManager(
		WithSource(e),
	)
	if err != nil {
		panic(err)
	}

	var dst BaseConfig
	err = m.Load(&dst)
	if err != nil {
		panic(err)
	}

	assert.Equal(t, "string", dst.Type)
}

func TestManagerSubscribe(t *testing.T) {
	e, err := environment.NewSource("CADRE", viper.New())
	if err != nil {
		panic(err)
	}

	m, err := NewManager(
		WithSource(e),
	)
	if err != nil {
		panic(err)
	}

	var dst BaseConfig
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	configChange, err := m.Subscribe(ctx)
	if err != nil {
		panic(err)
	}

	os.Unsetenv("CADRE_TYPE")
	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return

			case <-time.After(4 * time.Second):
				os.Setenv("CADRE_TYPE", "strings")
			}
		}
	}(ctx)
	for {
		select {
		case _, more := <-configChange:
			if !more {
				assert.Equal(t, "strings", dst.Type)
				return
			}

			m.Load(&dst)
		}
	}
}

func TestManagerNoSave(t *testing.T) {
	e, err := environment.NewSource("CADRE", viper.New())
	if err != nil {
		panic(err)
	}

	m, err := NewManager(
		WithSource(e),
	)
	if err != nil {
		panic(err)
	}

	var dst BaseConfig
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
	defer cancel()
	configChange, err := m.Subscribe(ctx)
	if err != nil {
		panic(err)
	}

	go func(ctx context.Context) {
		for {
			select {
			case <-ctx.Done():
				return

			case <-time.After(4 * time.Second):
				os.Setenv("CADRE_TYPE", "strings")
			}
		}
	}(ctx)
	for {
		select {
		case _, more := <-configChange:
			if !more {
				m.Save(&dst)
				return
			}

			m.Load(&dst)
		}
	}
}

func TestManagerDumpEnvToFile(t *testing.T) {
	f, err := file.NewSource("tmp.yaml", yaml.NewEncoder())
	if err != nil {
		panic(err)
	}

	e, err := environment.NewSource("CADRE", viper.New())
	if err != nil {
		panic(err)
	}

	m, err := NewManager(
		WithSource(e),
		WithSource(f),
	)
	if err != nil {
		panic(err)
	}

	var dst BaseConfig
	ctx, cancel := context.WithTimeout(context.Background(), 9*time.Second)
	defer cancel()
	os.Setenv("CADRE_TYPE", "strings")
	m.Load(&dst)
	err = m.Save(&dst)
	if err != nil {
		panic(err)
	}

	configChange, err := m.Subscribe(ctx)
	if err != nil {
		panic(err)
	}

	go func(ctx context.Context, dst *BaseConfig) {
		for {
			select {
			case <-ctx.Done():
				return

			case <-time.After(4 * time.Second):
				dst.FilePath = "tmp.yaml"
				m.Save(dst)
			}
		}
	}(ctx, &dst)
	for {
		select {
		case s, more := <-configChange:
			if !more {
				os.Remove("tmp.yaml")
				return
			}

			if s.SourceName == "file" {
				os.Setenv("CADRE_TYPE", "int")
			} else {
				m.LoadFromSource(&dst, s.SourceName)
			}

		case <-ctx.Done():
			dst = BaseConfig{}
			m.Load(&dst)
			assert.Equal(t, BaseConfig{Type: "int", FilePath: "tmp.yaml"}, dst)
			os.Remove("tmp.yaml")
			return
		}
	}

}
