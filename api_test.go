package template_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	// We need to include the service so it is started.
	"github.com/cypherium_private/mvp"
	_ "github.com/cypherium_private/mvp/service"
	"github.com/dedis/kyber/suites"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}

func TestClient_Clock(t *testing.T) {
	nbr := 5
	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, roster, _ := local.GenTree(nbr, true)
	defer local.CloseAll()

	c := template.NewClient()
	cl1, err := c.Clock(roster)
	require.Nil(t, err)
	require.Equal(t, nbr, cl1.Children)
	cl2, err := c.Clock(roster)
	require.Nil(t, err)
	require.Equal(t, nbr, cl2.Children)
}

func TestClient_Count(t *testing.T) {
	nbr := 5
	local := onet.NewTCPTest(tSuite)
	// generate 5 hosts, they don't connect, they process messages, and they
	// don't register the tree or entitylist
	_, roster, _ := local.GenTree(nbr, true)
	defer local.CloseAll()

	c := template.NewClient()
	// Verify it's all 0s before
	for _, s := range roster.List {
		count, err := c.Count(s)
		require.Nil(t, err)
		require.Equal(t, 0, count)
	}

	// Make some clock-requests
	for range roster.List {
		_, err := c.Clock(roster)
		require.Nil(t, err)
	}

	// Verify we have the correct total of requests
	total := 0
	for _, s := range roster.List {
		count, err := c.Count(s)
		require.Nil(t, err)
		total += count
	}
	require.Equal(t, nbr, total)
}
