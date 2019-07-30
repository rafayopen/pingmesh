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
connect to each other via HTTPS over TCP/IP. They can run within private IP
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
following after `make standalone` from the `pingmesh` base directory. It will
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

    
Each line has an epoch timestamp when the ping started, and the time in
milliseconds measured for the following actions, from `perftest`:
  * DNS: how long to look up the IP address(es) for the hostname
  * TCP: how long the TCP three-way handshake took to set up the connection
  * TLS: how long the SSL/TLS handshake took to establish a secure channel
  * First: how long until the first byte of the reply arrived (HTTP response headers)
  * LastB: how long until the last byte of the reply arrived (HTTP content body)
  * Total: response time of the application, from start of TCP connection until last byte
  * HTTP: response code returned from the server; 500 indicates a failure to connect
  * Size: response size in content bytes received from the upstream (response body, not headers)
  * From_Location: where you said the test was running from (REP_LOCATION environment variable)
  * Remote_Addr: the IP address hit by the test (may change over time, based upon DNS result)
  * proto://uri: the request URL (protocol and URI requested)

The final section provides the count of samples, the total time, and averages
for the above values. If you test to multiple endpoints you'll see multiple
sections as each completes.

**Docker**: To run the containerized app, use `gmake run` from the command line,
which will build the docker image (if needed) and run it out of the local docker
repo with default arguments. It will run in asymmetric mode but you can connect
to it from another system as follows

<!-- TODO: describe example how to connect to local instance from command line -->

If you want to push it to your DockerHub repo, you can `gmake push`.  This
requires the following environment variables:

``` shell
export DOCKER_USER="your docker username"
export DOCKER_EMAIL="the email address you registered with DockerHub"
```

You will need to login to DockerHub to establish credentials:

``` shell
docker login --username=${DOCKER_USER} --email=${DOCKER_EMAIL}
```

## Run from Docker

If you don't want to build your own local copy, but you have docker installed,
you can run a pre-built version from there using a command line similar to this:

<!-- Validate / update command line -->
``` shell
docker run rafaysystems/pingmesh:v1 -n 5 -d 1 https://www.google.com/
```


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
| AWS_REGION | your AWS preferred region | CloudWatch region |
| AWS_ACCESS_KEY_ID | your AWS access key id | CloudWatch credentials |
| AWS_SECRET_ACCESS_KEY | your AWS secret access key | CloudWatch credentials |

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
  * Login to [your AWS console](console.aws.amazon.com) (the one that
    corresponds to AWS credentials you entered into the Rafay console).
  * Navigate to CloudWatch Metrics, select all metrics, and watch the data
    roll in.
    
You'll note that the Rafay workload picks up a location label automatically
from the environment: we put REP_LOCATION, and a few other items, into the
shell environment of every container.  You can see them by running a
go-httpbin testapp we also provide (at the /env/ endpoint).

