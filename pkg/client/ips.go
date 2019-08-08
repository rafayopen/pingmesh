package client

import (
	"log"
	"net"
)

func GetIPs(hostname string) (ips []net.IP) {
	var err error
	if len(hostname) > 0 {
		ips, err = net.LookupIP(hostname)
		if err != nil {
			log.Println("Warning: Could not LookupIP for", hostname, err)
		}
	}
	return
}
