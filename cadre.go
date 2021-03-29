package cadre

import (
	"context"
	"net"
	stdhttp "net/http"
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

type cadre struct {
	ctx       context.Context
	ctxCancel func()

	logger  zerolog.Logger
	status  *status.Status
	metrics *metrics.Registry

	grpcHealthService *health.Server

	swg          sync.WaitGroup // services wait group
	grpcAddr     string
	grpcServer   *grpc.Server
	grpcListener net.Listener

	httpServers map[string]*stdhttp.Server
}

func (c *cadre) Start() error {
	// start http servers
	for port, httpServer := range c.httpServers {
		c.swg.Add(1)
		go c.startHttpServer(port, httpServer)
	}

	// start grpc server
	c.swg.Add(1)
	go c.startGRPC()

	<-c.ctx.Done()
	c.swg.Wait()
	return nil
}

func (c *cadre) Shutdown() error {
	c.ctxCancel()
	c.swg.Wait()
	return nil
}

func (c *cadre) startHttpServer(addr string, httpServer *stdhttp.Server) {
	defer c.swg.Done()

	c.logger.Debug().
		Str("addr", addr).
		Msg("starting http server")

	err := httpServer.ListenAndServe()
	if err != nil {
		c.logger.Error().
			Err(err).
			Msg("http server failed")
	}

	<-c.ctx.Done()
}

func (c *cadre) startGRPC() {
	defer c.swg.Done()

	c.logger.Debug().Interface("grpclistener", c.grpcListener)
	if c.grpcListener == nil || c.grpcServer == nil {
		c.logger.Trace().Msg("standalone grpc server disabled")

		return
	}

	c.logger.Debug().
		Str("addr", c.grpcAddr).
		Msg("starting grpc server")

	err := c.grpcServer.Serve(c.grpcListener)
	if err != nil {
		c.logger.Error().
			Err(err).
			Msg("grpc server failed")
	}

	<-c.ctx.Done()
}
