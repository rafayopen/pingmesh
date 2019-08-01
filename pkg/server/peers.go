package server

import (
	"github.com/rafayopen/perftest/pkg/cw" // cloudwatch integration
	"github.com/rafayopen/perftest/pkg/pt" // pingtimes and fetchurl

	"fmt"
	"log"
	"math"
	"time"
)

////
//  peer holds information about an endpoint that we are trying to ping.  The
//  meshSrv instance referenced in peer holds the array of peer objects that are
//  currently active.  Members must be exported for JSON to dump them.
type peer struct {
	Url      string // endpoint to ping
	Limit    int    // number of pings before exiting
	Delay    int    // delay between pings
	Location string // location of this peer

	Pings    int       // number of successful responses
	Start    time.Time // time we started pinging this peer
	LastPing time.Time // most recent ping response

	Summary pt.PingTimes // aggregates ping time results

	ms *meshSrv // point back to the server for receivers to access state

	// ping response fields go here ... TODO

	// auto-updated fields
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
//  Info returns a string with basic peer state
func (p *peer) Info() string {
	return fmt.Sprintf("%s delay %d (on %d of %d) started %v\n", p.Url, p.Delay, 0, p.Limit, p.Start)
}

////
//  Ping sends HTTP requests to the configured Url and captures detailed timing
//  information. It repeats the ping request after a delay (in time.Seconds).
func (p *peer) Ping() {
	// this task is recorded in the waitgroup, so clear waitgroup on return
	defer p.ms.WaitGroup().Done()

	if p.ms.Verbose() > 1 {
		log.Println("ping", p.Url)
	}

	mn := "TCP RTT"  // CloudWatch metric name
	ns := "pingmesh" // Cloudwatch namespace

	limit := p.Limit
	if limit == 0 {
		limit = math.MaxInt32
	}

	failcount := 0
	maxfail := 10

	for {
		// TODO -- replace pt.FetchURL with a version that obeys the REST API design
		ptResult := pt.FetchURL(p.Url, p.Location)
		if nil == ptResult {
			failcount++
			log.Println("fetch failure", failcount, "of", maxfail, "on", p.Url)
			if failcount > maxfail {
				return
			}
		} else {
			if p.Pings == 0 {
				////
				// first ping -- initialize ptResult and set up deferred reporter
				p.Pings = 1
				p.LastPing = time.Now()
				p.Summary = *ptResult
				defer func() { // summary printer, runs upon return
					elapsed := Hhmmss(time.Now().Unix() - p.Summary.Start.Unix())

					fc := float64(p.Pings)
					fmt.Printf("\nRecorded %d samples in %s, average values:\n"+"%s"+
						"%d %-6s\t%.03f\t%.03f\t%.03f\t%.03f\t%.03f\t%.03f\t\t%d\t%s\t%s\n\n",
						p.Pings, elapsed, pt.PingTimesHeader(),
						p.Pings, elapsed,
						pt.Msec(p.Summary.DnsLk)/fc,
						pt.Msec(p.Summary.TcpHs)/fc,
						pt.Msec(p.Summary.TlsHs)/fc,
						pt.Msec(p.Summary.Reply)/fc,
						pt.Msec(p.Summary.Close)/fc,
						pt.Msec(p.Summary.RespTime())/fc,
						p.Summary.Size/int64(p.Pings),
						"",
						*p.Summary.DestUrl)
				}()
			} else {
				// TODO: take a write lock on p before this block updates
				// take a read lock on p in order to read/return its result
				p.Pings++
				p.LastPing = time.Now()

				p.Summary.DnsLk += ptResult.DnsLk
				p.Summary.TcpHs += ptResult.TcpHs
				p.Summary.TlsHs += ptResult.TlsHs
				p.Summary.Reply += ptResult.Reply
				p.Summary.Close += ptResult.Close
				p.Summary.Total += ptResult.Total
				p.Summary.Size += ptResult.Size
			}

			if p.ms.Verbose() > 0 {
				fmt.Println(p.Pings, ptResult.MsecTsv())
			}

			if p.ms.CwFlag() {
				metric := pt.Msec(ptResult.TcpHs)
				myLocation := p.ms.MyLocation()
				if p.ms.Verbose() > 1 {
					log.Println("publishing TCP RTT", metric, "msec to CloudWatch ", ns)
				}
				respCode := "0"
				if ptResult.RespCode >= 0 {
					// 000 in cloudwatch indicates it was a zero return code from lower layer
					// while single digit 0 indicates an error making the request
					respCode = fmt.Sprintf("%03d", ptResult.RespCode)
				}

				////
				// Publish my location (IP or REP_LOCATION) and their location (the URL for now)
				cw.PublishRespTime(myLocation, p.Url, respCode, metric, mn, ns)
				// NOTE: using network RTT estimate (TcpHs) rather than full page response time
			}
		}

		if p.Pings >= limit {
			// report stats (see deferred func() above) upon return
			return
		}

		select {
		case <-time.After(time.Duration(p.Delay) * time.Second):
			// we waited for the delay and got nothing ... loop around

		case newdelay, more := <-p.ms.DoneChan():
			if !more {
				// channel is closed, we are done -- goodbye
				return
			}
			// else we got a new delay on this channel
			p.Delay = newdelay
		}

		if p.Delay <= 0 {
			// we were signaled to stop
			return
		}
	}
}

////
//  Hhmmss returns a representation of the number of seconds (secs) like
//  01h15m22s (leaving off 00h and 00h00m if they are zero).
func Hhmmss(secs int64) string {
	hr := secs / 3600
	secs -= hr * 3600
	min := secs / 60
	secs -= min * 60

	if hr > 0 {
		return fmt.Sprintf("%dh%02dm%02ds", hr, min, secs)
	}
	if min > 0 {
		return fmt.Sprintf("%dm%02ds", min, secs)
	}
	return fmt.Sprintf("%ds", secs)
}
