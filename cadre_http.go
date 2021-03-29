package cadre

import stdhttp "net/http"

type httpServer struct {
	services []string
	server   *stdhttp.Server
}
