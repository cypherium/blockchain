/*
 * Copyright (C) 2018 The Cypherium Blockchain authors
 *
 * This file is part of the Cypherium Blockchain library.
 *
 * The Cypherium Blockchain library is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Lesser General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * The Cypherium Blockchain library is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
 * GNU Lesser General Public License for more details.
 *
 * You should have received a copy of the GNU Lesser General Public License
 * along with the Cypherium Blockchain library. If not, see <http://www.gnu.org/licenses/>.
 *
 */

package cypherium

/*
Struct holds the messages that will be sent around in the protocol. You have
to define each message twice: once the actual message, and a second time
with the `*onet.TreeNode` embedded. The latter is used in the handler-function
so that it can find out who sent the message.
*/

import (
	"sync"

	"github.com/cypherium/blockchain/blockchain"
	"github.com/dedis/kyber"
	"github.com/dedis/onet"
)

// Signature is the final message out of the Cosi-protocol. It can
// be used together with the message and the aggregate public key
// to verify that it's valid.
type Signature struct {
	Challenge kyber.Scalar
	Response  kyber.Scalar
}

// Exception is what a node that does not want to sign should include when
// passing up a response
type Exception struct {
	Public     kyber.Point
	Commitment kyber.Point
}

// BlockSignature is what a cypherium protocol outputs. It contains the signature,
// the block and some possible exceptions.
type BlockSignature struct {
	// cosi signature of the commit round.
	Sig Signature
	// the block signed.
	Block *blockchain.TrBlock
	// List of peers that did not want to sign.
	Exceptions []Exception
}

// CypheriumProtocolName can be used from other packages to refer to this protocol.
const CypheriumProtocolName = "CypheriumProtocol"

// BFTCoSiProtocolName can be used in cypherium protocol.
const BFTCoSiProtocolName = "BFTCoSiProtocol"

// Announce is used to pass a message to all children.
type Announce struct {
	Message string
}

// StructAnnounce just contains Announce and the data necessary to identify and
// process the message in the sda framework.
type StructAnnounce struct {
	*onet.TreeNode
	Announce
}

// Reply returns the count of all children.
type Reply struct {
	ChildrenCount int
}

// StructReply just contains Reply and the data necessary to identify and
// process the message in the sda framework.
type StructReply struct {
	*onet.TreeNode
	Reply
}

type Counter struct {
	veriCount   int
	refuseCount int
	sync.Mutex
}

type Counters struct {
	counters []*Counter
	sync.Mutex
}

func (co *Counters) add(c *Counter) {
	co.Lock()
	co.counters = append(co.counters, c)
	co.Unlock()
}

func (co *Counters) size() int {
	co.Lock()
	defer co.Unlock()
	return len(co.counters)
}

func (co *Counters) get(i int) *Counter {
	co.Lock()
	defer co.Unlock()
	return co.counters[i]
}
