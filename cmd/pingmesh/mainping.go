// pingmesh is a mesh-based HTTP tester based upon rafayopen/perftest.
// This code base implements both ping and pong functions.  It collects
// L4 latency and L7 response time metrics.  See README and usage info.
//
// Measurement endpoints can be configured via command line arguments,
// environment variables, or HTTP requests to a running instance. Result
// data is written to stdout, and can be uploaded to CloudWatch
package main

import (
	//	"github.com/rafayopen/pingmesh/pkg/client"
	"github.com/rafayopen/pingmesh/pkg/server"

	"github.com/rafayopen/perftest/pkg/pt" // pingtimes and fetchurl

	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"
)

const usage = `Usage: %s [flags] endpoints...
endpoints: zero or more hostnames or IP addresses, they will be targets
of pinger client requests.  Repeats the request every $delay seconds.
If a port selected (-s servePort) then start a web server on that port.
If a pinger client fails enough times the process exits with an error.
You can interrupt it with ^C (SIGINT) or SIGTERM.

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
		numTests    int
		pingDelay   int
		servePort   int
		serveReport int
		myLocation  string
		myHost      string
		cwFlag      bool
		vf, qf      bool
		verbose     int = 1
	)

	flag.IntVar(&pingDelay, "d", 10, "delay in seconds between ping requests")
	flag.IntVar(&servePort, "s", 0, "server listen port; default zero means don't run a server")
	flag.IntVar(&serveReport, "r", 0, "server port to report as SrvPort (Rafay translates ports in edge)")
	flag.IntVar(&numTests, "n", 0, "number of tests to each endpoint (default 0 runs until interrupted)")
	flag.BoolVar(&cwFlag, "c", false, "publish metrics to CloudWatch")
	flag.BoolVar(&vf, "v", false, "be more verbose")
	flag.BoolVar(&qf, "q", false, "be less verbose")
	flag.StringVar(&myLocation, "I", "", "HTTP client's location to report")
	flag.StringVar(&myHost, "H", "", "My hostname (should resolve to accessible IPs)")

	flag.Usage = printUsage
	flag.Parse()

	if len(myLocation) == 0 {
		myLocation = pt.LocationFromEnv()
		myLocation = pt.LocationOrIp(&myLocation)
	}

	if vf {
		verbose += 1
	}
	if qf {
		verbose = 0
	}

	if cwFlag {
		cwRegion := os.Getenv("AWS_REGION")
		if len(cwRegion) > 0 && len(os.Getenv("AWS_ACCESS_KEY_ID")) > 0 && len(os.Getenv("AWS_SECRET_ACCESS_KEY")) > 0 {
			if verbose > 1 {
				log.Println("publishing to CloudWatch region", cwRegion)
			}
		} else {
			log.Println("CloudWatch requires in environment: AWS_REGION, AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY")
			cwFlag = false
		}
	}

	endpoints := flag.Args() // any remaining arguments are the endpoints to ping
	if len(endpoints) == 0 {
		if servePort == 0 {
			printUsage()
			return
		}
		fmt.Println("NOTE: not starting any pings, just serving")
	}

	hostEnv := os.Getenv("PINGMESH_HOSTNAME")
	if len(myHost) == 0 {
		myHost = hostEnv // if also be empty no DNS lookup is done
	}

	pm := server.NewPingmeshServer(myLocation, myHost, servePort, serveReport, cwFlag, verbose)

	if pm == nil {
		log.Println("error starting server")
		os.Exit(1)
	}

	////
	// Set up signal handler thread to close down Pinger goroutines gracefully
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	signal.Notify(sigchan, syscall.SIGTERM)
	go func() {
		for sig := range sigchan {
			if pm.DoneChan() != nil {
				fmt.Println("\nreceived", sig, "signal, terminating")
				pm.CloseDoneChan()
			} else {
				// something went wrong (should have exited already)
				fmt.Println("\nreceived", sig, "signal, hard exit")
				os.Exit(1)
			}
		}
		fmt.Println("close sigchan")
		close(sigchan)
	}()

	if len(endpoints) > 0 && verbose > 0 {
		if verbose > 1 {
			log.Println("starting ping across", endpoints)
		}
		pt.TextHeader(os.Stdout)
	}

	////
	// Start a Pinger for each endpoint on the command line
	for _, url := range endpoints {
		location := pm.SrvLocation()
		// TODO: take location out of argument (URL parameter)
		parts := strings.Split(url, "#")
		if len(parts) > 1 {
			location = parts[1]
		}
		server.AddPingTarget(url, location, numTests, pingDelay)
	}

	if verbose > 1 {
		log.Println("waiting for goroutines to exit")
	}

	pm.Wait()
	log.Println("all goroutines exited, returning from main")

	return
}
