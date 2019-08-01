package server

import (
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
func (s *meshSrv) SetupState(myLoc string, cwFlag bool, verbose int) {
	s.myLoc = myLoc
	s.cwFlag = cwFlag
	s.verbose = verbose
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
