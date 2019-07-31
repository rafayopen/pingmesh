// server implements the web server
package server

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

type server struct {
	myLoc  string
	cwFlag bool

	routes []Routes
}

var myServer server

func NewServer() *server {
	s := new(server)
	s.SetupRoutes()
	return s
}

func (s *server) SetupState(myLoc string, cwFlag bool) {
	s.myLoc = myLoc
	s.cwFlag = cwFlag
}

// StartServer runs a web server to listen on the given port.  It never returns, so
// invoke it with go StartServer(yourPort, routes).  Handlers and application state are
// set up separately.
func (s *server) StartServer(port int) error {
	max := 5 // 5 tries = 15 seconds (linear backoff -- 5th triangular number)

	addr := fmt.Sprintf(":%d", port)
	log.Println("starting server on port", port)

	// The ListenAndServe call should not return.  If it does the address may be in use
	// from an instance that just exited; if so retry a few times below.
	err := http.ListenAndServe(addr, nil)

	tries := 0
	for tries < max {
		tries++
		// sleep a little while longer each time through the loop
		log.Println(err, "sleep", tries)
		time.Sleep(time.Duration(tries) * time.Second)
		// now try again ... it may take a while for a previous instance to exit
		err = http.ListenAndServe(addr, nil)
	}

	return err
}
