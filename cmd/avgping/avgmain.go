// avgping reports a pingmesh server's peer list in JSON or a friendly format.
package main

import (
	"github.com/rafayopen/perftest/pkg/pt" // pingtimes and fetchurl
	"github.com/rafayopen/pingmesh/pkg/server"

	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
	"strings"
	"time"
)

const usage = `Usage: %s [flags]
This application will fetch the pingmesh peer list from the remote host
and report the average TCP RTT value for each one.  

Command line flags:
`

func printUsage() {
	fmt.Fprintf(os.Stderr, usage, os.Args[0])
	flag.PrintDefaults()
}

////
// main reads command line arguments then queries the pingmesh server
func main() {
	var (
		peerHost, peerIP string
		dumpJson         bool
		dumpDeleted      bool
	)

	flag.BoolVar(&dumpJson, "J", false, "dump output as the raw JSON object")

	flag.BoolVar(&dumpDeleted, "d", false, "included deleted peers in text output (JSON has DelPeers)")

	flag.StringVar(&peerHost, "H", "", "Hostname of a pingmesh peer (with optional :port suffix)")
	flag.StringVar(&peerIP, "I", "", "IP of a pingmesh peer (overrides DNS Hostname if set)")

	flag.Usage = printUsage
	flag.Parse()

	if len(peerHost) == 0 {
		printUsage()
		return
	}
	peerUrl := peerHost + "/v1/peers"

	rm, err := server.FetchRemotePeer(peerUrl, peerIP)
	if err != nil {
		if !strings.HasPrefix(peerUrl, "http") {
			fmt.Println("Trying with https:// protocol")
			peerUrl = "https://" + peerUrl
			rm, err = server.FetchRemotePeer(peerUrl, peerIP)
		}
		if err != nil {
			os.Exit(1) // FetchRemotePeer reported to log(stderr) already
		}
	}

	if dumpJson {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rm); err != nil {
			log.Fatal("failed to decode JSON")
		}
	} else {
		var override string
		if len(peerIP) > 0 {
			override = " at " + peerIP
		}
		fmt.Printf("%s %s%s has %d peers and started %s ago:\n",
			rm.SrvLoc, peerHost, override, len(rm.Peers), server.Hhmmss_d(rm.Start))
		if len(rm.Peers) > 0 {
			fmt.Printf("%20s\t%s\t%s\t%s\t%12s\t%s\t%12s\t%s\n",
				"Location", "Pings", "Fails", "Start Time", "Duration", "msecRTT", "totalMs", "PeerIP or URL")
		}
		sort.SliceStable(rm.Peers, func(i, j int) bool { return rm.Peers[i].Location < rm.Peers[j].Location })
		for _, p := range rm.Peers {
			var msecRTT, respTime float64
			start := p.FirstPing.Format(time.Stamp)[:12]
			duration := p.LatestPing.Sub(p.FirstPing).Nanoseconds() / 1e9
			if duration < 0 {
				duration = 0
			}

			if p.Pings > 0 {
				msecRTT = pt.Msec(p.PingTotals.TcpHs) / float64(p.Pings)
				respTime = pt.Msec(p.PingTotals.Total) / float64(p.Pings)
			}
			if len(p.PeerIP) == 0 {
				p.PeerIP = " unknown "
			}
			fmt.Printf("%20s\t%d\t%d\t%s\t%12v\t%.03f\t%12.03f\t%s\n",
				trimLoc(p.Location), p.Pings, p.Fails, start, server.Hhmmss(duration), msecRTT, respTime, p.PeerIP)
		}

		if dumpDeleted && len(rm.DelPeers) > 0 {
			fmt.Printf("%s %s%s has %d deleted peers:\n",
				rm.SrvLoc, peerHost, override, len(rm.DelPeers))
			fmt.Printf("%20s\t%s\t%s\t%s\t%12s\t%s\t%12s\t%s\n",
				"Location", "Pings", "Fails", "Start Time", "Duration", "msecRTT", "totalMs", "PeerIP or URL")

			for _, p := range rm.DelPeers {
				var msecRTT, respTime float64
				start := p.FirstPing.Format(time.Stamp)[:12]
				duration := p.LatestPing.Sub(p.FirstPing).Nanoseconds() / 1e9
				if duration < 0 {
					duration = 0
				}
				if p.Pings > 0 {
					msecRTT = pt.Msec(p.PingTotals.TcpHs) / float64(p.Pings)
					respTime = pt.Msec(p.PingTotals.Total) / float64(p.Pings)
				}
				if len(p.PeerIP) == 0 {
					p.PeerIP = " unknown "
				}
				fmt.Printf("%20s\t%d\t%d\t%s\t%12v\t%.03f\t%12.03f\t%s\n",
					trimLoc(p.Location), p.Pings, p.Fails, start, server.Hhmmss(duration), msecRTT, respTime, p.PeerIP)
			}
		}
	}

	return
}

func trimLoc(s string) string {
	pfx := []string{
		"https://",
		"http://",
	}
	for _, p := range pfx {
		if strings.HasPrefix(s, p) {
			return s[len(p):]
		}
	}
	return s
}
