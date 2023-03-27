package main

import (
	"context"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/spf13/viper"

	"github.com/moderntv/cadre"
	base_config "github.com/moderntv/cadre/_examples/config"
	"github.com/moderntv/cadre/config"
	"github.com/moderntv/cadre/config/encoder/yaml"
	"github.com/moderntv/cadre/config/source"
	"github.com/moderntv/cadre/config/source/environment"
	"github.com/moderntv/cadre/config/source/file"
	"github.com/moderntv/cadre/http"
	"github.com/moderntv/cadre/http/responses"
)

func maintainConfig(ctx context.Context, m *config.Manager, dst config.Config, configChange chan source.ConfigChange) {
	for {
		select {
		case s := <-configChange:
			if s.SourceName == "environment" {
				continue
			}

			m.LoadFromSource(dst, s.SourceName)
			m.Save(dst)
			// Reload config here

		case <-ctx.Done():
			return
		}
	}
}

func createConfig(ctx context.Context) {
	f, err := file.NewSource("tmp.yaml", yaml.NewEncoder())
	if err != nil {
		panic(err)
	}

	e, err := environment.NewSource("CADRE", viper.New())
	if err != nil {
		panic(err)
	}

	m, err := config.NewManager(
		config.WithSource(e),
		config.WithSource(f),
	)
	if err != nil {
		panic(err)
	}

	dst := &base_config.BaseConfig{}
	m.Load(dst)
	err = m.Save(dst)
	if err != nil {
		panic(err)
	}

	configChange, err := m.Subscribe(ctx)
	if err != nil {
		panic(err)
	}

	go maintainConfig(ctx, m, dst, configChange)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	createConfig(ctx)
	var logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	b, err := cadre.NewBuilder(
		"example",
		cadre.WithContext(ctx),
		cadre.WithLogger(logger),
		cadre.WithMetricsListeningAddress(":7000"),
		cadre.WithStatusListeningAddress(":7000"),
		cadre.WithHTTP(
			"main_http",
			cadre.WithHTTPListeningAddress(":8000"),
			cadre.WithRoutingGroup(http.RoutingGroup{
				Base: "",
				Routes: map[string]map[string][]gin.HandlerFunc{
					"/hello": map[string][]gin.HandlerFunc{
						"GET": []gin.HandlerFunc{
							func(c *gin.Context) {
								responses.Ok(c, gin.H{
									"hello": "world",
								})
							},
						},
					},
				},
			}),
		),
	)
	if err != nil {
		panic(err)
	}

	c, err := b.Build()
	if err != nil {
		panic(err)
	}

	c.Start()
	cancel()
	time.Sleep(1 * time.Second)
}
