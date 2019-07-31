package server

// define state types

////
//  Server state information
////

// get state
func MyLocation() string {
	return myServer.myLoc
}

func CwFlag() bool {
	return myServer.cwFlag
}

// update state
