package server

import (
	"github.com/rafayopen/pingmesh/pkg/client" // fetchurl

	"github.com/rafayopen/perftest/pkg/cw" // cloudwatch integration
	"github.com/rafayopen/perftest/pkg/pt" // pingtimes but not fetchurl

	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	//"io"
	//	"io/ioutil"
	"log"
	"math"
	"math/rand"
	//	"net/http"
	//	"net/http/httptrace"
	"sync"
	"time"
)

////
//  peer holds information about an endpoint that we are trying to ping.  The
//  meshSrv instance referenced in peer holds the array of peer objects that are
//  currently active.  Members must be exported for JSON to dump them.
type peer struct {
	Url      string // endpoint to ping
	Host     string // hostname from Url
	Limit    int    // number of pings before exiting
	Delay    int    // delay between pings
	Maxfail  int    // max failures before exiting
	Location string // location of this peer
	PeerIP   string // peer's IP address (used for IP override)

	FirstPing  time.Time    // first ping request
	LatestPing time.Time    // most recent ping response
	Pings      int          // number of successful responses
	Fails      int          // number of ping failures seen
	PingTotals pt.PingTimes // aggregates ping time results

	ms *meshSrv   // point back to the server for receivers to access state
	mu sync.Mutex // make peer reentrant
}

////
//  Peer errors
type peersError struct {
	reason string
}

func (e *peersError) Error() string {
	return e.reason
}

var (
	PeerAlreadyPresent = errors.New("Peer already present in peers list")
)

////
//  Info returns a string with basic peer state
func (p *peer) Info() string {
	return fmt.Sprintf("%s delay %d (%d of %d success, %d of %d fails)\n",
		p.Url, p.Delay, p.Pings, p.Limit, p.Fails, p.Maxfail)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

////
//  Ping sends HTTP requests to the configured Url and captures detailed timing
//  information. It repeats the ping request after a delay (in time.Seconds).
func (p *peer) Ping() {
	// this task is recorded in the waitgroup, so clear waitgroup on return
	defer p.ms.Done()
	// This must come after Done and before Reporter (executes in reverse order)
	defer p.ms.Delete(p)

	if p.ms.Verbose() > 1 {
		log.Println("ping", p.Url)
	}

	maxfail := p.Maxfail // default before thread quits trying
	mn := "TCP RTT"      // CloudWatch metric name
	ns := "pingmesh"     // Cloudwatch namespace

	limit := p.Limit // number of pings before we quit, "forever" if zero
	if limit == 0 {
		limit = math.MaxInt32
	}
	if maxfail > limit {
		maxfail = limit
	}

	////
	//  Reporter summarizes ping statistics to stdout at the end of the run
	defer func() { // Reporter
		if p.Pings == 0 {
			fmt.Printf("\nRecorded 0 valid samples, %d of %d failures\n", p.Fails, maxfail)
			return
		}

		fc := float64(p.Pings)
		elapsed := Hhmmss_d(p.PingTotals.Start)

		fmt.Printf("\nRecorded %d samples in %s, average values:\n"+"%s"+
			"%d %-6s\t%.03f\t%.03f\t%.03f\t%.03f\t%.03f\t%.03f\t\t%d\t%s\t%s\n\n",
			p.Pings, elapsed, pt.PingTimesHeader(),
			p.Pings, elapsed,
			pt.Msec(p.PingTotals.DnsLk)/fc,
			pt.Msec(p.PingTotals.TcpHs)/fc,
			pt.Msec(p.PingTotals.TlsHs)/fc,
			pt.Msec(p.PingTotals.Reply)/fc,
			pt.Msec(p.PingTotals.Close)/fc,
			pt.Msec(p.PingTotals.RespTime())/fc,
			p.PingTotals.Size/int64(p.Pings),
			pt.LocationOrIp(p.PingTotals.Location),
			*p.PingTotals.DestUrl)
	}()

	p.FirstPing = time.Now().UTC().Truncate(time.Second)
	for {
		if p.ms.DoneChan() == nil {
			// channel is nil, reading from it will block, return
			if p.ms.Verbose() > 1 {
				log.Println("peer.Ping: channel is nil, returning")
			}
			return
		}
		////
		// Sleep first, allows risk-free continue from error cases below
		sleepTime := p.Delay
		if p.Pings == 0 {
			sleepTime = 1 // short sleep the first time around
		}

		select {
		case <-time.After(JitterPct(sleepTime, 1)):
			// we waited for the delay and got nothing ... loop around

		case newdelay, more := <-p.ms.DoneChan():
			if !more {
				// channel is closed, we are done -- goodbye
				if p.ms.Verbose() > 1 {
					log.Println("peer.Ping: channel is closed, returning")
				}
				return
			}
			// else we got a new delay on this channel (0 is signal to stop)
			p.Delay = newdelay
			if p.Delay <= 0 {
				return
			}
			// we did not (finish) our sleep in this case ...
		}

		////
		// Try to fetch the URL
		ptResult := client.FetchURL(p.Url, p.PeerIP)

		switch {
		// result nil, something totally failed
		case nil == ptResult:
			func() {
				p.mu.Lock()
				defer p.mu.Unlock()
				p.Fails++
			}()
			log.Println("fetch failure", p.Fails, "of", maxfail, "on", p.Url)
			if p.Fails >= maxfail {
				return
			}
			continue

		// HTTP 200 OK and 300 series "OK" status codes
		case ptResult.RespCode <= 304:
			// Take a write lock on this peer before updating values
			// (make each peer read/write reentrant, also []*peers)
			func() {
				p.mu.Lock()
				defer p.mu.Unlock()
				p.Pings++
				now := time.Now().UTC()
				p.LatestPing = now.UTC().Truncate(time.Second)
				if p.Pings == 1 {
					////
					// first ping -- initialize ptResult
					p.PingTotals = *ptResult
				} else {
					p.PingTotals.DnsLk += ptResult.DnsLk
					p.PingTotals.TcpHs += ptResult.TcpHs
					p.PingTotals.TlsHs += ptResult.TlsHs
					p.PingTotals.Reply += ptResult.Reply
					p.PingTotals.Close += ptResult.Close
					p.PingTotals.Total += ptResult.Total
					p.PingTotals.Size += ptResult.Size
				}

				if len(p.PeerIP) == 0 && len(ptResult.Remote) > 0 {
					p.PeerIP = ptResult.Remote
				}

				if p.Location == client.LocUnknown {
					if *ptResult.Location != client.LocUnknown && len(*ptResult.Location) > 0 {
						p.Location = *ptResult.Location
						p.PingTotals.Location = &p.Location
						if p.ms.Verbose() > 1 {
							log.Println("Initialize remote location to", *ptResult.Location)
						}
					} else {
						// It's not returning a pingmesh Location response, use hostname
						p.Location = p.Url
						p.PingTotals.Location = &p.Location
					}
				}
			}()

		// HTTP 500 series error
		case ptResult.RespCode > 304:
			func() {
				p.mu.Lock()
				defer p.mu.Unlock()
				p.Fails++
			}()
			log.Println("HTTP error", ptResult.RespCode, "failure", p.Fails, "of", maxfail, "on", p.Url)
			if p.ms.Verbose() > 0 {
				fmt.Println(p.Pings, ptResult.MsecTsv())
			}
			if p.Fails >= maxfail {
				return
			}
			continue

			////
			// Other HTTP response codes can be coded here (error, redirect)
			////
		}

		////
		//  Execution should continue here only in NON-ERROR cases; errors
		//  continue the for{} above
		////

		if p.ms.Verbose() > 0 {
			if p.ms.Verbose() > 1 {
				fmt.Println(p.Pings, ptResult.MsecTsv())
			} else {
				fmt.Printf("%3d %8.03f msec %20s %s\n", p.Pings, pt.Msec(ptResult.TcpHs), pt.LocationOrIp(ptResult.Location), ptResult.Remote)
			}
		}

		if p.ms.CwFlag() {
			metric := pt.Msec(ptResult.TcpHs)
			myLocation := p.ms.SrvLocation()
			if p.ms.Verbose() > 2 {
				log.Println("publishing TCP RTT", metric, "msec to CloudWatch", ns, "from", myLocation)
			}
			respCode := "0"
			if ptResult.RespCode >= 0 {
				// 000 in cloudwatch indicates it was a zero return code from lower layer
				// while single digit 0 indicates an error making the request
				respCode = fmt.Sprintf("%03d", ptResult.RespCode)
			}

			////
			// Publish my location (IP or REP_LOCATION) and their location
			cw.PublishRespTime(myLocation, p.Location, respCode, metric, mn, ns)
			// NOTE: using network RTT estimate (TcpHs) rather than full page response time
			// TODO: This makes the legends wrong in Cloudwatch.  Fix that.
		}

		if p.Pings >= limit {
			// report stats (see deferred func() above) upon return
			return
		}

		if p.Delay <= 0 {
			// we were signaled to stop
			return
		}
	}
}

////
//  AddPeersPeers attempts to add my peer's peers to my list.
//  The peer must be a pingmesh peer (must support /v1/peers).
//  The peer's peers will be looked up in the local list, and any that are
//  not present will be added to the list with Ping() goroutines started.
func (p *peer) AddPeersPeers() {
	defer p.ms.Done() // it's a goroutine

	if strings.Index(p.Url, getPeersUrl) < 0 {
		log.Println("Error: you can only addpeers with a", getPeersUrl, "request")
		return
	}

	////
	// Get remote meshping server publich state.  This may take a while!
	// That's why this is a goroutine...
	rm, err := FetchRemotePeer(p.Url, p.PeerIP)
	if err != nil {
		return // FetchRemotePeer reported to log(stderr) already
	}

	if p.ms.Verbose() > 2 {
		log.Println("Got a remote server's state:")
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		// No need to take a lock on the mutex
		if err := enc.Encode(rm); err != nil {
			log.Println("Error converting remote state to json:", err)
		}
	}

	for _, rmp := range rm.Peers {
		url := rmp.Url
		ip := rmp.PeerIP
		peer := p.ms.FindPeer(url, ip)
		if peer != nil {
			log.Println("peer", url, ip, "-- PeerAlreadyPresent")
			continue
		}
		log.Println("adding peer", url, ip)

		peer = p.ms.NewPeer(url, ip, p.Location)
		peer.ms.Add() // for the Ping goroutine
		go peer.Ping()
	}
}

////
//  JitterPct returns a millisecond time.Duration jittered by +/- pct, which
//  should be between 1 and 100.  The returned duration will never be negative.
func JitterPct(secs, pct int) time.Duration {
	if pct < 1 {
		pct = 1
	} else if pct > 200 { // prevents retval from going negative
		pct = 200
	}

	msec := float64(secs * 1000)
	jitter := (msec * float64(pct) / 100.0) * (rand.Float64() - 0.5)

	return time.Duration(msec+jitter) * time.Millisecond

}

//  Hhmmss returns a representation of the number of seconds (secs) like
//  1h15m22s (leaving off 0d, 0h or 0m prefix if they are zero).
func Hhmmss(secs int64) string {
	dd := secs / 86400
	secs -= dd * 86400
	hr := secs / 3600
	secs -= hr * 3600
	min := secs / 60
	secs -= min * 60

	if dd > 0 {
		return fmt.Sprintf("%dd%0dh%02dm%02ds", dd, hr, min, secs)
	}
	if hr > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", hr, min, secs)
	}
	if min > 0 {
		return fmt.Sprintf("%dm%02ds", min, secs)
	}
	return fmt.Sprintf("%ds", secs)
}

//  Hhmmss_d returns a representation of a duration in the format of Hhmmss
func Hhmmss_d(d time.Time) string {
	return Hhmmss(time.Now().Unix() - d.Unix())
}
