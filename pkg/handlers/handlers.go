// handlers implements HTTP respond management for the server
package handlers

import (
	"encoding/json"
	"io/ioutil"
	"net/http"
)

type state struct {
	myLoc string
}

var appState state // this is a hack for now (v1)

func SetupState(myLoc string) error {
	appState.myLoc = myLoc
	return nil
}

func GetLocation() string {
	return appState.myLoc
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		// return default pages with links to other API endpoints
		w.Write(rootResponse())

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

func MetricsHandler(w http.ResponseWriter, r *http.Request) {
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

func PeersHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "POST":
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "Error reading request body",
				http.StatusInternalServerError)
		}

		// decode body into JSON ... if propertly formatted
		_ = body

		// handle incoming peer

		// write response
		w.Write([]byte("PeersHandler POST not implemented\n"))

	case "GET":
		// write response
		w.Write([]byte("PeersHandler GET not implemented\n"))

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

func PingHandler(w http.ResponseWriter, r *http.Request) {
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
		w.Write(pingResponse())

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

func init() {
	routelist = "<ul>\n"
	for _, route := range Routes {
		routelist += bullet(route.uri, route.doc)
	}
	routelist += "</ul>\n"

}

func bullet(url, text string) string {
	return "<li><a href=\"" + url + "\">" + text + "</a></li>\n"
}

func rootResponse() []byte {
	response := htmlHeader
	response += "<h1> pingmesh </h1>"
	response += "<p>Accessible URLs are:\n"
	response += routelist
	response += "<p>Served from " + appState.myLoc + "\n"
	response += htmlTrailer

	return []byte(response)
}

func pingResponse() []byte {
	response := htmlHeader
	response += "<h1> pingResponse </h1>"
	response += "<p>Served from " + appState.myLoc + "\n"
	response += htmlTrailer

	return []byte(response)
}
