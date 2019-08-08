package server

import (
	"github.com/rafayopen/pingmesh/pkg/client" // fetchurl

	"encoding/json"
	"io/ioutil"
	"log"
	"net"
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
		{"/v1/addpeer", "add a ping peer (takes ip, port, hostname)", s.AddPingHandler},
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
		reply += `<p>Enter the hostname (for SNI and virtual hosting),
an optional IP address (default is to lookup the hostname),
and optional port number (default is 443).</p>
<form action="/v1/addpeer">
<br>Host:  <input type="text" name="host" value="pingmesh.run.rafay-edge.net">
<br>Port:  <input type="text" name="port" value="443">
<br>IP(*): <input type="text" name="ip" value="">
<p><input type="submit" value="Submit">
</form>` + htmlTrailer

		w.Write([]byte(reply))
		return
	}

	var ip, host, port string

	ips := qs["ip"]
	ports := qs["port"]
	hosts := qs["host"]

	if len(hosts) == 0 {
		aphError("Please include a valid hostname")
		return
	}
	host = hosts[0]
	if len(ips) == 0 {
		ips, _ = net.LookupHost(host)
		if len(ips) == 0 {
			aphError("Found zero IPs for host " + host)
			return
		}
	}
	ip = ips[0]

	// assert ip && host are set
	if len(ports) > 0 {
		port = ports[0]
	} else {
		port = "443"
	}

	sOrNo := ""
	if port == "443" || port == "8443" {
		sOrNo = "s"
	}
	url := "http" + sOrNo + "://" + host + ":" + port + "/v1/ping"
	if s.Verbose() > 1 {
		log.Println("Add peer host ip:port = "+host, ip+":"+port, "via", url)
	}

	if peer, err := AddPingTarget(url, ip, "undefined", 0, 10); err != nil {
		log.Println("error with peer", peer)
		reply += `<p>Peer was already in the peer list since ` + peer.Start.String() + `:
<br>Url: ` + peer.Url + `
<br>IP: ` + peer.PeerIP + `</p><p><a href="/v1/peers">Click here</a> for JSON peer list.`
	} else {
		reply += `<p>Added information about the following ping meer you entered:
<br>Host: ` + host + `
<br>IP: ` + ip + `
<br>Port: ` + port + `
</p>` + `
<p><a href="/v1/peers">Click here</a> for JSON peer list.`
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
	s.wg.Done()
}
