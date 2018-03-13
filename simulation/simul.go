package main

import (
	// Service needs to be imported here to be instantiated.
	_ "github.com/cypherium_private/cypherium_simulation/service"
	"github.com/dedis/onet/simul"
)

func main() {
	simul.Start()
}
