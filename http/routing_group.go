package http

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

type grouper interface {
	Group(path string, handlers ...gin.HandlerFunc) *gin.RouterGroup
}

// RoutingGroup serves for setting up HTTP routes under Base route.
type RoutingGroup struct {
	Base       string
	Middleware []gin.HandlerFunc
	Routes     map[string]map[string][]gin.HandlerFunc // path:method:handlers
	Groups     []RoutingGroup
	Static     []StaticRoute
}

// StaticRoute serves for static route where static files are exposed under given Root path.
type StaticRoute struct {
	Path string
	Root string
	FS   http.FileSystem
}

func (rg RoutingGroup) Register(registrator grouper) (err error) {
	g := registrator.Group(rg.Base, rg.Middleware...)

	for path, methodHandlers := range rg.Routes {
		for method, handlers := range methodHandlers {
			g.Handle(method, path, handlers...)
		}
	}

	for _, staticRoute := range rg.Static {
		if staticRoute.Root == "" && staticRoute.FS == nil {
			panic("either `Root` or `FS` must be specified for static route")
		}
		if staticRoute.Root != "" && staticRoute.FS != nil {
			panic("cannot register static route with both `Root` and `FS` specified")
		}

		if staticRoute.Root != "" {
			g.Static(staticRoute.Path, staticRoute.Root)
		}

		if staticRoute.FS != nil {
			g.StaticFS(staticRoute.Path, staticRoute.FS)
		}
	}

	for _, subGroup := range rg.Groups {
		err = subGroup.Register(g)
		if err != nil {
			return
		}
	}

	return
}
