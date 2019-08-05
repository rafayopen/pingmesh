package server

import (
	"github.com/rafayopen/perftest/pkg/cw" // cloudwatch integration
	"github.com/rafayopen/perftest/pkg/pt" // pingtimes and fetchurl

	"fmt"
	"log"
	"math"
	"math/rand"
	"sync"
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

	Pings     int       // number of successful responses
	Fails     int       // number of ping failures seen
	Start     time.Time // time we started pinging this peer
	FirstPing time.Time // first recent ping response
	LastPing  time.Time // most recent ping response

	Summary pt.PingTimes // aggregates ping time results

	ms *meshSrv   // point back to the server for receivers to access state
	mu sync.Mutex // make peer reentrant
	// ping response fields go here ... TODO

	// auto-updated fields
}

////
//  Info returns a string with basic peer state
func (p *peer) Info() string {
	return fmt.Sprintf("%s delay %d (on %d of %d) started %v\n", p.Url, p.Delay, 0, p.Limit, p.Start)
}

func init() {
	rand.Seed(time.Now().UnixNano())
}

////
//  Ping sends HTTP requests to the configured Url and captures detailed timing
//  information. It repeats the ping request after a delay (in time.Seconds).
func (p *peer) Ping() {
	// this task is recorded in the waitgroup, so clear waitgroup on return
	defer p.ms.WaitGroup().Done()

	// This must come after Done and before Reporter (executes in reverse order)
	defer p.ms.Delete(p.Url)

	if p.ms.Verbose() > 1 {
		log.Println("ping", p.Url)
	}

	maxfail := 10    // max before thread quits trying
	mn := "TCP RTT"  // CloudWatch metric name
	ns := "pingmesh" // Cloudwatch namespace

	limit := p.Limit // number of pings before we quit, "forever" if zero
	if limit == 0 {
		limit = math.MaxInt32
	}

	////
	//  Reporter summarizes ping statistics to stdout at the end of the run
	defer func() { // Reporter
		if p.Pings == 0 {
			fmt.Printf("\nRecorded 0 valid samples, %d of %d failures\n", p.Fails, maxfail)
			return
		}

		fc := float64(p.Pings)
		elapsed := Hhmmss(time.Now().Unix() - p.Summary.Start.Unix())

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

	// TODO -- replace pt.FetchURL with a version that obeys the REST API design

	for {
		////
		// Sleep first, allows risk-free continue from error cases below
		select {
		case <-time.After(JitterPct(p.Delay, 10)):
			// we waited for the delay and got nothing ... loop around

		case newdelay, more := <-p.ms.DoneChan():
			if !more {
				// channel is closed, we are done -- goodbye
				return
			}
			// else we got a new delay on this channel
			p.Delay = newdelay
			// we did not (finish) our sleep in this case ...
		}

		////
		// Try to fetch the URL
		ptResult := pt.FetchURL(p.Url, p.Location)
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

		// HTTP 200 OK
		case ptResult.RespCode == 200:
			// Take a write lock on this peer before updating values, then
			// take a read lock on p in order to read/return its result
			// in handlers.go (i.e., make each peer reentrant, also peers)
			func() {
				p.mu.Lock()
				defer p.mu.Unlock()
				p.Pings++
				now := time.Now()
				p.LastPing = now
				if p.Pings == 1 {
					////
					// first ping -- initialize ptResult
					p.FirstPing = now
					p.Summary = *ptResult
				} else {
					p.Summary.DnsLk += ptResult.DnsLk
					p.Summary.TcpHs += ptResult.TcpHs
					p.Summary.TlsHs += ptResult.TlsHs
					p.Summary.Reply += ptResult.Reply
					p.Summary.Close += ptResult.Close
					p.Summary.Total += ptResult.Total
					p.Summary.Size += ptResult.Size
				}
			}()

		// HTTP 500 series error
		case ptResult.RespCode > 500:
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
			// Other HTTP response codes here (error, redirect)
			////
		}

		////
		//  Execution should continue here only in NON-ERROR cases; errors
		//  continue the for{} above
		////

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
//  JitterPct returns a millisecond time.Duration jittered by +/- pct, which
//  should be between 1 and 100.  The returned duration will never be negative.
func JitterPct(secs, pct int) time.Duration {
	if pct < 1 {
		pct = 1
	} else if pct > 200 {
		// prevents retval from going negative
		pct = 200
	}

	msec := float64(secs * 1000)
	jitter := (msec * float64(pct) / 100.0) * (rand.Float64() - 0.5)

	return time.Duration(msec+jitter) * time.Millisecond

}

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
