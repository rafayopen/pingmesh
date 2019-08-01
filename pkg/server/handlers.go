package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

func (s *meshSrv) RootHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// return default pages with links to other API endpoints
		w.Write(s.rootResponse())

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

func (s *meshSrv) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		memStats := GetMemStatSummary()
		jsonBody, err := json.Marshal(memStats)
		if err != nil {
			http.Error(w, "Error converting results to json",
				http.StatusInternalServerError)
		}
		w.Write(jsonBody)

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

func (s *meshSrv) PeersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body",
				http.StatusInternalServerError)
		}

		// decode body into JSON ... if propertly formatted
		// or use argument parameters to control
		_ = body

		// handle incoming peer

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
		w.Write(s.pingResponse())

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

var (
	htmlHeader  string = "<html><head><title>pingmesh</title></head><body>\n"
	htmlTrailer string = "\n</body></html>\n"
	routelist   string
)

func bullet(url, text string) string {
	return "<li><a href=\"" + url + "\">" + text + "</a></li>\n"
}

func (s *meshSrv) rootResponse() []byte {
	if len(routelist) == 0 { // TODO: or if routes changed...
		routelist = "<ul>\n"
		for _, route := range s.routes {
			routelist += bullet(route.uri, route.doc)
		}
		routelist += "</ul>\n"
	}

	response := htmlHeader
	response += "<h1> pingmesh </h1>"
	response += "<p>Accessible URLs are:\n"
	response += routelist
	response += "<p>Served from " + s.myLoc + "\n"
	response += htmlTrailer

	return []byte(response)
}

func (s *meshSrv) pingResponse() []byte {
	response := htmlHeader
	response += "<h1> pingResponse </h1>"
	response += "<p>Served from " + s.myLoc + "\n"
	response += htmlTrailer

	return []byte(response)
}
