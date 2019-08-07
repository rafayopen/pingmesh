// client implements the pingmesh request generator
package client

import (
	"github.com/rafayopen/perftest/pkg/pt"
	"github.com/rafayopen/pingmesh/pkg/server"

	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"time"
)

var _ = server.PingmeshServer()

// Parse the uri argument and return URL object for it, or nil on failure.
func ParseURL(uri string) *url.URL {
	if !strings.Contains(uri, "://") && !strings.HasPrefix(uri, "//") {
		uri = "//" + uri
	}

	url, err := url.Parse(uri)
	if err != nil {
		log.Printf("could not parse url %q: %v\n", uri, err)
		return nil
	}

	if url.Scheme == "" {
		url.Scheme = "http"
	}
	return url
}

// leveraged from net/http/http.go but return the index of the colon before port or -1
func portIndex(s string) int {
	lc := strings.LastIndex(s, ":")
	lb := strings.LastIndex(s, "]")
	if lc > lb {
		return lc
	}
	return -1
}

func HostNoPort(addr string) string {
	if colon := portIndex(addr); colon > 0 {
		return addr[:colon] // trim off :port
	}
	return addr
}

// FetchURL makes an HTTP request to the given URL, reads and discards the response
// body, and returns a PingTimes object with detailed timing information from the fetch.
// The caller should pass in a valid location string, for example "City,Country" where
// the client is running.
func FetchURL(rawurl string, myLocation string) *pt.PingTimes {
	// Leveraged from https://github.com/reorx/httpstat
	url := ParseURL(rawurl)
	if url == nil {
		log.Println("cannot parse URL", rawurl)
		return nil
	}

	urlStr := url.Scheme + "://" + url.Host + url.Path

	httpMethod := http.MethodGet

	req, err := http.NewRequest(httpMethod, urlStr, nil)
	if err != nil {
		log.Printf("create request: %v", err)
		return nil
	}

	rmtAddr := "undefined"

	var tStart, tDnsLk, tTcpHs, tConnd, tFirst, tTlsSt, tTlsHs, tClose time.Time

	tStart = time.Now()

	trace := &httptrace.ClientTrace{
		DNSStart: func(_ httptrace.DNSStartInfo) { tStart = time.Now() },
		DNSDone: func(i httptrace.DNSDoneInfo) {
			tDnsLk = time.Now()
		},
		ConnectStart: func(_, _ string) {
			if tDnsLk.IsZero() {
				// connecting to IP -- may be called multiple times (see httptrace.ClientTrace doc)
				// so only kee the first timestamp
				tDnsLk = tStart
			}
		},
		ConnectDone: func(net, addr string, err error) {
			tTcpHs = time.Now()
			rmtAddr = HostNoPort(addr)
			if err != nil {
				log.Printf("connect %s: %v", addr, err)
				// return
			}
		},

		// TLSHandshakeStart is called when the TLS handshake is started. When
		// connecting to a HTTPS site via a HTTP proxy, the handshake happens after
		// the CONNECT request is processed by the proxy.
		TLSHandshakeStart: func() { tTlsSt = time.Now() }, // same as tTcpHs (roughly)???
		TLSHandshakeDone: func(_ tls.ConnectionState, err error) {
			if err != nil {
				log.Printf("TLS HS: %v", err)
			}
			tTlsHs = time.Now() // same as tConnd???
		},

		GotConn:              func(_ httptrace.GotConnInfo) { tConnd = time.Now() },
		GotFirstResponseByte: func() { tFirst = time.Now() },
	}
	req = req.WithContext(httptrace.WithClientTrace(context.Background(), trace))

	tr := &http.Transport{
		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Warning: skips CA checks, but ping doesn't care
		},
	}

	client := &http.Client{
		Transport: tr,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// do not follow redirects; collect timing on the 301/302 instead
			return http.ErrUseLastResponse
		},
	}

	// capturing starttime here just before client.Do() would be more correct, but cause
	// tStart (DNS lookup start time) to appear to be in the past.  Is that OK?  I think no,
	// so request start time is before the connection is attempted.
	status := 520
	var bytes int64
	resp, err := client.Do(req)
	if resp != nil {
		// Close body if non-nil, whatever err says (even if err non-nil)
		defer resp.Body.Close() // after we read the resonse body
	}
	if err != nil {
		log.Printf("reading response: %v", err)
	} else {
		// drain the response body, read all the bytes to set close time correctly
		bytes = readResponseBody(req, resp)
		status = resp.StatusCode
	}
	tClose = time.Now() // after read body

	if tTcpHs.IsZero() { // DNS lookup failed or otherwise failed to connect
		tTcpHs = tDnsLk
		tFirst = tClose
		tConnd = tFirst
	} else if tConnd.IsZero() { // in case of read: connection reset by peer
		tFirst = tClose
		tConnd = tFirst
	}

	return &pt.PingTimes{
		Start:    tStart,             // request start
		DnsLk:    tDnsLk.Sub(tStart), // DNS lookup
		TcpHs:    tTcpHs.Sub(tDnsLk), // TCP connection handshake
		TlsHs:    tTlsHs.Sub(tTlsSt), // TLS handshake
		Reply:    tFirst.Sub(tConnd), // server processing: first byte time
		Close:    tClose.Sub(tFirst), // content transfer: last byte time
		Total:    tClose.Sub(tDnsLk), // request time not including DNS lookup
		DestUrl:  &urlStr,            // URL that received the request
		Location: &myLocation,        // Client location, City,Country
		Remote:   rmtAddr,            // Server IP from DNS resolution
		RespCode: status,
		Size:     bytes,
	}
}

// Consumes the body of the response ... simply discarding it at this point (be as fast as possible).
func readResponseBody(req *http.Request, resp *http.Response) int64 {
	if req.Method == http.MethodHead {
		return 0
	}

	w := ioutil.Discard
	bytes, err := io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("reading HTTP response body: %v", err)
	}
	return bytes
}

// LocationFromEnv returns the current location from environment variables:
// REP_LOCATION, if defined;
// otherwise REP_CITY "," REP_COUNTRY, if defined;
// otherwise "" (empty string)
func LocationFromEnv() string {
	var myLocation string
	repCity := os.Getenv("REP_CITY")
	repCountry := os.Getenv("REP_COUNTRY")
	repLocation := os.Getenv("REP_LOCATION")
	switch {
	case len(repLocation) > 0:
		myLocation = repLocation
	case len(repCity) > 0 && len(repCountry) > 0:
		myLocation = repCity + "," + repCountry
	case len(repCity) == 0 && len(repCountry) == 0:
		// log.Println("Warning: location not provided in env, using", myLocation)
	default:
		myLocation = repCity + repCountry
	}
	return myLocation
}
