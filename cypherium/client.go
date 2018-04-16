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
	"errors"
	"fmt"

	"github.com/blockchain/blockchain"
	"github.com/dedis/onet/log"
)

var magicNum = [4]byte{0xF9, 0xBE, 0xB4, 0xD9}

// ReadFirstNBlocks specifcy how many blocks in the the BlocksDir it must read
// (so you only have to copy the first blocks to deterLab)
const ReadFirstNBlocks = 400

// Client is a client simulation. At the moment we do not measure the
// communication between client and server. Hence, we do not even open a real
// network connection
type Client struct {
	// holds the sever as a struct
	srv BlockServer
}

// NewClient returns a fresh new client out of a blockserver
func NewClient(s BlockServer) *Client {
	return &Client{srv: s}
}

// StartClientSimulation can be called from outside (from an simulation
// implementation) to simulate a client. Parameters:
// blocksDir is the directory where to find the transaction blocks (.dat files)
// numTxs is the number of transactions the client will create
func (c *Client) StartClientSimulation(blocksDir string, numTxs int) error {
	return c.triggerTransactions(blocksDir, numTxs)
}

func (c *Client) triggerTransactions(blocksPath string, nTxs int) error {
	log.Lvl2("cypherium Client will trigger up to", nTxs, "transactions")
	parser, err := blockchain.NewParser(blocksPath, magicNum)
	if err != nil {
		log.Error("Error: Couldn't parse blocks in", blocksPath,
			".\nPlease download bitcoin blocks as .dat files first and place them in",
			blocksPath, "Either run a bitcoin node (recommended) or using a torrent.")
		return err
	}

	transactions, err := parser.Parse(0, ReadFirstNBlocks)
	if err != nil {
		return fmt.Errorf("Error while parsing transactions %v", err)
	}
	if len(transactions) == 0 {
		return errors.New("Couldn't read any transactions.")
	}
	if len(transactions) < nTxs {
		return fmt.Errorf("Read only %v but caller wanted %v", len(transactions), nTxs)
	}
	consumed := nTxs
	for consumed > 0 {
		for i, _ := range transactions {
			// "send" transaction to server (we skip tcp connection on purpose here)
			//c.srv.AddTransaction(tr)
			i++
			break
		}
		consumed--
	}
	return nil
}
