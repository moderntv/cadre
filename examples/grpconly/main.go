// Command grpconly starts a Cadre server with a single gRPC interface on :9000.
//
// Health checking and reflection are registered by default, so the server can be inspected with grpcurl:
//
//	grpcurl -plaintext localhost:9000 list
//	grpcurl -plaintext -d '{"name":"Alice"}' localhost:9000 example.GreeterService/SayHi
//
// Without an HTTP server there is nowhere to expose /metrics and /status - configure
// cadre.WithMetricsListeningAddress and cadre.WithStatusListeningAddress to get them.
package main

import (
	"os"
	"time"

	"github.com/moderntv/cadre"
	"github.com/moderntv/cadre/examples"
	greeter_pb "github.com/moderntv/cadre/examples/proto/greeter"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
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

	b, err := cadre.NewBuilder(
		"example",
		cadre.WithLogger(logger),
		cadre.WithGRPC(
			cadre.WithGRPCListeningAddress(":9000"),
			cadre.WithService("example.GreeterService", greeterRegistrator),
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
