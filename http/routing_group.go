package http

import (
	"github.com/gin-gonic/gin"
)

type grouper interface {
	Group(path string, handlers ...gin.HandlerFunc) *gin.RouterGroup
}

type RoutingGroup struct {
	Base       string
	Middleware []gin.HandlerFunc
	Routes     map[string]map[string][]gin.HandlerFunc // path:method:handlers
	Groups     []RoutingGroup
	Static     []StaticRoute
}

type StaticRoute struct {
	Path string
	Root string
}

func (rg RoutingGroup) Register(registrator grouper) (err error) {
	g := registrator.Group(rg.Base, rg.Middleware...)

	for path, methodHandlers := range rg.Routes {
		for method, handlers := range methodHandlers {
			g.Handle(method, path, handlers...)
		}
	}

	for _, staticRoute := range rg.Static {
		g.Static(staticRoute.Path, staticRoute.Root)
	}

	for _, subGroup := range rg.Groups {
		err = subGroup.Register(g)
		if err != nil {
			return
		}
	}

	return
}
