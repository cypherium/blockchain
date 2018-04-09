package main

/*
The simulation-file can be used with the `cothority/simul` and be run either
locally or on deterlab. Contrary to the `test` of the protocol, the simulation
is much more realistic, as it tests the protocol on different nodes, and not
only in a test-environment.

The Setup-method is run once on the client and will create all structures
and slices necessary to the simulation. It also receives a 'dir' argument
of a directory where it can write files. These files will be copied over to
the simulation so that they are available.

The Run-method is called only once by the root-node of the tree defined in
Setup. It should run the simulation in different rounds. It can also
measure the time each run takes.

In the Node-method you can read the files that have been created by the
'Setup'-method.
*/

import (
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/cypherium/blockchain/blockchain"
	"github.com/cypherium/blockchain/cypherium"
	"github.com/dedis/onet"
	"github.com/dedis/onet/log"
	"github.com/dedis/onet/simul/monitor"
)

func init() {
	onet.SimulationRegister("cypherium", NewCypheriumSimulation)
}

// CypheriumSimulation implements onet.Simulation.
type CypheriumSimulation struct {
	onet.SimulationBFTree
	// your simulation specific fields:
	CypheriumSimulationConfig
}

// CypheriumSimulationConfig is the config used by the simulation for cypherium
type CypheriumSimulationConfig struct {
	// Blocksize is the number of transactions in one block:
	Blocksize int
	// timeout the leader after TimeoutMs milliseconds
	TimeoutMs uint64
	// Fail:
	// 0  do not fail
	// 1 fail by doing nothing
	// 2 fail by sending wrong blocks
	Fail uint
}

// NewCypheriumSimulation is used internally to register the simulation (see the init()
// function above).
func NewCypheriumSimulation(config string) (onet.Simulation, error) {
	es := &CypheriumSimulation{}
	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, err
	}
	return es, nil
}

// Setup implements onet.Simulation interface. It checks on the availability
// of the block-file and downloads it if missing. Then the block-file will be
// copied to the simulation-directory
func (s *CypheriumSimulation) Setup(dir string, hosts []string) (
	*onet.SimulationConfig, error) {

	err := blockchain.EnsureBlockIsAvailable(dir)
	if err != nil {
		log.Fatal("Couldn't get block:", err)
	}

	sc := &onet.SimulationConfig{}
	s.CreateRoster(sc, hosts, 2000)
	err = s.CreateTree(sc)
	if err != nil {
		return nil, err
	}
	return sc, nil
}

type monitorMut struct {
	*monitor.TimeMeasure
	sync.Mutex
}

func (m *monitorMut) NewMeasure(id string) {
	m.Lock()
	defer m.Unlock()
	m.TimeMeasure = monitor.NewTimeMeasure(id)
}
func (m *monitorMut) Record() {
	m.Lock()
	defer m.Unlock()
	m.TimeMeasure.Record()
	m.TimeMeasure = nil
}

// Node can be used to initialize each node before it will be run
// by the server. Here we call the 'Node'-method of the
// SimulationBFTree structure which will load the roster- and the
// tree-structure to speed up the first round.
func (s *CypheriumSimulation) Node(config *onet.SimulationConfig) error {
	index, _ := config.Roster.Search(config.Server.ServerIdentity.ID)
	if index < 0 {
		log.Fatal("Didn't find this node in roster")
	}
	log.Lvl3("Initializing node-index", index)
	return s.SimulationBFTree.Node(config)
}

// Run implements onet.Simulation.
func (s *CypheriumSimulation) Run(onetConf *onet.SimulationConfig) error {

	size := onetConf.Tree.Size()
	log.Lvl2("Simulation starting with: Rounds=", s.Rounds, s.Fail, "Size is:", size)

	server := cypherium.NewCypheriumServer(s.Blocksize, s.TimeoutMs, s.Fail)

	for round := 0; round < s.Rounds; round++ {

		log.Lvl1("Starting round", round)

		client := cypherium.NewClient(server)
		err := client.StartClientSimulation(blockchain.GetBlockDir(), s.Blocksize)
		if err != nil {
			log.Error("Error in ClientSimulation:", err)
			return err
		}
		// instantiate a cypherium protocol
		pi, err := onetConf.Overlay.CreateProtocol(cypherium.CypheriumProtocolName, onetConf.Tree,
			onet.NilServiceID)
		if err != nil {
			return err
		}

		// rComplete := monitor.NewTimeMeasure("round")
		proto := pi.(*cypherium.TemplateProtocol)
		root, err := server.Instantiate(proto)
		if err != nil {
			return err
		}

		root.CreateProtocol = func(name string, t *onet.Tree) (onet.ProtocolInstance, error) {
			return onetConf.Overlay.CreateProtocol(name, t, onet.NilServiceID)
		}

		go pi.Start()

		timeout := time.Second * 60
		select {
		case children := <-root.ChildCount:
			log.Lvl2("Instance 1 is done", children)
		case <-time.After(timeout):
			log.Lvl2("Didn't finish in time")
		}
		log.Lvl3("Round", round, "finished")

	}
	return nil
}
