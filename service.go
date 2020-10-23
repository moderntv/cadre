package cadre

import (
	"github.com/moderntv/cadre/status"
	"google.golang.org/grpc/health"
	"google.golang.org/grpc/health/grpc_health_v1"
)

type Service interface {
	Name() string
	// Address() string
}

type service struct {
	name string
	// endpoint    string

	status        *status.ComponentStatus
	healthService *health.Server // can be nil
}

// newService creates a new service instance. if healthService is disabled in Server, it can be nil
func newService(name string, cs *status.ComponentStatus, healthService *health.Server) (i Service) {
	i = &service{
		name: name,

		status:        cs,
		healthService: healthService,
	}

	(i.(*service)).SetUnhealthy()

	return
}

func (i *service) Name() string { return i.name }

// func (i *instance) Address() string     { return i.endpoint }

func (i *service) SetHealthy() {
	if i.healthService == nil {
		return
	}

	i.status.SetStatus(status.OK, "OK")
	i.healthService.SetServingStatus(i.Name(), grpc_health_v1.HealthCheckResponse_SERVING)
}

func (i *service) SetUnhealthy() {
	if i.healthService == nil {
		return
	}

	i.status.SetStatus(status.ERROR, "unhealthy")
	i.healthService.SetServingStatus(i.Name(), grpc_health_v1.HealthCheckResponse_NOT_SERVING)
}
