// server implements the web server
package server

import (
	"github.com/rafayopen/pingmesh/pkg/client" // fetchurl

	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
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

	SrvLoc     string // user-supplied location info for CW reporting
	SrvHost    string // optional hostname
	SrvPort    int    // server port number as reported to peers
	listenPort int    // actual server listen port number (NOT in JSON)

	Peers      []*peer // information about ping mesh peers (see peers.go)
	Requests   int     // how many API requests (or pings) I have served
	NumActive  int     // count of active peers
	NumDeleted int     // count of deleted peers
	DelPeers   []*peer // list of last 100 deleted peers

	numTests  int // from main() command line args or env vars, server default
	pingDelay int // from main() server default ping delay
	maxFail   int // from main() server default max failures before exiting

	wg      *sync.WaitGroup // ping and server threads share this wg
	mu      sync.Mutex      // make meshSrv reentrant (protect peers)
	done    chan int        // used to signal when threads should exit
	cwFlag  bool            // user flag controls writing to CloudWatch
	verbose int             // controls logging to stdout

	routes []route // HTTP request to handler function mapping (plus info)
}

var (
	srvServer  *meshSrv  // srvServer is a singleton
	once       sync.Once // initialize it only once
	httpServer *http.Server
)

////
//  NewPingmeshServer creates a new server instance (only once), assigns its
//  values from the parameters, sets up HTTP routes, and starts a web server
//  on the local host:port if configured.
func NewPingmeshServer(myLoc, hostname string, port, report int, cwFlag bool, numTests, pingDelay, maxFail, verbose int) *meshSrv {
	if report == 0 {
		report = port
	}

	once.Do(func() {
		ms := &meshSrv{
			Start:      time.Now().UTC().Truncate(time.Second),
			SrvLoc:     myLoc,
			SrvHost:    hostname,
			SrvPort:    report,
			listenPort: port,
			cwFlag:     cwFlag,
			numTests:   numTests,
			pingDelay:  pingDelay,
			maxFail:    maxFail,
			verbose:    verbose,
			wg:         new(sync.WaitGroup), // used by server and ping peers, controls exit from main()
			done:       make(chan int),      // signals goroutines to exit after signal caught in main()
		}

		////
		// Start server if a listen port has been configured
		if port > 0 {
			ms.SetupRoutes()
			go ms.startServer()
		}

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
	if ms.verbose > 1 {
		log.Println("starting meshSrv listening on port", ms.listenPort, "reporting on", ms.SrvPort)
	}

	// The ListenAndServe call should not return.  If it does the address may be in use
	// from an instance that just exited; if so retry a few times below.
	httpServer = &http.Server{Addr: addr, Handler: nil}
	err := httpServer.ListenAndServe()
	if err == http.ErrServerClosed {
		return nil
	}

	tries := 0
	for tries < max {
		tries++
		// sleep a little while longer each time through the loop
		log.Println(err, "-- sleep number", tries)
		time.Sleep(time.Duration(tries) * time.Second)
		// now try again ... it may take a while for a previous instance to exit
		err = httpServer.ListenAndServe()
		if err == http.ErrServerClosed {
			return nil
		}
	}

	return err
}

func (ms *meshSrv) Shutdown() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Println("server.Shutdown:", err)
	}
}

////////////////////////////////////////////////////////////////////////////////////////
//  Internal fetch methods
////////////////////////////////////////////////////////////////////////////////////////

const (
	getPeersUrl = "/v1/peers"
)

func FetchRemotePeer(rawurl, ip string) (rm *meshSrv, err error) {
	url := client.ParseURL(rawurl)
	if url == nil {
		log.Println("cannot parse URL", rawurl)
		return nil, errors.New("FetchRemotePeer: Bad URL")
	}

	host, peerAddr := client.MakePeerAddr(url.Scheme, url.Host, ip)
	urlStr := url.Scheme + "://" + host + url.Path

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}

	tr := &http.Transport{
		MaxIdleConns:          100,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, peerAddr)
		},
	}

	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	req, err := http.NewRequest(http.MethodGet, urlStr, nil)
	if err != nil {
		log.Println("FetchRemotePeer: NewRequest", err)
		return
	}

	req.Header.Set("User-Agent", "pingmesh-client")
	resp, err := client.Do(req)
	if resp != nil {
		// Close body if non-nil, whatever err says (even if err non-nil)
		defer resp.Body.Close() // after we read the resonse body
	}
	if err != nil {
		log.Println("FetchRemotePeer: client.request", urlStr, "on", ip, err)
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Println("FetchRemotePeer: ReadAll body:", err)
		return
	}

	rm = new(meshSrv)
	err = json.Unmarshal(body, rm)
	if err != nil {
		log.Println("FetchRemotePeer: json.Unmarshal:", err)
		if rm.verbose > 2 {
			log.Println("body was:", string(body))
		}
		return
	}

	return // rm should be initialized by now
}
