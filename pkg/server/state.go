package server

import (
	"sync"
	"time"
)

// define state types

type meshSrv struct {
	wg      *sync.WaitGroup
	done    chan int
	myLoc   string
	cwFlag  bool
	verbose int

	routes []route
	peers  []peer

	start time.Time // time we started the ping
}

var myServer *meshSrv

func init() {
	myServer = new(meshSrv)
	myServer.start = time.Now()
	myServer.SetupRoutes()
}

func PingmeshServer() *meshSrv {
	return myServer
}

func (s *meshSrv) SetupState(myLoc string, cwFlag bool, verbose int) {
	s.myLoc = myLoc
	s.cwFlag = cwFlag
	s.verbose = verbose
}

////
//  Server state information
////

// get state
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

// update state
func (s *meshSrv) SetWaitGroup(wg *sync.WaitGroup) {
	s.wg = wg
}

func (s *meshSrv) SetDoneChan(done chan int) {
	s.done = done
}

func (s *meshSrv) SetVerbose(verbose int) {
	s.verbose = verbose
}
