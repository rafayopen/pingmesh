#!/bin/sh
PM="cmd/pingmesh/pingmesh -v"

echo try one
$PM -s 8080 -n 5 -d 2 http://localhost:8080/v1/ping
[ $# -eq 0 ] && exit 0

echo try two
####
# Run a pair of standalone instances that talk to each other locally and
# report metrics into cloudwatch
$PM -s 8080 -c -n 60 -d 10 http://127.0.0.1:8081/v1/ping &
$PM -s 8081 -c -n 60 -d 10 http://127.0.0.1:8080/v1/ping http://127.0.0.1:8081/v1/ping &

wait
