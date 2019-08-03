package server

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
)

////////////////////////////////////////////////////////////////////////
//  Request routing and endpoint documentation.
//  TODO: Create proper OpenAPI 3.0 spec and server side implementation
////////////////////////////////////////////////////////////////////////

////
//  routes connects the incoming URI with a doc string and response handler
type route struct {
	uri     string
	doc     string // see rootResponse()
	handler func(w http.ResponseWriter, r *http.Request)
}

func (s *meshSrv) SetupRoutes() {
	s.routes = []route{
		{"/", "", s.RootHandler},
		{"/v1", "", s.RootHandler},
		{"/v1/metrics", "get memory statistics", s.MetricsHandler},
		{"/v1/peers", "get or update list of peers", s.PeersHandler},
		{"/v1/ping", "get a ping response", s.PingHandler},
		{"/v1/quit", "shut down this pinger", s.QuitHandler},
	}
	for _, route := range s.routes {
		http.HandleFunc(route.uri, route.handler)
	}
}

////////////////////////////////////////////////////////////////////////
//  Handlers
////////////////////////////////////////////////////////////////////////

var (
	htmlHeader  string = "<html><head><title>pingmesh</title></head><body>\n"
	htmlTrailer string = "\n</body></html>\n"
	routelist   string
)

func bullet(url, text string) string {
	return "<li><a href=\"" + url + "\">" + text + "</a></li>\n"
}

func (s *meshSrv) RootHandler(w http.ResponseWriter, r *http.Request) {
	//log.Println("RootHandler")

	switch r.Method {
	case "GET":
		// return default pages with links to other API endpoints
		if len(routelist) == 0 { // TODO: or if routes changed...
			routelist = "<ul>\n"
			for _, route := range s.routes {
				if len(route.doc) > 0 {
					routelist += bullet(route.uri, route.doc)
				}
			}
			routelist += "</ul>\n"
		}

		response := htmlHeader
		response += "<h1> pingmesh </h1>"
		response += "<p>Accessible URLs are:\n"
		response += routelist
		response += "<p>Served from " + s.myLoc + "\n"
		response += htmlTrailer

		w.Write([]byte(response))

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

func (s *meshSrv) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	//log.Println("MetricsHandler")

	switch r.Method {
	case "GET":
		memStats := GetMemStatSummary()
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(memStats); err != nil {
			http.Error(w, "Error converting memStats to json",
				http.StatusInternalServerError)
		}

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

func (s *meshSrv) PeersHandler(w http.ResponseWriter, r *http.Request) {
	//log.Println("PeersHandler")

	switch r.Method {
	case "POST":
		////
		// TODO: Decide if we want to PUSH data at peers, or let each
		// one PULL data from peers they connect to, and decide to ping
		// those peers itself, rather than instructing someone else ...
		////
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body",
				http.StatusInternalServerError)
		}

		// decode body into JSON ... if properly formatted
		// or use argument parameters to control
		_ = body

		// handle incoming peer data

		// write response
		w.Write([]byte("Peer POST not implemented"))

	case "GET":
		// write response

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(s.peers); err != nil {
			http.Error(w, "Error converting peer to json",
				http.StatusInternalServerError)
		}

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

func (s *meshSrv) PingHandler(w http.ResponseWriter, r *http.Request) {
	//log.Println("PingHandler")

	//var h http.HandlerFunc
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body",
				http.StatusInternalServerError)
		}
		_ = body

		// write response
		w.Write([]byte("PeersHandler POST not implemented\n"))

	case "GET":
		// write response
		response := htmlHeader
		response += "<h1> pingResponse </h1>"
		response += "<p>Served from " + s.myLoc + "\n"
		response += htmlTrailer

		w.Write([]byte(response))

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

////
//  QuitHandler reports the currently active peers for this server and
//  then closes the done channel, causing the pinger peers to exit.
//  When they have exited main() will return.
func (s *meshSrv) QuitHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("QuitHandler called, shutting down server")

	response := htmlHeader
	response += "<h1> quitResponse </h1>"
	response += "<p>Server in " + s.myLoc + " shutting down with these peers:\n<pre>\n"
	w.Write([]byte(response))

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(s.peers); err != nil {
		http.Error(w, "Error converting peer to json",
			http.StatusInternalServerError)
	}

	w.Write([]byte("</pre>\n" + htmlTrailer))
	////
	//  Close the meshSrv done channel so the pinger peers will exit.
	close(s.done)
}
