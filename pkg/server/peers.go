package server

import (
	"github.com/rafayopen/perftest/pkg/cw" // cloudwatch integration
	"github.com/rafayopen/perftest/pkg/pt" // pingtimes and fetchurl

	"fmt"
	"log"
	"math"
	"time"
)

type peer struct {
	// configuration settings
	url      string
	numTries int
	delay    int
	ms       *meshSrv

	// auto-updated
	start    time.Time // time we started the ping
	lastPing time.Time // most recent ping response
}

////
//  NewPeer creates a new peer and increments the server's WaitGroup by one
//  (this needs to happen before invoking the goroutine)
func (ms *meshSrv) NewPeer(url string, numTries, delay int) *peer {
	////
	//  ONLY create a NewPeer if you are planning to call Ping right after!
	ms.WaitGroup().Add(1)
	// wg.Add needs to happen here, not in Ping() due to race condition: if we get
	// to wg.Wait() before goroutine has gotten scheduled we'll exit prematurely

	if numTries == 0 {
		numTries = math.MaxInt32
	}

	p := &peer{
		url:      url,
		numTries: numTries,
		delay:    delay,
		ms:       ms,
		start:    time.Now(),
	}

	ms.peers = append(ms.peers, p)
	if ms.Verbose() > 1 {
		log.Println("added peer", p.url, "total", len(ms.peers), "peers")
	}
	return p
}

func (p *peer) Info() string {
	return fmt.Sprintf("%s %d %d of %d started %v\n", p.url, p.delay, 0, p.numTries, p.start)
}

// PingPeer sends HTTP request(s) to the configured host:port/uri and captures detailed
// timing information. It repeats the ping request after a delay (in time.Seconds).
//
// PingPeer stores ping results in peer state, available via meshSrv to clients via the
// REST API.
func (p *peer) Ping() {
	//func Pinger(url string, numTries, delay int, done chan int, wg *sync.WaitGroup)
	// this task recorded in the waitgroup: clear waitgroup on return
	defer p.ms.WaitGroup().Done()

	if p.ms.Verbose() > 1 {
		log.Println("ping", p.url)
	}

	var count int64
	failcount := 0
	maxfail := 10
	var ptSummary pt.PingTimes // aggregates ping time results
	mn := "TCP RTT"            // CloudWatch metric name
	ns := "pingmesh"           // Cloudwatch namespace

	for {
		// TODO -- replace pt.FetchURL with a version that obeys the REST API design
		ptResult := pt.FetchURL(p.url, p.ms.MyLocation())
		if nil == ptResult {
			failcount++
			log.Println("fetch failure", failcount, "of", maxfail, "on", p.url)
			if failcount > maxfail {
				return
			}
		} else {
			if count == 0 {
				ptSummary = *ptResult
				defer func() { // summary printer, runs upon return
					elapsed := Hhmmss(time.Now().Unix() - ptSummary.Start.Unix())

					fc := float64(count) // count will be 1 by time this runs
					fmt.Printf("\nRecorded %d samples in %s, average values:\n"+"%s"+
						"%d %-6s\t%.03f\t%.03f\t%.03f\t%.03f\t%.03f\t%.03f\t\t%d\t%s\t%s\n\n",
						count, elapsed, pt.PingTimesHeader(),
						count, elapsed,
						pt.Msec(ptSummary.DnsLk)/fc,
						pt.Msec(ptSummary.TcpHs)/fc,
						pt.Msec(ptSummary.TlsHs)/fc,
						pt.Msec(ptSummary.Reply)/fc,
						pt.Msec(ptSummary.Close)/fc,
						pt.Msec(ptSummary.RespTime())/fc,
						ptSummary.Size/count,
						"",
						*ptSummary.DestUrl)
				}()
			} else {
				ptSummary.DnsLk += ptResult.DnsLk
				ptSummary.TcpHs += ptResult.TcpHs
				ptSummary.TlsHs += ptResult.TlsHs
				ptSummary.Reply += ptResult.Reply
				ptSummary.Close += ptResult.Close
				ptSummary.Total += ptResult.Total
				ptSummary.Size += ptResult.Size
			}
			count++

			if p.ms.Verbose() > 0 {
				fmt.Println(ptResult.MsecTsv())
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
				cw.PublishRespTime(myLocation, p.url, respCode, metric, mn, ns)
				// NOTE: using network RTT estimate (TcpHs) rather than full page response time
			}
		}

		if count >= int64(p.numTries) {
			// report stats (see deferred func() above) upon return
			return
		}

		select {
		case <-time.After(time.Duration(p.delay) * time.Second):
			// we waited for the delay and got nothing ... loop around

		case newdelay, more := <-p.ms.DoneChan():
			if !more {
				// channel is closed, we are done -- goodbye
				return
			}
			// else we got a new delay on this channel
			p.delay = newdelay
		}

		if p.delay <= 0 {
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
