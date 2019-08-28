package server

import (
	"github.com/rafayopen/pingmesh/pkg/client" // fetchurl

	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
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
		{"/v1/ping", "get a ping response", s.PingHandler},
		{"/v1/peers", "get a list of peers", s.PeersHandler},
		{"/v1/addpeer", "add a ping peer (takes ip, port, hostname)", s.AddPingHandler},
		{"/v1/metrics", "get memory statistics", s.MetricsHandler},
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
	s.Requests++
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
		response += client.ServedFromPrefix + s.SrvLoc + client.ServedFromSuffix
		response += htmlTrailer

		w.Write([]byte(response))

	default:
		reason := "Invalid request method: " + r.Method
		http.Error(w, reason, http.StatusMethodNotAllowed)
	}
}

func (s *meshSrv) MetricsHandler(w http.ResponseWriter, r *http.Request) {
	s.Requests++
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

////
//  AddPingHandler takes URI parameters as follows and attempts to add a new
//  peer pinging the target endpoint.
//  Parameters   (meaning of the parameter)        example value
//  - ip=        (the IP address connect to)       1.2.3.4
//  - port=      (the port number on that ip)      443
//  - hostname=  (the host header value to use)    pingmesh.run.rafay-edge.net
//
//  You can leave off ip to use whatever DNS lookup comes up with.  Port
//  defaults to 80 if you leave it off.
func (s *meshSrv) AddPingHandler(w http.ResponseWriter, r *http.Request) {
	s.Requests++

	reply := htmlHeader(s.SrvLoc) + `<h1>Add a pingmesh Peer</h1>`

	aphError := func(reason string) {
		if s.Verbose() > 0 {
			log.Println("AddPingHandler error:", reason)
		}
		reply += "<b>Error adding pingmesh peer:" + reason + "</b>" + htmlTrailer
		w.Write([]byte(reply))
		return
	}

	qs := r.URL.Query()
	if len(qs) == 0 {
		reply += `<p>Enter the URL to ping including :port if necessary.
You may specify an optional IP address (default is to lookup the hostname),
this will send the request to that IP address with the hostname in the URL.
<br>Example <em>https://www.google.com/</em> will collect perf data from google,
or <em>https://pingmesh.run.rafay-edge.net/v1/ping</em> to measure to a peer
(in this case you should use an IP override, else it will ping itself).</p>

<form action="/v1/addpeer">
<br>URL: <input type="text" name="url" value="https://rafay.co">
<br>IP(*): <input type="text" name="ip" value=""> (overrides DNS lookup if set)
<p><input type="submit" value="Submit">
</form>` + htmlTrailer

		w.Write([]byte(reply))
		return
	}

	urls := qs["url"]
	ips := qs["ip"]

	if len(urls) == 0 {
		aphError("Please include a valid URL")
		return
	}
	if len(urls) > 1 {
		reply += `<p><b>Warning: Only one URL accepted</b>, but ` + string(len(urls)) + `supplied</p>\n`
	}

	url := strings.Trim(urls[0], " \t")
	var addpeers bool
	if ap := strings.Index(url, "addpeers"); ap > 0 {
		addpeers = strings.Index(url[ap:], "true") > 0
	}
	if qi := strings.Index(url, "?"); qi > 0 {
		url = url[:qi] // trim query string from target URL
	}
	var ip, override string // optional IP override

	if len(ips) > 0 {
		ip = strings.Trim(ips[0], " \t")
		if len(ip) > 0 {
			override = " (with IP override " + ip + ")"
		}
		if len(ips) > 1 {
			reply += `<p><b>Warning: Only one IP override accepted</b>, but ` + string(len(ips)) + `supplied</p>\n`
		}
	}

	if peer, err := s.AddPingTarget(url, ip, client.LocUnknown); err != nil {
		log.Println("error adding peer", peer)
		if err == PeerAlreadyPresent {
			reply += `<p>Peer was already in the peer list since ` + peer.FirstPing.String() + `:
<br>Url: ` + peer.Url + `
<br>IP: ` + peer.PeerIP + `</p><p><a href="/v1/peers">Click here</a> for JSON peer list.`
		} else {
			reply += `<p>Unknown error: ` + err.Error()
		}
	} else { // err == nil, peer had better != nil
		log.Println("added peer", url+override)
		reply += `<p>Added a new peer for ` + url + override + `
<p><a href="/v1/peers">Click here</a> for JSON peer list.`
		////
		// Now see if we are supposed to add this peer's peers
		if addpeers {
			log.Println("starting thread to addpeers from", peer)
			peer.ms.Add()           // for the AddPeers goroutine
			go peer.AddPeersPeers() // must call Done()
		}
	}

	reply += htmlTrailer

	w.Write([]byte(reply))
	return
}

func (s *meshSrv) PeersHandler(w http.ResponseWriter, r *http.Request) {
	s.Requests++
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
	s.Requests++
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
		response += client.ServedFromPrefix + s.SrvLoc + client.ServedFromSuffix
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
	s.Requests++
	//	log.Println("EnvHandler called")

	response := htmlHeader(s.SrvLoc)
	response += "<h1> Runtime Envronment </h1>"
	response += client.ServedFromPrefix + s.SrvLoc + client.ServedFromSuffix + "<h2>Shell Environment</h2>\n<pre>\n"
	env := os.Environ()
	sort.Strings(env)
	for _, pair := range env {
		response += pair + "\n"
	}
	w.Write([]byte(response))

	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")

	response = "</pre>\n<h2> Server and Peer State </h2>\n<pre>\n"
	w.Write([]byte(response))

	func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if err := enc.Encode(s); err != nil {
			http.Error(w, "Error converting peer to json",
				http.StatusInternalServerError)
		}
	}()

	response = "</pre>\n<h2> Memory Stats </h2>\n<pre>\n"
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
	s.Requests++
	log.Println("QuitHandler called, shutting down server")

	response := htmlHeader(s.SrvLoc)
	response += "<h1> quitResponse </h1>"
	response += client.ServedFromPrefix + s.SrvLoc + client.ServedFromSuffix + "<p>shutting down with these peers:\n<pre>\n"
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
	s.CloseDoneChan()
}
