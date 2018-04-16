package main

import (
	// Service needs to be imported here to be instantiated.
	_ "github.com/blockchain/service"
	"github.com/dedis/onet/simul"
)

func main() {
	simul.Start()
}
