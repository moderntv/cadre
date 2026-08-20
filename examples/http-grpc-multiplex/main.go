// Command http-grpc-multiplex serves HTTP and gRPC on a single port (:9000).
//
// cadre.WithGRPCMultiplex() makes Cadre drop the standalone gRPC listener and instead route each request on
// the HTTP server's port by protocol: HTTP/2 requests with a `application/grpc` content type go to the gRPC
// server, everything else to gin.
//
//	curl localhost:9000/hello
//	grpcurl -plaintext -d '{"name":"Alice"}' localhost:9000 example.GreeterService/SayHi
//
// Note that WithGRPCMultiplex and WithGRPCListeningAddress are mutually exclusive.
package main

import (
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/moderntv/cadre"
	"github.com/moderntv/cadre/examples"
	greeter_pb "github.com/moderntv/cadre/examples/proto/greeter"
	"github.com/moderntv/cadre/http"
	"github.com/moderntv/cadre/http/responses"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	logger := zerolog.New(zerolog.ConsoleWriter{
		Out:        os.Stderr,
		TimeFormat: time.RFC3339,
	}).With().Timestamp().Logger()

	greeterSvc := &examples.GreeterService{}
	greeterRegistrator := func(s *grpc.Server) {
		greeter_pb.RegisterGreeterServiceServer(s, greeterSvc)
	}

	logger.Debug().Msg("building cadre")

	greeterCon, err := grpc.NewClient("localhost:9000", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		logger.Fatal().Err(err).Msg("cannot create connection to grpc server")
	}

	greeterClient := greeter_pb.NewGreeterServiceClient(greeterCon)

	b, err := cadre.NewBuilder(
		"example",
		cadre.WithLogger(logger),
		cadre.WithGRPC(
			cadre.WithGRPCMultiplex(),
			cadre.WithService("example.GreeterService", greeterRegistrator),
		),
		cadre.WithHTTP(
			"main_http",
			cadre.WithHTTPListeningAddress(":9000"),
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
					"/greet": {
						"GET": {
							func(c *gin.Context) {
								name := c.DefaultQuery("name", "world")

								res, err := greeterClient.SayHi(
									c.Request.Context(),
									&greeter_pb.GreetingRequest{Name: name},
								)
								if err != nil {
									responses.InternalError(c, responses.NewError(err))
									return
								}

								responses.Ok(c, res)
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
