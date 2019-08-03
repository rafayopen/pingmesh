// client implements the pingmesh request generator
package client

import (
	"github.com/rafayopen/pingmesh/pkg/server"
)

////
//  AddPeer adds a ping peer for the given url, in location loc.  It
//  will ping numTests times with a pingDelay between each test.
func AddPeer(url, loc string, numTests, pingDelay int) {
	// Create a new peer -- and increment the server's wait group
	peer := server.PingmeshServer().NewPeer(url, loc, numTests, pingDelay)
	go peer.Ping()
}
