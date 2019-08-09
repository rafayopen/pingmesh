#!/bin/sh
PM="cmd/pingmesh/pingmesh"
CW=${CW:-""}					# use -c to publish multi-location data to cloudwatch
DEL=${DEL:-5}					# delay between pings

usage="Usage: $0 [number of instances]"

if [ "$1" = "loc" ] ; then
    export REP_LOCATION=Petosky,MI
    $PM -s 8081 -v http://localhost:8081/v1/ping &			# start one server pinging itself
    export REP_LOCATION=Charlevoix,MI
    # start a local instance, pinging the remote instance
    $PM -s 8080 -v -n ${DEL} "http://localhost:8081/v1/ping" "http://127.0.0.1:8081/v1/ping" &
    wait
    exit 0
fi

echo try one ... run for one minute on 8080
$PM -v -H localhost -s 8080 -d 1 -n 60 "http://localhost:8080/v1/ping" 
code=$?
if [ $code -ne 0 ] ; then
    echo $PM exited with code $code
    echo $usage
    exit $code
fi

[ $# -eq 0 ] && exit 0

echo
if [ "$1" -gt 0 ] ; then
    min=8081
    max=$((8080 + $1))
    echo try N=$1 instances from $min to $max pinging forever... use SIGINT
else
    echo $usage
    exit 1
fi

urls=""
for i in $(seq $min $max) ; do
    urls="$urls http://127.0.0.1:$i/v1/ping"
    $PM $CW -s $i -d ${DEL} $urls &
done

wait
