// Command separate-metrics-status shows how the internal /metrics and /status endpoints are placed.
//
// By default they are merged into the first configured HTTP server. Giving them an explicit address moves
// them onto their own internal HTTP server, which is what you want when the main server is exposed publicly:
//
//	:8000 - HTTP    /hello
//	:7000 - metrics /metrics
//	:7010 - status  /status
//
// Both endpoints can also share one internal server by passing the same address to both options, and either
// one can be left out to keep it on the main HTTP server.
package main

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moderntv/cadre"
	"github.com/moderntv/cadre/http"
	"github.com/moderntv/cadre/http/responses"
	"github.com/rs/zerolog"
)

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	b, err := cadre.NewBuilder(
		"example",
		cadre.WithLogger(logger),
		cadre.WithMetricsListeningAddress(":7000"),
		cadre.WithStatusListeningAddress(":7010"),
		cadre.WithHTTP(
			"main_http",
			cadre.WithHTTPListeningAddress(":8000"),
			cadre.WithRoutingGroup(http.RoutingGroup{
				Base: "",
				Routes: map[string]map[string][]gin.HandlerFunc{
					"/hello": {
						"GET": {
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
		logger.Fatal().Err(err).Msg("cannot configure cadre")
	}

	c, err := b.Build()
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot build cadre")
	}

	// Start blocks until the process is signalled (or Shutdown is called) and every server has stopped.
	err = c.Start()
	if err != nil {
		logger.Fatal().Err(err).Msg("cadre failed")
	}
}
