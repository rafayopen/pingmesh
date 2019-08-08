// server implements the web server
package server

import (
	"github.com/rafayopen/pingmesh/pkg/client" // fetchurl

	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
//  Server state data types
////////////////////////////////////////////////////////////////////////////////

////
//  meshSrv is the core mesh server data structure.  Request handlers and state
//  access and manipulation functions are receivers on meshSrv so they can use
//  private (protected, unexported) data members.
////
type meshSrv struct {
	Start time.Time // time we started the pingmesh server itself

	SrvLoc     string   // user-supplied location info for CW reporting
	SrvHost    string   // optional hostname
	SrvPort    int      // server port number as reported to peers
	listenPort int      // actual server listen port number (NOT in JSON)
	SrvIPs     []net.IP // public IPs from DNS if hostname is set

	Peers      []*peer // information about ping mesh peers (see peers.go)
	Requests   int     // how many API requests (or pings) I have served
	NumActive  int     // count of active peers
	NumDeleted int     // count of deleted peers

	NumTests  int // from main() command line args or env vars
	PingDelay int // from main() command line args or env vars

	wg      *sync.WaitGroup // ping and server threads share this wg
	mu      sync.Mutex      // make meshSrv reentrant (protect peers)
	done    chan int        // used to signal when threads should exit
	cwFlag  bool            // user flag controls writing to CloudWatch
	verbose int             // controls logging to stdout

	routes []route // HTTP request to handler function mapping (plus info)
}

var (
	srvServer *meshSrv  // srvServer is a singleton
	once      sync.Once // initialize it only once
)

////
//  NewPingmeshServer creates a new server instance (only once), assigns its
//  values from the parameters, sets up HTTP routes, and starts a web server
//  on the local host:port if configured.
func NewPingmeshServer(myLoc, hostname string, port, report int, cwFlag bool, numTests, pingDelay, verbose int) *meshSrv {
	if report == 0 {
		report = port
	}

	once.Do(func() {
		ms := &meshSrv{
			Start:      time.Now(),
			SrvLoc:     myLoc,
			SrvHost:    hostname,
			SrvIPs:     client.GetIPs(hostname),
			SrvPort:    report,
			listenPort: port,
			cwFlag:     cwFlag,
			NumTests:   numTests,
			PingDelay:  pingDelay,
			verbose:    verbose,
			wg:         new(sync.WaitGroup), // used by server and ping peers, controls exit from main()
			done:       make(chan int),      // signals goroutines to exit after signal caught in main()
		}

		////
		// Start server if a listen port has been configured
		if port > 0 {
			ms.wg.Add(1)
			go ms.startServer()
		}

		ms.SetupRoutes()
		srvServer = ms
	})
	return srvServer
}

////
//  PingmeshServer returns the (already existing) singleton pointer
func PingmeshServer() *meshSrv {
	return srvServer
}

// StartServer runs a web server to listen on the given port.  It never returns, so
// invoke it with go StartServer(yourPort, routes).  Handlers and application state are
// set up separately.
func (ms *meshSrv) startServer() error {
	max := 5 // 5 tries = 15 seconds (linear backoff -- 5th triangular number)

	addr := fmt.Sprintf(":%d", ms.listenPort)
	log.Println("starting meshSrv listening on port", ms.listenPort, "reporting on", ms.SrvPort)

	// The ListenAndServe call should not return.  If it does the address may be in use
	// from an instance that just exited; if so retry a few times below.
	err := http.ListenAndServe(addr, nil)

	tries := 0
	for tries < max {
		tries++
		// sleep a little while longer each time through the loop
		log.Println(err, "-- sleep number", tries)
		time.Sleep(time.Duration(tries) * time.Second)
		// now try again ... it may take a while for a previous instance to exit
		err = http.ListenAndServe(addr, nil)
	}

	return err
}

////////////////////////////////////////////////////////////////////////////////////////
//  Internal fetch methods
////////////////////////////////////////////////////////////////////////////////////////

const (
	getPeersUrl = "/v1/peers"
)

func fetchRemoteServer(url, ip) (rm *server, err error) {

	// need to move this from func (stack scope) to module scope?
	tr := &http.Transport{
		MaxIdleConns:          100,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Warning: skips CA checks, but ping doesn't care
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, getPeersUrl, nil)
	if err != nil {
		log.Println("fetchRemoteServer: NewRequest", err)
		return
	}

	req.Header.Set("User-Agent", "pingmesh-client")
	resp, err := client.Do(req)
	if resp != nil {
		// Close body if non-nil, whatever err says (even if err non-nil)
		defer resp.Body.Close() // after we read the resonse body
	}
	if err != nil {
		log.Println("fetchRemoteServer: client.request:", err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("fetchRemoteServer: ReadAll body:", err)
		return
	}

	err := json.Unmarshal(body, rm)
	if err != nil {
		log.PrintlnI("fetchRemoteServer: json.Unmarshal:", err)
		return
	}

	return // rm should be initialized by now
}
