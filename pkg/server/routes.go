package server

import (
	//"fmt"
	//"log"
	"net/http"
)

////
//  Routes connects the incoming URI with a doc string and response handler
type Routes struct {
	uri     string
	doc     string
	handler func(w http.ResponseWriter, r *http.Request)
}

func (s *server) SetupRoutes() {
	s.routes = []Routes{
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
