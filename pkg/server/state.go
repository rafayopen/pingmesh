package server

import (
	"log"
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
	wg      *sync.WaitGroup // ping and server threads share this wg
	mu      sync.Mutex      // make meshSrv reentrant (protect peers)
	done    chan int        // used to signal when threads should exit
	myLoc   string          // user-supplied location info for CW reporting
	cwFlag  bool            // user flag controls writing to CloudWatch
	verbose int             // controls logging to stdout

	routes []route   // HTTP request to handler function mapping (plus info)
	peers  []*peer   // information about ping mesh peers (see peers.go)
	start  time.Time // time we started the pingmesh server itself

}

////
//  myServer is a singleton instance of the server for the app
var myServer *meshSrv

////
//  init creates the myServer instances and sets up HTTP routes
func init() {
	myServer = new(meshSrv)
	myServer.start = time.Now()
	myServer.SetupRoutes()
}

////
//  PingmeshServer returns the (already existing) singleton pointer
func PingmeshServer() *meshSrv {
	return myServer
}

////
//  SetupState is called by main() to provide initial conditions for the
//  pingmesh instance
func (ms *meshSrv) SetupState(myLoc string, cwFlag bool, verbose int) {
	//	ms.mu.Lock()
	//	defer ms.mu.Unlock()
	// really don't need to lock here

	ms.myLoc = myLoc
	ms.cwFlag = cwFlag
	ms.verbose = verbose
}

////////////////////////////////////////////////////////////////////////////////
//  Peer manipulation receivers
////////////////////////////////////////////////////////////////////////////////

////
//  NewPeer creates a new peer and increments the server's WaitGroup by one
//  (this needs to happen before invoking the goroutine)
func (ms *meshSrv) NewPeer(url, location string, limit, delay int) *peer {
	////
	//  ONLY create a NewPeer if you are planning to call Ping right after!
	ms.WaitGroup().Add(1)
	// wg.Add needs to happen here, not in Ping() due to race condition: if we get
	// to wg.Wait() before goroutine has gotten scheduled we'll exit prematurely

	p := peer{
		Url:      url,
		Limit:    limit,
		Delay:    delay,
		Location: location,
		ms:       ms,
		Start:    time.Now(),
	}

	func() {
		ms.mu.Lock()
		defer ms.mu.Unlock()
		ms.peers = append(ms.peers, &p)
	}()

	return &p
}

////
//  DeletePeer removes a peer from the peer list.  The caller (likely
//  Ping() from a deferred func) will need to call WaitGroup.Done.
func (ms *meshSrv) Delete(peerUrl string) {
	ms.mu.Lock() // protect this whole dang func...
	defer ms.mu.Unlock()

	var peers []*peer // replacement peer array
	found := 0

	// TODO: (make reentrant)
	for _, p := range ms.peers {
		if p.Url != peerUrl {
			peers = append(peers, p)
		} else {
			found++
		}
	}
	switch found {
	case 0:
		log.Println("Warning: failed to delete pinger for", peerUrl)
		return
	case 1:
		if ms.Verbose() > 0 {
			log.Println("Deleted pinger for", peerUrl)
		}
	default:
		log.Println("Note: deleted", found, "pingers for", peerUrl)
	}
	ms.peers = peers
}

////////////////////////////////////////////////////////////////////////////////
//  Server state accessors, self-explanatory
////////////////////////////////////////////////////////////////////////////////

func (s *meshSrv) MyLocation() string {
	return s.myLoc
}

func (s *meshSrv) CwFlag() bool {
	return s.cwFlag
}

func (s *meshSrv) WaitGroup() *sync.WaitGroup {
	return s.wg
}

func (s *meshSrv) DoneChan() chan int {
	return s.done
}

func (s *meshSrv) Verbose() int {
	return s.verbose
}

////////////////////////////////////////////////////////////////////////////////
//  Server state mutators
////////////////////////////////////////////////////////////////////////////////

func (s *meshSrv) SetWaitGroup(wg *sync.WaitGroup) {
	s.wg = wg
}

func (s *meshSrv) SetDoneChan(done chan int) {
	s.done = done
}

func (s *meshSrv) SetVerbose(verbose int) {
	s.verbose = verbose
}
