package cadre

import (
	"context"
	"net"
	"net/http"
	stdhttp "net/http"
	"os"
	"os/signal"
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
	ctx              context.Context
	ctxCancel        func()
	finisherCallback Finisher
	handledSigs      []os.Signal
	sigsDone         chan bool

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
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, c.handledSigs...)

	go func() {
		n := 0
		for sig := range sigs {
			if c.finisherCallback == nil {
				c.sigsDone <- true
				break
			}

			if n >= 2 { // 3 SIGINTS kills me
				c.sigsDone <- true
				break
			}
			n += 1

			if c.finisherCallback != nil && n == 1 {
				go func(sig os.Signal) {
					c.finisherCallback(sig)
					c.sigsDone <- true
				}(sig)
			}
		}
	}()

	// start http servers
	for port, httpServer := range c.httpServers {
		c.swg.Add(1)
		go c.startHttpServer(port, httpServer)
	}

	// start grpc server
	c.swg.Add(1)
	go c.startGRPC()

	<-c.sigsDone
	c.shutdown()

	<-c.ctx.Done()

	return nil
}

// shutdown the context and waits for WaitGroup of goroutines
func (c *cadre) shutdown() error {
	c.ctxCancel()
	c.swg.Wait()

	return nil
}

// This function shutdown the Start function that is waiting for sigsDone. The Start function initiates the context
// cancelation and waits
func (c *cadre) Shutdown() error {
	c.sigsDone <- true
	close(c.sigsDone)
	return nil
}

func (c *cadre) startHttpServer(addr string, httpServer *stdhttp.Server) {
	defer c.swg.Done()

	c.logger.Debug().
		Str("addr", addr).
		Msg("starting http server")

	go func() {
		// wait for cadre's context to be done and shutdown the http server
		<-c.ctx.Done()
		httpServer.Shutdown(context.Background())
	}()

	err := httpServer.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		c.logger.Error().
			Err(err).
			Msg("http server failed")
	}
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

	go func() {
		// wait for cadre's context to be done and shutdown the grpc server
		<-c.ctx.Done()
		c.grpcServer.GracefulStop()
	}()

	err := c.grpcServer.Serve(c.grpcListener)
	if err != nil {
		c.logger.Error().
			Err(err).
			Msg("grpc server failed")
	}
}
