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
func (ms *meshSrv) NewPeer(url, ip, location string) *peer {
	////
	//  ONLY create a NewPeer if you are planning to "go peer.Ping" right after!
	ms.wg.Add(1)
	// wg.Add needs to happen here, not in Ping() due to race condition: if we get
	// to wg.Wait() before goroutine has gotten scheduled we'll exit prematurely

	p := peer{
		Url:      url,
		PeerIP:   ip, // may be empty
		Limit:    ms.NumTests,
		Delay:    ms.PingDelay,
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

func (ms *meshSrv) FindPeer(url, ip string) *peer {
	ms.mu.Lock()
	defer ms.mu.Unlock()

	for _, p := range ms.Peers {
		// It's OK to ping the same URL (host) on multiple IPs
		if p.Url == url && p.PeerIP == ip {
			return p
		}
	}
	return nil
}

////
//  Delete removes all peers from the peer list matching url and ip.  The
//  caller (e.g., from Ping()) MUST follow Delete with WaitGroup.Done.
func (ms *meshSrv) Delete(p *peer) {
	ms.mu.Lock() // protect this whole dang func...
	defer ms.mu.Unlock()

	ms.NumActive--
	ms.NumDeleted++

	var newPeers []*peer // replacement peer array
	found := 0

	for _, plist := range ms.Peers {
		if plist.Url == p.Url && (len(p.PeerIP) == 0 || plist.PeerIP == p.PeerIP) {
			found++
		} else {
			newPeers = append(newPeers, plist)
		}
	}
	switch found {
	case 0:
		if ms.Verbose() > 0 {
			log.Println("Warning: failed to delete pinger for", p.Url, "on", p.PeerIP)
		}
		return
	case 1:
		if ms.Verbose() > 0 {
			log.Println("Deleted pinger for", p.Url)
		}
	default:
		if ms.Verbose() > 0 {
			log.Println("Note: deleted", found, "pingers for", p.Url, "on", p.PeerIP)
		}
	}
	ms.Peers = newPeers
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
