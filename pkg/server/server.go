// server implements the web server
package server

import (
	"fmt"
	"log"
	"net/http"
	"time"
)

// StartServer runs a web server to listen on the given port.  It never returns, so
// invoke it with go StartServer(yourPort, routes).  Handlers and application state are
// set up separately.
func (s *meshSrv) StartServer(port int) error {
	max := 5 // 5 tries = 15 seconds (linear backoff -- 5th triangular number)

	addr := fmt.Sprintf(":%d", port)
	log.Println("starting meshSrv listening on port", port)

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

	ms.peers = append(ms.peers, &p)

	return &p
}

////
//  DeletePeer removes a peer from the peer list.  The caller (likely
//  Ping() from a deferred func) will need to call WaitGroup.Done.
func (ms *meshSrv) Delete(peerUrl string) {
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
