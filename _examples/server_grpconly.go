package main

import (
	"os"
	"time"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"

	"github.com/moderntv/cadre"
	"github.com/moderntv/cadre/_example/greeter"
	greeter_pb "github.com/moderntv/cadre/_example/proto/greeter"
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
			cadre.WithGRPCListeningAddress(":9000"),
			cadre.WithService("example.GreeterService", greeterRegistrator),
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
