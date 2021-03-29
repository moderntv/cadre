package cadre

import (
	"fmt"

	grpc_zerolog "github.com/rkollar/go-grpc-middleware/logging/zerolog"
	grpc_recovery "github.com/rkollar/go-grpc-middleware/recovery"
	"google.golang.org/grpc"
)

// GRPC Options
type grpcOptions struct {
	listeningAddress string
	// whether the grpc server should be on the same http server as the main http server
	multiplexWithHTTP bool

	services map[string]ServiceRegistrator

	// whether enable recovery middleware
	enableRecoveryMiddleware  bool
	recoveryMiddlewareOptions []grpc_recovery.Option

	// logging middleware
	enableLoggingMiddleware  bool
	loggingMiddlewareOptions []grpc_zerolog.Option

	// whether to register the grpc health service
	enableHealthService bool

	// whether to enable reflection
	enableReflection bool

	// whether grpc channelz should be enabled
	enableChannelz   bool
	channelzHttpAddr string

	// allow registration of custom interceptors
	extraUnaryInterceptors  []grpc.UnaryServerInterceptor
	extraStreamInterceptors []grpc.StreamServerInterceptor
}

func defaultGRPCOptions() *grpcOptions {
	return &grpcOptions{
		services:                 map[string]ServiceRegistrator{},
		enableRecoveryMiddleware: true,
		enableLoggingMiddleware:  true,
		enableHealthService:      true,
		enableReflection:         true,
		enableChannelz:           false,
		channelzHttpAddr:         ":8192",
		extraUnaryInterceptors:   []grpc.UnaryServerInterceptor{},
		extraStreamInterceptors:  []grpc.StreamServerInterceptor{},
	}
}

func (g *grpcOptions) ensure() (err error) {
	if g.listeningAddress == "" && !g.multiplexWithHTTP {
		err = fmt.Errorf("grpc server has to either have listening address or be set up to be multiplexed with http server")
		return
	}
	if g.listeningAddress != "" && g.multiplexWithHTTP {
		err = fmt.Errorf("grpc can be either configured with a standalone listening address or to be multiplexed with other grpc")
		return
	}

	return
}

type GRPCOption func(*grpcOptions) error

// WithGRPC configures cadre with a GRPC server
func WithGRPC(grpcOptions ...GRPCOption) Option {
	return func(b *Builder) error {
		if b.grpcOptions == nil {
			b.grpcOptions = defaultGRPCOptions()
		}

		for _, option := range grpcOptions {
			err := option(b.grpcOptions)
			if err != nil {
				return err
			}
		}

		return nil
	}
}

// WithGRPCListeningAddress configures gRPC's standalone server's listening address
func WithGRPCListeningAddress(addr string) GRPCOption {
	return func(g *grpcOptions) error {
		g.listeningAddress = addr

		return nil
	}
}

// WithGRPCMultiplex configures Cadre to multiplex grpc and http on the same port
func WithGRPCMultiplex() GRPCOption {
	return func(g *grpcOptions) error {
		g.multiplexWithHTTP = true

		return nil
	}
}

// WithService registers a new gRPC service to the Cadre's gRPC server
func WithService(name string, registrator ServiceRegistrator) GRPCOption {
	return func(g *grpcOptions) error {
		_, ok := g.services[name]
		if ok {
			return fmt.Errorf("service already registered to grpc server")
		}

		g.services[name] = registrator

		return nil
	}
}

// WithoutLogging disables logging middleware - default on
func WithoutLogging() GRPCOption {
	return func(g *grpcOptions) error {
		g.enableLoggingMiddleware = false

		return nil
	}
}

func WithLoggingOptions(opts []grpc_zerolog.Option) GRPCOption {
	return func(g *grpcOptions) error {
		g.loggingMiddlewareOptions = opts

		return nil
	}
}

// WithoutReflection disables gRPC's reflection service - default on
func WithoutReflection() GRPCOption {
	return func(g *grpcOptions) error {
		g.enableReflection = false

		return nil
	}
}

// WithChannelz enables gRPC's channelz http server and configures its listening address
// if the listening address is left empty, it will use the default value (:8123)
func WithChannelz(listenAddr string) GRPCOption {
	return func(g *grpcOptions) error {
		g.enableChannelz = true
		if listenAddr != "" {
			g.channelzHttpAddr = listenAddr
		}

		return nil
	}
}

// WithoutRecovery disables gRPC's recovery middleware
func WithoutRecovery() GRPCOption {
	return func(g *grpcOptions) error {
		g.enableRecoveryMiddleware = false

		return nil
	}
}

// WithRecoveryOptions configures gRPC's recovery middleware with custom options
func WithRecoveryOptions(opts []grpc_recovery.Option) GRPCOption {
	return func(g *grpcOptions) error {
		g.recoveryMiddlewareOptions = opts

		return nil
	}
}

// WithUnaryInterceptors adds custom grpc unary interceptor(s) to the end of the interceptor chain
// default order (if not disabled) - ctxtags, logging, recovery, metrics
func WithUnaryInterceptors(unaryInterceptors ...grpc.UnaryServerInterceptor) GRPCOption {
	return func(g *grpcOptions) error {
		g.extraUnaryInterceptors = append(g.extraUnaryInterceptors, unaryInterceptors...)

		return nil
	}
}

// WithStreamInterceptors adds custom grpc stream interceptor(s) to the end of the interceptor chain
// default order (if not disabled) - ctxtags, logging, recovery, metrics
func WithStreamInterceptors(streamInterceptors ...grpc.StreamServerInterceptor) GRPCOption {
	return func(g *grpcOptions) error {
		g.extraStreamInterceptors = append(g.extraStreamInterceptors, streamInterceptors...)

		return nil
	}
}
