// client implements the pingmesh request generator
package client

import (
	"github.com/rafayopen/perftest/pkg/pt"

	"context"
	"crypto/tls"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"net/http/httptrace"
	"net/url"
	"os"
	"strings"
	"time"
)

const (
	ServedFromPrefix = "<p>Served from "
	ServedFromSuffix = "\n"

	HttpUnknown = 502
	LocUnknown  = "unknown"
)

var (
	PingPeerPaths = []string{
		"/v1/peers",
		"/v1/ping",
		"/v1/addpeer",
	}
)

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
// NOTE: the location handling is different from perftest!  The caller does
// not pass in a location, instead location is parsed from the response body
// and returned in pt.Location.
func FetchURL(rawurl, rmtIP string) *pt.PingTimes {
	// Leveraged from https://github.com/reorx/httpstat
	url := ParseURL(rawurl)
	if url == nil {
		log.Println("cannot parse URL", rawurl)
		return nil
	}

	urlStr := url.Scheme + "://" + url.Host + url.Path

	peerPort := "443"
	if url.Scheme == "http" {
		peerPort = "80"
	}

	peerAddr := url.Host // may contain a port number, if not must add one
	pi := portIndex(url.Host)
	if pi < 0 { // no port, must add one to peerAddr
		peerAddr += ":" + peerPort
	}

	if len(rmtIP) > 0 { // override the hostname with a specific IP
		if pi := portIndex(url.Host); pi > 0 {
			port := url.Host[pi+1:]
			peerAddr = rmtIP + ":" + port
			//			log.Println("override host", url.Host, "with IP[:port]", peerAddr)
		}
	}

	httpMethod := http.MethodGet

	req, err := http.NewRequest(httpMethod, urlStr, nil)
	if err != nil {
		log.Printf("create request: %v", err)
		return nil
	}

	var rmtAddr string

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
				// so only kee the first timestamp; DnsLk will be exactly zero
				tDnsLk = tStart
			}
		},
		ConnectDone: func(net, addr string, err error) {
			tTcpHs = time.Now()
			rmtAddr = HostNoPort(addr)
			if err != nil {
				log.Printf("connect %s: %v", addr, err)
				tTlsSt = tTcpHs
				tTlsHs = tTcpHs
				tFirst = tTcpHs
				tConnd = tTcpHs
				tClose = tTcpHs
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

	dialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
		DualStack: true,
	}

	tr := &http.Transport{
		//		Proxy:                 http.ProxyFromEnvironment,
		MaxIdleConns:          100,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: true, // Warning: skips CA checks, but ping doesn't care
		},
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			//			addr = ip + ":" + urlPort // + addr[strings.LastIndex(addr, ":"):]
			return dialer.DialContext(ctx, network, peerAddr)
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
	status := HttpUnknown
	location := LocUnknown
	var bytes int64
	resp, err := client.Do(req)
	if resp != nil {
		// Close body if non-nil, whatever err says (even if err non-nil)
		defer resp.Body.Close() // after we read the resonse body
	}
	if err != nil {
		log.Printf("reading response: %v", err)
	} else {
		status = resp.StatusCode
		if status == 200 && IsPingmeshPeer(url.Path) {
			location, bytes = readPingResp(req, resp)
		} else {
			bytes = readDiscardBody(req, resp)
		}
	}
	tClose = time.Now() // after read body

	p := pt.PingTimes{
		Start:    tStart,             // request start
		DnsLk:    tDnsLk.Sub(tStart), // DNS lookup
		TcpHs:    tTcpHs.Sub(tDnsLk), // TCP connection handshake
		TlsHs:    tTlsHs.Sub(tTlsSt), // TLS handshake
		Reply:    tFirst.Sub(tConnd), // server processing: first byte time
		Close:    tClose.Sub(tFirst), // content transfer: last byte time
		Total:    tClose.Sub(tDnsLk), // request time not including DNS lookup
		DestUrl:  &urlStr,            // URL that received the request
		Location: &location,          // SERVER, **NOTE**: different from perftest/FetchURL!
		Remote:   rmtAddr,            // Server IP from DNS resolution
		RespCode: status,
		Size:     bytes,
	}

	return &p
}

func IsPingmeshPeer(path string) bool {
	for _, ppp := range PingPeerPaths {
		if path == ppp {
			return true
		}
	}
	return false
}

// readPingResp consumes an HTML ping response body, expecting a location
// string in the <title> and body.  Discards the remaining body.
func readPingResp(req *http.Request, resp *http.Response) (location string, bytes int64) {
	if req.Method == http.MethodHead {
		log.Printf("no HTTP response body in a HEAD")
		return
	}

	body, err := ioutil.ReadAll(resp.Body)
	bytes = int64(len(body))
	if err != nil {
		log.Println("readPingResp:", err)
		return
	}

	sb := string(body)
	locStart := strings.Index(sb, ServedFromPrefix)
	if locStart > 0 {
		locStart += len(ServedFromPrefix) // start of the text
		locEnd := strings.Index(sb[locStart:], ServedFromSuffix)
		if locEnd > 0 {
			location = sb[locStart : locStart+locEnd]
		} else {
			log.Println("Found location prefix, but no suffix?")
		}
	} else {
		log.Println("Did not find location prefix in content body")
	}
	return
}

// Consumes the body of the response ... simply discarding it (as fast as possible).
func readDiscardBody(req *http.Request, resp *http.Response) int64 {
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
