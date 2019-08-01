// client implements the pingmesh request generator
package client

import (
	"github.com/rafayopen/pingmesh/pkg/server"
)

func AddPeer(url, loc string, numTests, pingDelay int) {
	// Create a new peer -- and increment the server's wait group
	peer := server.PingmeshServer().NewPeer(url, loc, numTests, pingDelay)
	go peer.Ping()
}
