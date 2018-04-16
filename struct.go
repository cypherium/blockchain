package template

/*
This holds the messages used to communicate with the service over the network.
*/

import (
	"github.com/blockchain/blockchain"
	"github.com/dedis/onet"
	"github.com/dedis/onet/network"
)

// We need to register all messages so the network knows how to handle them.
func init() {
	network.RegisterMessages(
		Count{}, CountReply{},
		Clock{}, ClockReply{},
		Transaction{}, TransReply{},
	)
}

const (
	// ErrorParse indicates an error while parsing the protobuf-file.
	ErrorParse = iota + 4000
)

// Clock will run the tepmlate-protocol on the roster and return
// the time spent doing so.
type Clock struct {
	Roster *onet.Roster
}

// ClockReply returns the time spent for the protocol-run.
type ClockReply struct {
	Time     float64
	Children int
}

// Count will return how many times the protocol has been run.
type Count struct {
}

// CountReply returns the number of protocol-runs
type CountReply struct {
	Count int
}

var MagicNum = [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

// ReadFirstNBlocks specifcy how many blocks in the the BlocksDir it must read
// (so you only have to copy the first blocks to deterLab)
const ReadFirstNBlocks = 400

// Transaction will run the tepmlate-protocol on the roster and return
// the result status.
type Transaction struct {
	Roster   *onet.Roster
	TransMsg []blockchain.STransaction
}

// TransReply return the result status.
type TransReply struct {
	Time     float64
	Children int
	Status   bool
}
