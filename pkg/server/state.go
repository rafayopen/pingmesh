package server

import (
	"sync"
)

// define state types

type meshSrv struct {
	wg      *sync.WaitGroup
	done    chan int
	myLoc   string
	cwFlag  bool
	verbose int

	routes []routes
	peers  []*peer
}

var myServer *meshSrv

func PingmeshServer() *meshSrv {
	if myServer == nil {
		myServer = new(meshSrv)
		myServer.SetupRoutes()
	}
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
