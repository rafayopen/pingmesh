package server

import (
	//"fmt"
	//"log"
	"net/http"
)

////
//  routes connects the incoming URI with a doc string and response handler
type route struct {
	uri     string
	doc     string
	handler func(w http.ResponseWriter, r *http.Request)
}

func (s *meshSrv) SetupRoutes() {
	s.routes = []route{
		{"/", "root", s.RootHandler},
		{"/v1", "root", s.RootHandler},
		{"/v1/metrics", "get memory statistics", s.MetricsHandler},
		{"/v1/peers", "get or update list of peers", s.PeersHandler},
		{"/v1/ping", "get a ping response", s.PingHandler},
	}
	for _, route := range s.routes {
		http.HandleFunc(route.uri, route.handler)
	}
}
