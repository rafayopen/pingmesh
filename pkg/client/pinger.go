// client implements the pingmesh request generator
package client

import (
	"github.com/rafayopen/pingmesh/pkg/handlers"

	"github.com/rafayopen/perftest/pkg/pt" // pingtimes and fetchurl

	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

var verbose = 3 // for now

// Pinger sends HTTP request(s) to the given host:port/uri and captures detailed timing
// information. It repeats the ping request after a delay (in time.Seconds).  If delay
// is zero Pinger will just ping once and return.
//
// Pinger holds ping results in memory, available to peers and other clients via the
// REST API.
//
// The url parameter should be a fully qualified URL.  The done chan allows the parent
// to shut down the goroutine by closing the channel, or to adjust the delay parameter
// by sending an int.
func Pinger(url string, numTries, delay int, done chan int, wg *sync.WaitGroup) {
	// this task recorded in the waitgroup: clear waitgroup on return
	defer wg.Done()
	if numTries == 0 {
		numTries = math.MaxInt32
	}

	if verbose > 1 {
		log.Println("ping", url)
	}

	var count int64
	failcount := 0
	maxfail := 10
	var ptSummary pt.PingTimes // aggregates ping time results
	for {
		// TODO -- replace pt.FetchURL with a version that obeys the REST API design
		ptResult := pt.FetchURL(url, handlers.GetLocation())
		if nil == ptResult {
			failcount++
			log.Println("fetch failure", failcount, "of", maxfail, "on", url)
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

			if verbose > 0 {
				fmt.Println(ptResult.MsecTsv())
			}

			// if *cwFlag {
			// 	if verbose > 1 {
			// 		log.Println("publishing", pt.Msec(ptResult.RespTime()), "msec to cloudwatch")
			// TODO -- use network RTT estimate (TcpHs) rather than full page response time
			// 	}
			// 	respCode := "0"
			// 	if ptResult.RespCode >= 0 {
			// 		// 000 in cloudwatch indicates it was a zero return code from lower layer
			// 		// while single digit 0 indicates an error making the request
			// 		respCode = fmt.Sprintf("%03d", ptResult.RespCode)
			// 	}

			// 	cw.PublishRespTime(myLocation, urlStr, respCode, pt.Msec(ptResult.RespTime()))
			// }
		}

		if count >= int64(numTries) {
			// report stats (see deferred func() above) upon return
			return
		}

		select {
		case <-time.After(time.Duration(delay) * time.Second):
			// we waited for the delay and got nothing ... loop around

		case newdelay, more := <-done:
			if !more {
				// channel is closed, we are done -- goodbye
				return
			}
			// else we got a new delay on this channel
			delay = newdelay
		}

		if delay <= 0 {
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
