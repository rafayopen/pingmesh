# Rafay Pingmesh

This application uses [Rafay perftest](https://github.com/rafayopen/perftest) to
measure and report the network performance and availability of a set of peers
set up in a mesh, all testing one another. The peers can publish measurements to
CloudWatch where you may notice some unusual changes in network behavior based
on direction and by time of day.

It depends upon open source `perftest` and AWS SDK components, which are pulled
down automatically if you have modern go (1.12) with support for go modules.

## Description

The mesh requires two or more `pingmesh` applications that know about and can
connec to each other via HTTPS over TCP/IP. They can run within private IP
address space as long as they have internal connectivity. The app needs public
IP connectivity to publish to CloudWatch, but you could of course alter that.

## Prerequisites

Pingmesh is written in [golang](https://golang.org/doc/), you can install it
using instructions [here](https://golang.org/doc/install). 

If you don't want to build from source you can run a pre-built version I have
published to DockerHub (see "Run from Docker" below). In this case you will need
the [Docker environment](https://docs.docker.com/get-started/).

## How to Build and Run

You can build either a standalone image, which can run on your local system and
others like it, or a docker image, which runs on any docker environment. To get
started:

  * Clone this repo.
  * Use `make standalone` to build a local standalone app, and `make test` to
    run the local instance with some default parameters so you an try it out.
  * Use `make docker` to build a docker image, and `make push` to push the
    container to DockerHub using credentials from your environment.
  
Once you've built the app you can run it either standalone or as a docker. It's
most useful if you have multiple instances that know about each other, so find a
couple of systems, perhaps running in Kubernetes clusters. You can also run an
asymmetric test, which is basically just `perftest`.

**Standalone**: To run a test from the command line try something like the
following after `make stadalone` from the `pingmesh` base directory. It will
listen on a local port in one thread and ping itself from another thread.
Terminate the application with control-c (interrupt) from the terminal.

    $ cmd/pingmesh/pingmesh -s 8080 -d 5 http://localhost:8080/v1/ping
    2019/07/29 09:48:28 starting server on port 8080
    2019/07/29 09:48:28 ping http://localhost:8080/v1/ping
    1564408108	2.825	1.608	0.000	0.414	0.073	2.179	200	116	192.168.1.10	[::1]	http://localhost:8080/v1/ping
    1564408113	1.280	0.468	0.000	0.433	0.129	1.090	200	116	192.168.1.10	[::1]	http://localhost:8080/v1/ping
    1564408118	1.264	0.458	0.000	0.412	0.107	1.034	200	116	192.168.1.10	[::1]	http://localhost:8080/v1/ping
    1564408123	1.758	0.450	0.000	0.426	0.107	1.065	200	116	192.168.1.10	[::1]	http://localhost:8080/v1/ping
      C-c 
    received interrupt signal, terminating
    
    Recorded 4 samples in 16s, average values:
    # timestamp	DNS	TCP	TLS	First	LastB	Total	HTTP	Size	From_Location	Remote_Addr	proto://uri
    4 16s   	1.782	0.746	0.000	0.421	0.104	1.342		116		http://localhost:8080/v1/ping
    
    2019/07/29 09:48:44 all goroutines exited, returning from main

