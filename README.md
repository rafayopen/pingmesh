# Rafay Pingmesh

This application builds on [Rafay perftest](https://github.com/rafayopen/perftest) to
measure and report the network performance and availability of a set of peers
set up in a mesh, all testing one another. The peers can publish measurements to
CloudWatch, as well as logging to stdout. Looking at the data you may notice
some unusual changes in network behavior based on direction and by time of day.
Tracking this data can help inform container placement or application routing
decisions.

The `pingmesh` uses code from the open source `perftest` application, plus AWS
SDK components. It's based on golang 1.12 with modules, so dependencies will be
pulled down automatically if you also have a modern version go.

## Description

The mesh requires two or more `pingmesh` applications that can connect to each
other via HTTP(S) over TCP/IP. They can run within private IP address space as
long as they have internal connectivity. You can identify them with either host
names or IP addresses as shown in examples below.

The app needs public IP connectivity to publish metrics to CloudWatch, if you
want to use that to view metrics. Of course you can alter the code to publish
anywhere you like...

I shared some more information, including a couple of performance graphs in a
[blog post on LinkedIn](https://www.linkedin.com/pulse/ping-mesh-john-dilley/)
if you want to check that out.

## Prerequisites

Pingmesh is written in [golang](https://golang.org/doc/), which you can install
using the instructions [here](https://golang.org/doc/install).

If you don't want to build from go source you can run a pre-built version I have
published to DockerHub (see "Run from Docker" below). In this case you will need
the [Docker environment](https://docs.docker.com/get-started/).

## Usage

The command's usage string below briefly explains the options. The examples
later show them in action.

    Usage: cmd/pingmesh/pingmesh [flags] endpoints...
    endpoints: zero or more hostnames or IP addresses, they will be targets
    of pinger client requests.  Repeats the request every $delay seconds.
    If a port selected (-s servePort) then start a web server on that port.
    If a pinger client fails enough times the process exits with an error.
    You can interrupt it with ^C (SIGINT) or SIGTERM.

    Command line flags:
      -H string
        	My hostname (should resolve to accessible IPs)
      -I string
        	remote peer IP address override
      -L string
        	HTTP client's location to report
      -c	publish metrics to CloudWatch
      -d int
        	delay in seconds between ping requests (default 10)
      -n int
        	number of tests to each endpoint (default 0 runs until interrupted)
      -q	be less verbose
      -r int
        	server port to report as SrvPort (Rafay translates ports in edge)
      -s int
        	server listen port; default zero means don't run a server
      -v	be more verbose

In addition, some options can be controlled via environment variables. This
makes it easier to deploy on the Rafay distributed computing platform, and
likely on other similar platforms.



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
asymmetric test, which is basically just `perftest`. See the perftest page
linked above for details of the output format.

# Use Case Examples #

First run one instance of `pingmesh` on your local laptop or a server, either as
a docker or standalone. The example shows running from a local executable while
you are in the base directory of pingmesh. I like the -v flag for verbose output
while I'm testing so you'll see that in most of my tests.

``` shell
make standalone
cmd/pingmesh/pingmesh -v -s 8080 -n=5 -d=2 "http://localhost:8080/v1/ping"
```

The above runs a server listening on port 8080 and pings itself five times, very
much like a `perftest` use case. You can add multiple hostnames, for example add
"www.google.com" to the end of the above and see what happens.

## Exercising the Server ##

Let's look at the server control options. First, fire up a bare server instance.

``` shell
make standalone
cmd/pingmesh/pingmesh -v -s 8080 -n=5 -d=2
```

The server is listening for requests via a web interface (rudimentary API). Here
are the requests you can make and what you should expect to see. You can make
these API requests with a web browser, with the `curl` utility, or from code if
you wish (like we do `pkg/server/server.go:fetchRemoteServer` to invoke our own
API to fetch a peer's peers).

Start with a web browser. Enter the address `localhost:8080/v1/` and hit RETURN.

**The Base Page** /v1 has links to the other interesting application pages:
  * get a ping response -- /v1/ping -- returns a short page with location in HTML
  * get a list of peers -- /v1/peers -- the endpoints that are being monitored
  * add a ping peer -- /v1/addpeer -- adds a peer to the monitored list
  * get memory statistics -- /v1/memstats -- see some stats about this server
  * shut down this pinger -- /v1/quit -- "does what it says on the tin"

The most interesting are `peers` and `addpeer`. When you first run the server
(without URLs as the example above) you'll see an empty page. Try running with
its own localhost (loopback) URL: then you'll see one peer when it starts up.

**Add Ping Targets** `/v1/addpeer` takes any URL, including the `/v1/ping`
endpoint of a `pingmesh` peer, or regular web URLs (as with `perftest`). For
example:

  * www.google.com -- a bare hostname will be pinged over HTTP by default
  * https://www.google.com -- specify the protocol to use HTTPS (TLS/SSL)
  * https://my.pingmesh.host.name/v1/ping -- send requests to a peer instance
    (that is just a made-up name, you will need your own). See Run on Rafay
    below for an easy way to run the docker on many distributed endpoints.

You can override the IP address for any of these hostnames if you want to ping a
specific instance, while still sending the correct host header. The host header
must be correct for services using TLS (SSL) Server Name Indication (SNI), which
is done for most interesting internet requests. Here's an example:

  * http://my.local.pingmesh.name:8080/v1/ping with IP of 127.0.0.1 --
    override the DNS lookup to send pings to your local instance

If you want your location to show up correctly be sure to set REP_LOCATION. I
use City,CC (where CC is the ISO country code).

## Adding Peers Peers

To build the mesh go to the `addpeers` form page and enter the URL of a
`pingmesh` peer, ending with `/v1/peers?addpeers=true` (add a query string flag
to the get peers request for the remote server). If it's a pingmesh instance you
may want to specify an IP override. Now the server will ask a peer for its
peers, and add them to its list. This is how we build the mesh.

We detect duplication of the hostname and IP address to prevent the obvious
explosive of entries. In the future we'll add a feature to automatically grow
the mesh, but this one requires extra care to avoid internet worm syndrome. So
keep watch for future versions.

-------------------------------------------------------------------------------

# Run on Rafay

To measure WAN performance you need to have multiple instances deployed to
diverse wide geographic and/or network locations. Distribution and operation of
containerized microservices like `pingmesh` used to be pretty hard, but we're
working to simplify the experience.

Rafay Systems delivers a SaaS-based platform that automates operations and
lifecycle management for containerized applications. For both enterprises and
service providers, Rafay's platform delivers unique dynamic application
placement and distribution capabilities, making it ideal for multi-region,
multi-cloud, hybrid, and edge/MEC operations.

Rafay provides a sandbox platform that's free to try, with Kubernetes clusters
running in diverse geographic and network locations. Let's give it a try! And
don't worry, you don't have to know anything about Kubernetes to use the
platform. If you do, you can access power user features.

## Creating a Rafay Workload

First, you build a docker container with `make docker`. You don't need to push
it to DockerHub, you will publish it to the Rafay registry via a CLI tool we
provide.

To run a workload on the Rafay Platform you will need to sign up for an account
(they are free) and then configure the workload to run on one or more edges.
Refer to the [perftest](https://github.com/rafayopen/perftest) README for how to
sign up, login to the Rafay admin console, initialize the Rafay CLI, and upload
your container.

### Configure Workload

Once logged in to the Admin Console go to the Workloads page and click New
Workload.  Enter a name, and description, like this:

| Field | Value |
|-------|-------|
| Name | my-pingmesh |
| Description | Performance testing application |

I am creating this in my organization's default namespace, JADe Systems.
Your org name will populate in the Namespace box, you don't need to do
anything with this for now.

The next screen lets you configure your container.  Click on New Container.

The Container Name is `pingmesh`.
Choose the image name and tag name from above.
This image does not use any ports, you can leave the default port 80 there.

Select a size and initial replica count.  The micro size is fine for this
app, and there should only be one replica.

Now select the Startup Configuration section.  This is where we'll specify
what test we are going to run and where to send the output.

Click Add Startup Configuration and select Environment Variable.  Fill in
with values like this, clicking Add Startup Configuration as needed:

| Name | Value | Comments |
|------|-------|----------|
| PINGMESH_HOSTNAME | The local hostname | Shared in /v1/peers as JSON as "SrvHost" |
| PINGMESH_URL | A pingmesh endpoint | In addition to args, can be used for "ping master" |
| REP_LOCATION | City,CC (ISO country code) | Sent to CloudWatch, in stdout, and JSON "SrvLoc" |
| AWS_REGION | your AWS preferred region | CloudWatch region |
| AWS_ACCESS_KEY_ID | your AWS access key id | CloudWatch credentials |
| AWS_SECRET_ACCESS_KEY | your AWS secret access key | CloudWatch credentials |
| PINGMESH_LIMIT | Number of tests | Overrides the -n option (env var has precedence) |
| PINGMESH_DELAY | Time between requests | Overrides the -d option (env has precedence) |

If you leave these marked Secure they will not appear in the UI and will be
transmitted securely to the Rafay platform.

Click "Save and Return to Container List".

You will see selections for log aggregation, shared volumes, and a key-value
store.  This application doesn't need any of that so let's move on ...

Click "Save and go to Policies".

### Publish the Workload

The next screen lets you choose how to upgrade the container when a new one
is published.  We support Recreate (the default) which kills the running
container and starts a new one.  This is fine for `pingmesh`, but for a
container with user interaction you may prefer a gentler upgrade policy.

Click "Save and go to Placement".

Now we choose where to run the application.  You'll see "Specific Locations"
in the dropdown and a list of cities to choose from.  We also support a
"Performance" policy if your container takes user requests: we can place it
near the user base for that workload, automatically adjusting placement as
the situation changes.  But for this workload we just choose one location
and try it from there.

Click "Save and go to Publish" for the final step.

Click "Publish". You will see a pin bouncing at the location you specified. It
will turn green in a minute or so if deployment succeeds.

When you're done, click "Unpublish" to stop the workload running.

Or you can change the runtime configuration settings (environment variables)
if you want to test a different target.  Or change Placement if you want to
test from a different location.  Then go back to the Publish page and click
"Republish".

If you change the application code or Dockerfile you should update VERSION
in the Makefile, rebuild the docker, and upload it again with `rafay-cli`
using the new version tag, instead of ":v2" as specified above.

You can find more detailed configuration instructions in our [Quick Start
guide](https://rafay.zendesk.com/hc/en-us/articles/360007054432-Quick-Start-Guide-Custom-Image-)
for this and other scenarios.  You'll need Rafay credentials to pull up
zendesk content at this time.


## Debugging Workload Issues

If the workload fails to place, try running it locally, for example
(assuming you are now at v3):

``` shell
docker run pingmesh:v3 -v -n 5 -d 1 https://www.google.com/
```

If it runs OK locally, click Debug and then "Show Container Events" to see
what happened we tried to load the container.  And look at the logs to see
if the application encountered an issue.  You can also get a shell to try
out the application locally.  (The shell does not work in firefox, so use
chrome or safari for this feature.)

## Looking at Memory Utilization

I was curious how much memory I was using in my workload, and if I had a
memory leak.  Debugging this in a docker is not so easy (especially a
minimal one), and doing so in multiple locations is even harder.   So I
added a simple web server function to my application to return memory usage
information using the golang `runtime.MemStats` feature.

Docker version v5 includes this new flag.  Here's an example, using a
not-so-random port, testing against google every 30 seconds:

``` shell
docker run pingmesh:v5 -d 30 -s 52378 https://www.google.com/
```

View memory stats by pointing your browser to `localhost:52378/memstats`.

## Viewing Workload Results

There are a couple ways to see what is happening.

First, click "Debug".  You will see the stdout of the container, which looks
similar to what you saw running it locally.  You can save the stdout log
using the download link (arrow icon).

If you configured the AWS environment you can also view it in your AWS
CloudWatch instance.
  * Login to [your AWS console](https://console.aws.amazon.com) (that
    corresponds to AWS credentials you entered into the Rafay console).
  * Navigate to CloudWatch Metrics, select all metrics, and watch the data
    roll in.

You'll note that the Rafay workload picks up a location label automatically
from the environment: we put REP_LOCATION, and a few other items, into the
shell environment of every container.  You can see them by running a
go-httpbin testapp we also provide (at the /env/ endpoint).

I shared some more information, including a couple of performance graphs in a
[blog post on LinkedIn](https://www.linkedin.com/pulse/ping-mesh-john-dilley/).
