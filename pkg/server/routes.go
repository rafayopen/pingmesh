package server

import (
	//"fmt"
	//"log"
	"net/http"
)

////
//  Routes connects the incoming URI with a doc string and response handler
var Routes = []struct {
	uri     string
	doc     string
	handler func(w http.ResponseWriter, r *http.Request)
}{
	{"/", "root", RootHandler},
	{"/v1", "root", RootHandler},
	{"/v1/metrics", "get memory statistics", MetricsHandler},
	{"/v1/peers", "get or update list of peers", PeersHandler},
	{"/v1/ping", "get a ping response", PingHandler},
}

func SetupRoutes() {
	for _, route := range Routes {
		http.HandleFunc(route.uri, route.handler)
	}
}
