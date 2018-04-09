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

package cypherium_test

/*
The test-file should at the very least run the cypherium for a varying number
of nodes. It is even better practice to test the different methods of the
cypherium, as in Test Driven Development.
*/

import (
	"testing"
	"time"

	"github.com/cypherium/blockchain/bftcosi"
	"github.com/cypherium/blockchain/cypherium"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/network"
	"github.com/stretchr/testify/require"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

// Tests a 2, 5 and 13-node system. It is good practice to test different
// sizes of trees to make sure your cypherium is stable.
func TestNode(t *testing.T) {

	log.Lvl1("cypherium protocal unit test")
	// Register test protocol using cypherium main protocal
	onet.GlobalProtocolRegister(cypherium.CypheriumProtocolName, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return cypherium.NewProtocol(n)
	})
	// Register test protocol using BFTCoSi
	onet.GlobalProtocolRegister(cypherium.BFTCoSiProtocolName, func(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
		return bftcosi.NewBFTCoSiProtocol(n, cypherium.Verify)
	})

	nodes := []int{2, 5, 13, 100}

	log.SetDebugVisible(5)

	for _, nbrNodes := range nodes {
		local := onet.NewLocalTest(tSuite)
		servers, roster, tree := local.GenBigTree(nbrNodes, nbrNodes, nbrNodes-1, true)
		require.NotNil(t, roster)
		log.Lvl3(tree.Dump())

		pi, err := local.CreateProtocol(cypherium.CypheriumProtocolName, tree)
		require.Nil(t, err)

		root := pi.(*cypherium.TemplateProtocol)
		root.CreateProtocol = local.CreateProtocol
		root.Msg = []byte("Hello BFTCoSi")

		// kill the leafs first
		killCount := min(0, len(servers))
		for i := len(servers) - 1; i > len(servers)-killCount-1; i-- {
			log.Lvl1("Closing server:", servers[i].ServerIdentity.Public, servers[i].Address())
			if e := servers[i].Close(); e != nil {
				log.Lvl1("Closing server error:", e)
			}
		}

		err = root.Start()
		require.Nil(t, err)

		timeout := network.WaitRetry * time.Duration(network.MaxRetryConnect*nbrNodes*2) * time.Millisecond
		select {
		case children := <-root.ChildCount:
			log.Lvl2("Instance 1 is done", children)
			require.Equal(t, children, nbrNodes, "Didn't get a child-cound of", nbrNodes)
		case <-time.After(timeout):
			t.Fatal("Didn't finish in time")
		}
		local.CloseAll()
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
