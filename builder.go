package cadre

import (
	"fmt"
	"log"
	"net"
	stdhttp "net/http"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	grpc_prometheus "github.com/grpc-ecosystem/go-grpc-prometheus"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	channelz "github.com/rantav/go-grpc-channelz"
	grpc_middleware "github.com/rkollar/go-grpc-middleware"
	grpc_zerolog "github.com/rkollar/go-grpc-middleware/logging/zerolog"
	grpc_recovery "github.com/rkollar/go-grpc-middleware/recovery"
	grpc_ctxtags "github.com/rkollar/go-grpc-middleware/tags"
	"github.com/rs/zerolog"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	channelz_service "google.golang.org/grpc/channelz/service"
	"google.golang.org/grpc/health"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/grpc/reflection"

	"github.com/moderntv/cadre/http"
	"github.com/moderntv/cadre/http/responses"
	"github.com/moderntv/cadre/metrics"
	"github.com/moderntv/cadre/status"
)

type ServiceRegistrator func(*grpc.Server)

type Finisher func(sig os.Signal)

type Builder struct {
	name string // app name

	ctx context.Context

	finisherCallback Finisher
	handledSigs      []os.Signal

	// logging
	logger zerolog.Logger

	// status
	status               *status.Status
	statusHTTPServerAddr string
	statusPath           string

	// metrics
	metrics               *metrics.Registry
	prometheusRegistry    *prometheus.Registry
	metricsHTTPServerAddr string
	metricsPath           string

	grpcOptions *grpcOptions
	httpOptions []*httpOptions
}

// NewBuilder creates a new Builder instance and allows the user to configure the Cadre server by various options.
func NewBuilder(name string, options ...Option) (b *Builder, err error) {
	b = &Builder{
		name:        name,
		ctx:         context.Background(),
		handledSigs: []os.Signal{syscall.SIGINT, syscall.SIGTERM},

		logger: zerolog.Nop(),

		statusPath:  "/status",
		metricsPath: "/metrics",

		grpcOptions: nil,
		httpOptions: nil,
	}

	for _, option := range options {
		err = option(b)
		if err != nil {
			err = fmt.Errorf("cannot apply option: %w", err)
			return
		}
	}

	return
}

// Build validates the Builder's configuration and creates a new Cadre server.
func (b *Builder) Build() (c *cadre, err error) {
	err = b.ensure()
	if err != nil {
		err = fmt.Errorf("cadre builder validation is invalid: %w", err)
		return
	}
	// logger, status, metrics already exist here no matter what

	ctx, ctxCancel := context.WithCancel(b.ctx)
	c = &cadre{
		ctx:              ctx,
		ctxCancel:        ctxCancel,
		finisherCallback: b.finisherCallback,
		handledSigs:      b.handledSigs,
		finalizerDone:    make(chan bool),

		logger:  b.logger,
		status:  b.status,
		metrics: b.metrics,

		httpServers: make(map[string]*stdhttp.Server),
	}

	if b.httpOptions == nil && b.grpcOptions == nil {
		err = fmt.Errorf("both grpc and http will be disabled. what do you want me to do?")
		return
	}

	// extra http services init
	if b.metricsHTTPServerAddr != "" {
		err = WithHTTP("metrics_http",
			WithHTTPListeningAddress(b.metricsHTTPServerAddr),
			WithRoute("GET", b.metricsPath, gin.WrapH(promhttp.HandlerFor(b.prometheusRegistry, promhttp.HandlerOpts{}))),
		)(b)
		if err != nil {
			err = fmt.Errorf("adding metrics http server failed: %w", err)
			return
		}
	}

	if b.statusHTTPServerAddr != "" {
		err = WithHTTP("status_http",
			WithHTTPListeningAddress(b.statusHTTPServerAddr),
			WithRoute("GET", b.statusPath, func(c *gin.Context) {
				report := b.status.Report()
				if report.Status != status.ERROR {
					responses.Ok(c, report)
					return
				}

				c.AbortWithStatusJSON(503, gin.H{
					"data": report,
				})
			}),
		)(b)
		if err != nil {
			err = fmt.Errorf("adding status http server failed: %w", err)
			return
		}
	}

	if b.grpcOptions != nil && b.grpcOptions.enableChannelz {
		channelzHandler := channelz.CreateHandler("/", b.grpcOptions.listeningAddress)

		err = WithHTTP("channelz_http",
			WithHTTPListeningAddress(b.grpcOptions.channelzHttpAddr),
			WithRoute("GET", "/channelz/*path", func(c *gin.Context) {
				channelzHandler.ServeHTTP(c.Writer, c.Request)
			}),
		)(b)
		if err != nil {
			err = fmt.Errorf("adding channelz http server failed: %w", err)
			return
		}
	}

	// create and configure grpc server
	if b.grpcOptions != nil {
		err = b.buildGrpc(c)
		if err != nil {
			err = fmt.Errorf("grpc server building failed: %w", err)
			return
		}

		if b.grpcOptions.enableChannelz {
			channelz_service.RegisterChannelzServiceToServer(c.grpcServer)
		}
	}

	// create and configure http server
	var httpServers map[string]*http.HttpServer
	if b.httpOptions != nil {
		httpServers, err = b.buildHTTP(c, ctx)
		if err != nil {
			return
		}
	}

	c.httpServers = map[string]*stdhttp.Server{}
	for addr, httpServer := range httpServers {
		httpServer.LogRegisteredRoutes()

		var h stdhttp.Handler = httpServer

		// http+grpc multiplexing
		if b.grpcOptions != nil && b.grpcOptions.listeningAddress == addr {
			h = stdhttp.HandlerFunc(func(w stdhttp.ResponseWriter, r *stdhttp.Request) {
				log.Printf("handling http request. protomajor = %v; content-type = %v; headers = %v", r.ProtoMajor, r.Header.Get("content-type"), r.Header)
				if r.ProtoMajor == 2 && strings.HasPrefix(r.Header.Get("Content-Type"), "application/grpc") {
					c.grpcServer.ServeHTTP(w, r)
				} else {
					httpServer.ServeHTTP(w, r)
				}
			})

			log.Println("disable grpcListener")
			c.grpcListener = nil
		}

		c.httpServers[addr] = &stdhttp.Server{
			Addr:              addr,
			Handler:           h,
			ReadHeaderTimeout: 5 * time.Second,
		}
	}

	return
}

func (b *Builder) ensure() (err error) {
	// check prometheus & metrics. ensure they use the same prometheus registry
	if b.metrics != nil && b.prometheusRegistry != nil {
		err = fmt.Errorf("pass either existing metrics.Registry or prometheus.Registry. not both")
		return
	}
	// initialize metrics
	if b.metrics == nil {
		// if b.prometheusRegistry is nil, metrics will just create a new one
		b.metrics, err = metrics.NewRegistry(b.name, b.prometheusRegistry)
	}
	if b.prometheusRegistry == nil {
		b.prometheusRegistry = b.metrics.GetPrometheusRegistry()
	}

	// basic checks
	if b.status == nil {
		b.status = status.NewStatus("TODO")
	}

	// grpc checks
	if b.grpcOptions != nil {
		err = b.grpcOptions.ensure()
		if err != nil {
			return
		}
	}

	// http checks
	for _, httpServerOptions := range b.httpOptions {
		err = httpServerOptions.ensure()
		if err != nil {
			return
		}
	}

	// configure metrics + status endpoint http servers
	if len(b.httpOptions) >= 1 {
		if b.statusHTTPServerAddr == "" {
			b.statusHTTPServerAddr = b.httpOptions[0].listeningAddress
		}
		if b.metricsHTTPServerAddr == "" {
			b.metricsHTTPServerAddr = b.httpOptions[0].listeningAddress
		}

		if b.grpcOptions != nil && b.grpcOptions.multiplexWithHTTP {
			b.grpcOptions.listeningAddress = b.httpOptions[0].listeningAddress
		}
	}

	return
}

func (b *Builder) buildGrpc(c *cadre) (err error) {
	// metrics
	grpcMetrics := grpc_prometheus.NewServerMetrics()
	err = b.metrics.Register("grpc", grpcMetrics)
	if err != nil {
		err = fmt.Errorf("cannot register grpc metrics to metrics registry: %w", err)
		return
	}

	// interceptors - always in order: tags, logging, tracing, metrics, custom, recovery => recovery should be always the last one
	unaryInterceptors := []grpc.UnaryServerInterceptor{}
	streamInterceptors := []grpc.StreamServerInterceptor{}

	// logging
	if b.grpcOptions.enableLoggingMiddleware {
		unaryInterceptors = append(
			unaryInterceptors,
			grpc_ctxtags.UnaryServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zerolog.UnaryServerInterceptor(b.logger, b.grpcOptions.loggingMiddlewareOptions...),
		)
		streamInterceptors = append(
			streamInterceptors,
			grpc_ctxtags.StreamServerInterceptor(grpc_ctxtags.WithFieldExtractor(grpc_ctxtags.CodeGenRequestFieldExtractor)),
			grpc_zerolog.StreamServerInterceptor(b.logger, b.grpcOptions.loggingMiddlewareOptions...),
		)
	}

	// TODO: add tracing middleware
	// metrics middleware
	unaryInterceptors = append(unaryInterceptors, grpcMetrics.UnaryServerInterceptor())
	streamInterceptors = append(streamInterceptors, grpcMetrics.StreamServerInterceptor())

	// add extra interceptors
	unaryInterceptors = append(unaryInterceptors, b.grpcOptions.extraUnaryInterceptors...)
	streamInterceptors = append(streamInterceptors, b.grpcOptions.extraStreamInterceptors...)

	// recovery middleware
	if b.grpcOptions.enableRecoveryMiddleware {
		unaryInterceptors = append(unaryInterceptors, grpc_recovery.UnaryServerInterceptor(b.grpcOptions.recoveryMiddlewareOptions...))
		streamInterceptors = append(streamInterceptors, grpc_recovery.StreamServerInterceptor(b.grpcOptions.recoveryMiddlewareOptions...))
	}

	// create grpc server
	c.grpcAddr = b.grpcOptions.listeningAddress
	c.grpcServer = grpc.NewServer(
		grpc_middleware.WithUnaryServerChain(unaryInterceptors...),
		grpc_middleware.WithStreamServerChain(streamInterceptors...),
	)

	// replace gRPC logger
	// grpc_zerolog.ReplaceGrpcLoggerV2(b.logger.Level(zerolog.ErrorLevel))

	// register services
	// health service
	if b.grpcOptions.enableHealthService {
		healthService := health.NewServer()

		healthpb.RegisterHealthServer(c.grpcServer, healthService)
	}

	// reflection
	if b.grpcOptions.enableReflection {
		reflection.Register(c.grpcServer)
	}

	// user-specified grpc services
	for _, registrator := range b.grpcOptions.services {
		registrator(c.grpcServer)
	}

	// grpc listener
	if b.grpcOptions.listeningAddress != "" && !b.grpcOptions.multiplexWithHTTP {
		c.grpcListener, err = net.Listen("tcp", b.grpcOptions.listeningAddress)
		if err != nil {
			return
		}
	}

	return
}

func (b *Builder) buildHTTP(_ *cadre, cadreContext context.Context) (httpServers map[string]*http.HttpServer, err error) {
	httpServers = map[string]*http.HttpServer{}

	mergedHTTPOptions := map[string]*httpOptions{}
	for _, newServer := range b.httpOptions {
		addr := newServer.listeningAddress
		if existingServer, ok := mergedHTTPOptions[addr]; ok {
			mergedHTTPOptions[addr], err = existingServer.merge(newServer)
			if err != nil {
				return
			}

			continue
		}

		mergedHTTPOptions[newServer.listeningAddress] = newServer
	}

	for _, httpOptions := range mergedHTTPOptions {
		httpServers[httpOptions.listeningAddress], err = httpOptions.build(cadreContext, b.logger, b.metrics)
		if err != nil {
			return
		}
	}

	return
}
