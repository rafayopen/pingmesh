package server

import (
	"github.com/rafayopen/pingmesh/pkg/client" // ParseURL

	"log"
	"time"
)

////////////////////////////////////////////////////////////////////////////////
//  Peer manipulation receivers
////////////////////////////////////////////////////////////////////////////////

////
//  NewPeer creates a new peer object
func (ms *meshSrv) NewPeer(url, ip, location string) *peer {
	u := client.ParseURL(url)
	if u == nil {
		log.Println("NewPeer: cannot parse URL", url)
		return nil
	}

	host := u.Host

	////
	// Create a new peer with default limit, delay, and fails.
	// See override code in handlers.go:AddPingHandler
	p := peer{
		Url:      url,
		Host:     host,
		PeerIP:   ip, // may be empty
		Limit:    ms.numTests,
		Delay:    ms.pingDelay,
		Maxfail:  ms.maxFail,
		Location: location,
		ms:       ms,
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
//  AddPingTarget adds a ping target at the given url, in location loc.  It
//  picks up numTests and pingDelay from the pingmesh server instance.
func (ms *meshSrv) AddPingTarget(url, ip, loc string) (*peer, error) {
	peer := ms.FindPeer(url, ip)
	if peer != nil {
		return peer, PeerAlreadyPresent
	}

	// Create a new peer -- and increment the server's wait group
	peer = ms.NewPeer(url, ip, loc)
	ms.Add() // for the ping goroutine
	go peer.Ping()
	return peer, nil
}

func (ms *meshSrv) FindPeer(url, ip string) *peer {
	u := client.ParseURL(url)
	if u == nil {
		log.Println("FindPeer: cannot parse URL", url)
		return nil
	}

	host := u.Host

	ms.mu.Lock()
	defer ms.mu.Unlock()

	for _, p := range ms.Peers {
		// It's OK to ping the same URL (host) on multiple IPs
		if p.Host == host && p.PeerIP == ip {
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
	var delPeers []*peer
	found := 0

	for _, plist := range ms.Peers {
		if plist.Url == p.Url && (len(p.PeerIP) == 0 || plist.PeerIP == p.PeerIP) {
			found++
			// replace latest ping time with deletion time
			plist.LatestPing = time.Now().UTC().Truncate(time.Second)
			delPeers = append(delPeers, plist)
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
		if ms.Verbose() > 1 {
			log.Println("Deleted pinger for", p.Url, "on", p.PeerIP, "in", p.Location)
		}
	default:
		if ms.Verbose() > 1 {
			log.Println("Note: deleted", found, "pingers for", p.Url, "on", p.PeerIP)
		}
	}
	ms.Peers = newPeers
	ms.DelPeers = append(ms.DelPeers, delPeers...) // may get repeats
	if len(ms.DelPeers)+len(delPeers) > 100 {
		// keep only most recent 100 deleted peers
		ms.DelPeers = ms.DelPeers[len(ms.DelPeers)-100:]
	}
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

func (s *meshSrv) Add() {
	s.wg.Add(1)
}

func (s *meshSrv) Wait() {
	// TODO: also check done chan for nil?
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
