// avgping reports a pingmesh server's peer list in JSON or a friendly format.
package main

import (
	"github.com/rafayopen/pingmesh/pkg/server"

	"github.com/rafayopen/perftest/pkg/pt" // pingtimes and fetchurl

	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"sort"
)

const usage = `Usage: %s [flags]

Command line flags:
`

func printUsage() {
	fmt.Fprintf(os.Stderr, usage, os.Args[0])
	flag.PrintDefaults()
}

////
// main reads command line arguments, sets up signal handler, then
// starts web server and endpoint pingers
func main() {
	////
	//  flags
	var (
		peerHost           string
		peerIP             string
		dumpText, dumpJson bool
	)

	flag.BoolVar(&dumpText, "T", true, "dump text output")
	flag.BoolVar(&dumpJson, "J", false, "dump JSON output")

	flag.StringVar(&peerHost, "H", "", "Hostname of a pingmesh peer")
	flag.StringVar(&peerIP, "I", "", "IP of a pingmesh peer (overrides DNS Hostname if set)")

	flag.Usage = printUsage
	flag.Parse()

	if len(peerHost) == 0 {
		printUsage()
		return
	}
	peerUrl := "https://" + peerHost + "/v1/peers"

	rm, err := server.FetchRemotePeer(peerUrl, peerIP)
	if err != nil {
		return // FetchRemotePeer reported to log(stderr) already
	}

	if dumpText {
		var override string
		if len(peerIP) > 0 {
			override = " at " + peerIP
		}
		fmt.Printf("%s %s%s has %d peers and started %s ago:\n",
			rm.SrvLoc, peerHost, override, len(rm.Peers), server.Hhmmss_d(rm.Start))
		if len(rm.Peers) > 0 {
			fmt.Printf("%20s\t%s\t%s\t%s\t%12s\t%s\t%s\n",
				"Location", "    PeerIP", "Pings", "Fails", "Running", "msecRTT", "respTime")
		}
		sort.SliceStable(rm.Peers, func(i, j int) bool { return rm.Peers[i].Location < rm.Peers[j].Location })
		for _, p := range rm.Peers {
			var msecRTT, respTime float64
			if p.Pings > 0 {
				msecRTT = pt.Msec(p.PingTotals.TcpHs) / float64(p.Pings)
				respTime = pt.Msec(p.PingTotals.Total) / float64(p.Pings)
			}
			fmt.Printf("%20s\t%s\t%d\t%d\t%12v\t%.03f\t%.03f\n",
				p.Location, p.PeerIP, p.Pings, p.Fails, server.Hhmmss_d(p.Start), msecRTT, respTime)
		}
	}
	if dumpJson {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rm); err != nil {
			log.Fatal("failed to decode JSON")
		}
	}

	return
}
