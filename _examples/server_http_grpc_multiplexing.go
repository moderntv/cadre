package main

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/moderntv/cadre"
	"github.com/moderntv/cadre/_example/greeter"
	greeter_pb "github.com/moderntv/cadre/_example/proto/greeter"
	"github.com/moderntv/cadre/http"
	"github.com/moderntv/cadre/http/responses"
)

func main() {
	var logger = zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	greeterSvc := &greeter.GreeterService{}
	greeterRegistrator := func(s *grpc.Server) {
		greeter_pb.RegisterGreeterServiceServer(s, greeterSvc)
	}

	logger.Debug().Msg("building cadre")

	b, err := cadre.NewBuilder(
		"example",
		cadre.WithLogger(&logger),
		cadre.WithGRPC(
			cadre.WithGRPCMultiplex(),
			cadre.WithService("example.GreeterService", greeterRegistrator),
		),
		cadre.WithHTTP(
			cadre.WithHTTPListeningAddress(":9000"),
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

	panic(c.Start())
}
