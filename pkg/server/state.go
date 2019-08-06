package server

import (
	"log"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
//  Peer manipulation receivers
////////////////////////////////////////////////////////////////////////////////

////
//  NewPeer creates a new peer and increments the server's WaitGroup by one
//  (this needs to happen before invoking the goroutine)
func (ms *meshSrv) NewPeer(url, location string, limit, delay int) *peer {
	////
	//  ONLY create a NewPeer if you are planning to "go peer.Ping" right after!
	ms.wg.Add(1)
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
		ms.Peers = append(ms.Peers, &p)
		ms.NumActive++
	}()

	return &p
}

////
//  DeletePeer removes a peer from the peer list.  The caller (likely
//  Ping() from a deferred func) will need to call WaitGroup.Done.
func (ms *meshSrv) Delete(peerUrl string) {
	ms.mu.Lock() // protect this whole dang func...
	defer ms.mu.Unlock()

	ms.NumActive--
	ms.NumDeleted++

	var peers []*peer // replacement peer array
	found := 0

	// TODO: (make reentrant)
	for _, p := range ms.Peers {
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
	ms.Peers = peers
}

////////////////////////////////////////////////////////////////////////////////
//  Server state accessors, self-explanatory
////////////////////////////////////////////////////////////////////////////////

func (s *meshSrv) SrvLocation() string {
	return s.SrvLoc
}

func (s *meshSrv) CwFlag() bool {
	return s.cwFlag
}

func (s *meshSrv) Wait() {
	s.wg.Wait()
}

func (s *meshSrv) Done() {
	s.wg.Done()
}

func (s *meshSrv) DoneChan() chan int {
	return s.done
}

func (s *meshSrv) Verbose() int {
	return s.verbose
}

////
// Close the wg DoneChan and set it to nil
func (s *meshSrv) CloseDoneChan() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.done != nil {
		close(s.done)
		s.done = nil
	}
}
