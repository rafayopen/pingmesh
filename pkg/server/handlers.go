package server

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"time"
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

type metrics struct {
	AppStart   time.Time
	NumPeers   int
	NumActive  int
	NumDeleted int
	MemStats   *MemStatSummary
}

func (s *meshSrv) SetupRoutes() {
	s.routes = []route{
		{"/", "", s.RootHandler},
		{"/v1", "", s.RootHandler},
		{"/v1/env", "", s.envHandler},
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
	htmlTrailer string = "\n</body></html>\n"
	routelist   string
)

func htmlHeader(title string) string {
	return "<html><head><title>" + title + "</title></head><body>\n"
}

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

		response := htmlHeader(s.SrvLoc)
		response += "<h1> pingmesh </h1>"
		response += "<p>Accessible URLs are:\n"
		response += routelist
		response += "<p>Served from " + s.SrvLoc + "\n"
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
		m := metrics{
			AppStart:   s.Start,
			MemStats:   GetMemStatSummary(),
			NumPeers:   s.NumActive + s.NumDeleted,
			NumActive:  s.NumActive,
			NumDeleted: s.NumDeleted,
		}

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		if err := enc.Encode(m); err != nil {
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

		if len(s.SrvHost) > 0 && s.SrvIPs == nil {
			log.Println("Try again, lookup IPs for", s.SrvHost)
			s.SrvIPs = GetIPs(s.SrvHost)
		}

		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		func() {
			s.mu.Lock()
			defer s.mu.Unlock()
			if err := enc.Encode(s); err != nil {
				http.Error(w, "Error converting peer to json",
					http.StatusInternalServerError)
			}
		}()

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
		response := htmlHeader(s.SrvLoc)
		response += "<h1> pingResponse </h1>"
		response += "<p>Served from " + s.SrvLoc + "\n"
		response += htmlTrailer

		w.Write([]byte(response))

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

////
//  envHandler dumps the shell environment, server and peer state
func (s *meshSrv) envHandler(w http.ResponseWriter, r *http.Request) {
	//	log.Println("EnvHandler called")

	response := htmlHeader(s.SrvLoc)
	response += "<h1> Runtime Envronment </h1>"
	response += "<p>Server in " + s.SrvLoc + " with environment:\n<pre>\n"
	env := os.Environ()
	sort.Strings(env)
	for _, pair := range env {
		response += pair + "\n"
	}
	w.Write([]byte(response))

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	response = "</pre>\n<h2> Server and Peer State </h2>\n<pre>"
	w.Write([]byte(response))

	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := enc.Encode(s); err != nil {
			http.Error(w, "Error converting peer to json",
				http.StatusInternalServerError)
		}
	}()

	response = "</pre>\n<h2> Memory Stats </h2>\n<pre>"
	w.Write([]byte(response))
	m := GetMemStatSummary()
	enc.Encode(m)

	w.Write([]byte("</pre>\n" + htmlTrailer))
}

////
//  QuitHandler reports the currently active peers for this server and
//  then closes the done channel, causing the pinger peers to exit.
//  When they have exited main() will return.
func (s *meshSrv) QuitHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("QuitHandler called, shutting down server")

	response := htmlHeader(s.SrvLoc)
	response += "<h1> quitResponse </h1>"
	response += "<p>Server in " + s.SrvLoc + " shutting down with these peers:\n<pre>\n"
	w.Write([]byte(response))

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := enc.Encode(s.Peers); err != nil {
			http.Error(w, "Error converting peer to json",
				http.StatusInternalServerError)
		}
	}()

	w.Write([]byte("</pre>\n" + htmlTrailer))
	////
	//  Close the meshSrv done channel so the pinger peers will exit.
	close(s.done)
}
