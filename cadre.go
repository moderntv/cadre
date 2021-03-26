package cadre

import (
	"context"
	"net"
	stdhttp "net/http"
	"strings"
	"sync"

	"github.com/rs/zerolog"
	"google.golang.org/grpc"
	"google.golang.org/grpc/health"

	"github.com/moderntv/cadre/metrics"
	"github.com/moderntv/cadre/status"
)

type Cadre interface {
	Start() error
	Shutdown() error
}

type httpServer struct {
	Services []string
	Server   *stdhttp.Server
	Mux      *stdhttp.ServeMux
}

type cadre struct {
	ctx       context.Context
	ctxCancel func()

	// config stuff that is good to know at runtime
	httpAddr       string
	grpcAddr       string
	channelzAddr   string
	prometheusAddr string

	logger  zerolog.Logger
	status  *status.Status
	metrics *metrics.Registry

	grpcHealthService *health.Server

	swg          sync.WaitGroup // services wait group
	grpcServer   *grpc.Server
	grpcListener net.Listener

	httpServers map[string]*httpServer
	httpServer  *stdhttp.Server
}

func (c *cadre) Start() error {
	// start http server
	c.swg.Add(1)
	go c.startHttp()

	// start grpc server
	c.swg.Add(1)
	go c.startGRPC()

	// start other http servers
	for _, server := range c.httpServers {
		c.swg.Add(1)
		c.startHttpServer(server)
	}

	<-c.ctx.Done()
	c.swg.Wait()
	return nil
}

func (c *cadre) Shutdown() error {
	c.ctxCancel()
	c.swg.Wait()
	return nil
}

func (c *cadre) startGRPC() {
	defer c.swg.Done()

	if c.grpcListener == nil || c.grpcServer == nil {
		c.logger.Trace().Msg("standalone grpc server disabled")

		return
	}

	go func() {
		c.logger.Debug().
			Str("addr", c.grpcAddr).
			Msg("starting grpc server")

		err := c.grpcServer.Serve(c.grpcListener)
		if err != nil {
			c.logger.Error().
				Err(err).
				Msg("grpc server failed")
		}

	}()
	<-c.ctx.Done()
}

func (c *cadre) startHttp() {
	defer c.swg.Done()

	if c.httpServer == nil {
		c.logger.Trace().Msg("standalone http server disabled")
		return
	}

	go func() {
		c.logger.Debug().
			Str("addr", c.httpAddr).
			Msg("starting http server")

		err := c.httpServer.ListenAndServe()
		if err != nil {
			c.logger.Error().
				Err(err).
				Msg("http server failed")
		}

	}()

	// TODO: cleanup
}

func (c *cadre) startHttpServer(server *httpServer) {
	defer c.swg.Done()

	go func() {
		c.logger.Debug().
			Str("addr", server.Server.Addr).
			Msg("starting " + strings.Join(server.Services, ", ") + " http server")

		err := server.Server.ListenAndServe()
		if err != nil {
			c.logger.Error().
				Err(err).
				Msg("http server failed")
		}

	}()
}
