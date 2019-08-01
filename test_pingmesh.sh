#!/bin/sh
PM="cmd/pingmesh/pingmesh"
usage="Usage: $0 [number of instances]"

echo try one
$PM -v -s 8080 -n 5 -d 1 http://localhost:8080/v1/ping#Charlevoix,MI
code=$?
if [ $code -ne 0 ] ; then
    echo $PM exited with code $code
    echo $usage
    exit $code
fi

echo
if [ "$1" -gt 0 ] ; then
    min=8081
    max=$((8080 + $1))
    echo try N=$1 instances from $min to $max
else
    echo $usage
    exit 1
fi

urls=""
for i in $(seq $min $max) ; do
    urls="$urls http://127.0.0.1:$i/v1/ping"
    $PM -s $i -c -n 60 -d 10 $urls &
done

wait
