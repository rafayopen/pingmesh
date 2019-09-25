package client

import (
	"testing"
)

func TestMakePeerAddr(t *testing.T) {
	cases := []struct {
		scheme, host, ip  string
		newhost, peerAddr string
	}{
		// fully specified
		{"http", "hosta", "1.2.3.4", "hosta", "1.2.3.4:80"},
		{"https", "hosta", "1.2.3.4", "hosta", "1.2.3.4:443"},
		{"http", "hosta:80", "1.2.3.4", "hosta", "1.2.3.4:80"},
		{"https", "hosta:80", "1.2.3.4", "hosta", "1.2.3.4:80"},
		{"http", "hosta:443", "1.2.3.4", "hosta", "1.2.3.4:443"},
		{"https", "hosta:443", "1.2.3.4", "hosta", "1.2.3.4:443"},

		// no scheme specified
		{"", "www.example.com", "1.2.3.4", "www.example.com", "1.2.3.4:443"},
		{"", "www.example.com:80", "1.2.3.4", "www.example.com", "1.2.3.4:80"},
		{"", "www.example.com:443", "1.2.3.4", "www.example.com", "1.2.3.4:443"},

		// no override IP address specified (use hostname)
		{"http", "hosta", "", "hosta", "hosta:80"},
		{"https", "hosta", "", "hosta", "hosta:443"},
		{"http", "hosta:80", "", "hosta", "hosta:80"},
		{"https", "hosta:80", "", "hosta", "hosta:80"},
		{"http", "hosta:443", "", "hosta", "hosta:443"},
		{"https", "hosta:443", "", "hosta", "hosta:443"},
		{"", "www.example.com", "", "www.example.com", "www.example.com:443"},
		{"", "www.example.com:80", "", "www.example.com", "www.example.com:80"},
		{"", "www.example.com:443", "", "www.example.com", "www.example.com:443"},
	}

	for n, c := range cases {
		newhost, peerAddr := MakePeerAddr(c.scheme, c.host, c.ip)
		if newhost != c.newhost {
			t.Error("case", n, "got host", newhost, "want", c.newhost)
		}
		if peerAddr != c.peerAddr {
			t.Error("case", n, "got peer", peerAddr, "want", c.peerAddr)
		}
	}
}

func TestFetchURL(t *testing.T) {
	cases := []struct {
		url      string
		ok       bool
		respCode int
	}{
		{"", true, 502},
		{"ftp://hostany", true, 502},
		{"http:/bad-url-oneslash", true, 502},
		{"http://bad-hostname", true, 502},
		{"https://bad-hostname", true, 502},
		{"http://google.com", true, 301},
		{"https://google.com", true, 301},
		{"https://www.google.com", true, 200},
	}

	for n, c := range cases {
		pt := FetchURL(c.url, "")
		if pt != nil {
			if !c.ok {
				t.Error("case", n, "expected fail but got", *pt)
			}
			if pt.RespCode != c.respCode {
				t.Error("case", n, "got resp", pt.RespCode, "want", c.respCode)
			}
		} else if c.ok {
			t.Error("case", n, "fetch failed")
		}
	}
}
