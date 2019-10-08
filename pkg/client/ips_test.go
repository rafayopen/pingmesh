package client

import (
	"testing"

	"net"
)

func TestGetIPs(t *testing.T) {
	cases := []struct {
		host string
		ip   net.IP // want at least this one
	}{
		{"localhost", net.IPv6loopback},
		{"localhost", []byte{127, 0, 0, 1}}, // should be net.IsLoopback?
	}

	for n, c := range cases {
		ips := GetIPs(c.host)
		found := false
		for _, ip := range ips {
			if ip.Equal(c.ip) {
				found = true
			}
		}
		if !found {
			t.Error("case", n, "IP", c.ip, "not found for", c.host, "in", ips)
		}
	}
}
