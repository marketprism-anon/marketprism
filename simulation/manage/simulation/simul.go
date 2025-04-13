package main

import (

	//"os/exec"

	"flag"

	"net/http"
	_ "net/http/pprof"

	"github.com/BurntSushi/toml"
	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	simul "github.com/chainBoostScale/ChainBoost/simulation"
	"golang.org/x/xerrors"
)

/*Defines the simulation for each-protocol*/
func init() {
	onet.SimulationRegister("ChainBoost", NewSimulation)
}

// Simulation only holds the BFTree simulation
type simulation struct {
	onet.SimulationBFTree
}

// NewSimulation returns the new simulation, where all fields are
// initialised using the config-file
func NewSimulation(config string) (onet.Simulation, error) {
	es := &simulation{}
	// Set defaults before toml.Decode
	es.Suite = "Ed25519"

	_, err := toml.Decode(config, es)
	if err != nil {
		return nil, xerrors.Errorf("decoding: %v", err)
	}
	return es, nil
}

// Setup creates the tree used for that simulation
func (e *simulation) Setup(dir string, hosts []string) (
	*onet.SimulationConfig, error) {
	sc := &onet.SimulationConfig{}
	log.LLvl1(": creating roster with hosts number of nodes, out of given addresses and starting from port:2000")
	e.CreateRoster(sc, hosts, 2000)
	err := e.CreateTree(sc)
	if err != nil {
		return nil, xerrors.Errorf("creating tree: %v", err)
	}
	return sc, nil
}

// Run is used on the destination machines and runs a number of
// rounds
/*
func (e *simulation) Run(config *onet.SimulationConfig) error {
	size := config.Tree.Size()
	log.LLvl1("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		log.LLvl1("Starting round", round)
		p, err := config.Overlay.CreateProtocol("Count", config.Tree, onet.NilServiceID)
		if err != nil {
			return xerrors.Errorf("creating protocol: %v", err)
		}
		go p.Start()
		children := <-p.(*manage.ProtocolCount).Count
		round.Record()
		if children != size {
			return xerrors.New("Didn't get " + strconv.Itoa(size) +
				" children")
		}
	}
	return nil
}
*/
//Coder
func (e *simulation) Run(config *onet.SimulationConfig) error {
	size := config.Tree.Size()
	log.LLvl1("Size is:", size, "rounds:", e.Rounds)
	for round := 0; round < e.Rounds; round++ {
		log.LLvl1("Starting round", round)
		p, err := config.Overlay.CreateProtocol("ChainBoost", config.Tree, onet.NilServiceID)
		if err != nil {
			return xerrors.Errorf("creating protocol: %v", err)
		}
		go p.Start()
		//round.Record()
	}
	return nil
}

func main() {
	go func() {
		log.LLvl1(http.ListenAndServe("localhost:6060", nil))
	}()
	flag.Parse()
	simul.Start("localhost", "ChainBoost.toml")
}
