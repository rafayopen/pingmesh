package client

import (
	"net"
	"time"
)

var (
	myIPs   []net.IP
	fetched time.Time
	ipTTL   time.Duration = time.Second
)

func GetIPs(hostname string) []net.IP {
	if len(myIPs) > 0 && time.Since(fetched) < ipTTL {
		return myIPs
	}

	if len(hostname) > 0 {
		if ips, err := net.LookupIP(hostname); err == nil {
			myIPs = ips
		}
		fetched = time.Now()
		if ipTTL < time.Hour {
			ipTTL = 2 * ipTTL
		}
	}

	return myIPs
}
