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

import (
	"sync"

	"github.com/blockchain/blockchain"
	//"github.com/blockchain/blockchain/blkparser"
	"github.com/dedis/onet/log"
)

// BlockServer is a struct where Client can connect and that instantiate cypherium
// protocols when needed.
type BlockServer interface {
	AddTransaction(blockchain.STransaction)
	Instantiate(p *TemplateProtocol) (*TemplateProtocol, error)
}

// Server is the long-term control service that listens for transactions and
// dispatch them to a new cypherium for each new signing that we want to do.
// It creates the cypherium protocols and run them. only used by the root since
// only the root participates to the creation of the block.
type Server struct {
	// transactions pool where all the incoming transactions are stored
	transactions []blockchain.STransaction
	// lock associated
	transactionLock sync.Mutex
	// how many transactions should we give to an instance
	blockSize int
	timeOutMs uint64
	fail      uint
	// blockSignatureChan is the channel used to pass out the signatures that
	// cypherium's instances have made
	blockSignatureChan chan BlockSignature
	// enoughBlock signals the server we have enough
	// no comments..
	transactionChan chan blockchain.STransaction
	requestChan     chan bool
	responseChan    chan []blockchain.STransaction
}

// NewCypheriumServer returns a new fresh CypheriumServer. It must be given the blockSize in order
// to efficiently give the transactions to the cypherium instances.
func NewCypheriumServer(blockSize int, timeOutMs uint64, fail uint) *Server {
	s := &Server{
		blockSize:          blockSize,
		timeOutMs:          timeOutMs,
		fail:               fail,
		blockSignatureChan: make(chan BlockSignature),
		transactionChan:    make(chan blockchain.STransaction),
		requestChan:        make(chan bool),
		responseChan:       make(chan []blockchain.STransaction),
	}
	go s.listenEnoughBlocks()
	return s
}

// AddTransaction add a new transactions to the list of transactions to commit
func (s *Server) AddTransaction(tr blockchain.STransaction) {
	s.transactionChan <- tr
}

// ListenClientTransactions will bind to a port a listen for incoming connection
// from clients. These client will be able to pass the transactions to the
// server.
func (s *Server) ListenClientTransactions() {
	panic("not implemented yet")
}

// Instantiate takes blockSize transactions and create the cypherium instances.
func (s *Server) Instantiate(p *TemplateProtocol) (*TemplateProtocol, error) {
	// wait until we have enough blocks
	currTransactions := s.WaitEnoughBlocks()
	//fmt.Printf("Cypherium instantiate: %+v\n", currTransactions)
	log.Lvl2("Instantiate cypherium Round with", len(currTransactions), "transactions")
	pi, err := NewCypheriumRootProtocol(p, currTransactions, s.timeOutMs, s.fail)

	return pi, err
}

// BlockSignaturesChan returns a channel that is given each new block signature as
// soon as they are arrive (Wether correct or not).
func (s *Server) BlockSignaturesChan() <-chan BlockSignature {
	return s.blockSignatureChan
}

func (s *Server) onDoneSign(blk BlockSignature) {
	s.blockSignatureChan <- blk
}

// WaitEnoughBlocks is called to wait on the server until it has enough
// transactions to make a block
func (s *Server) WaitEnoughBlocks() []blockchain.STransaction {
	s.requestChan <- true
	transactions := <-s.responseChan
	return transactions
}

func (s *Server) listenEnoughBlocks() {
	// TODO the server should have a transaction pool instead:
	var transactions []blockchain.STransaction
	var want bool
	for {
		select {
		case tr := <-s.transactionChan:
			// FIXME this will lead to a very large slice if the client sends many
			if len(transactions) < s.blockSize {
				transactions = append(transactions, tr)
			}
			if want {
				if len(transactions) >= s.blockSize {
					s.responseChan <- transactions[:s.blockSize]
					transactions = transactions[s.blockSize:]
					want = false
				}
			}
		case <-s.requestChan:
			want = true
			if len(transactions) >= s.blockSize {
				s.responseChan <- transactions[:s.blockSize]
				transactions = transactions[s.blockSize:]
				want = false
			}
		}
	}
}
