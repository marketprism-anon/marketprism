package platform

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"

	"strings"

	"time"

	//"github.com/chainBoostScale/ChainBoost/onet/app"
	"github.com/BurntSushi/toml"
	"github.com/chainBoostScale/ChainBoost/MainAndSideChain/blockchain"
	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"golang.org/x/xerrors"
)

// Localhost is responsible for launching the app with the specified number of nodes
// directly on your machine, for local testing.

// Localhost is the platform for launching thee apps locally
type Localhost struct {
	// Need mutex because build.go has a global variable that
	// is used for multiple experiments
	sync.Mutex

	// Address of the logger (can be local or not)
	logger string

	// The simulation to run
	Simulation string

	// Where is the Localhost package located
	localDir string
	// Where to build the executables +
	// where to read the config file
	// it will be assembled like LocalDir/RunDir
	runDir string

	// Debug level 1 - 5
	debug int

	// The number of servers
	servers int
	// All addresses - we use 'localhost1'..'localhostn' to
	// identify the different cothorities, but when opening the
	// ports they will be converted to normal 'localhost'
	addresses []string

	// Whether we started a simulation
	running bool
	// WaitGroup for running processes
	wgRun sync.WaitGroup

	// errors go here:
	errChan chan error

	// Suite used for the simulation
	Suite string

	// SimulationConfig holds all things necessary for the run
	sc *onet.SimulationConfig

	// PreScript is run before the simulation is started
	PreScript string

	// RunWait for long simulations
	RunWait string

	// : adding some other system-wide configurations
	MCRoundDuration                  int
	PercentageTxPay                  int
	MainChainBlockSize               int
	SideChainBlockSize               int
	SectorNumber                     int
	NumberOfPayTXsUpperBound         int
	NumberOfActiveContractsPerServer int
	SimulationRounds                 int
	SimulationSeed                   int
	//-- bls cosi

	NbrSubTrees              int
	Threshold                int
	SCRoundDuration          int
	CommitteeWindow          int
	MCRoundPerEpoch          int
	SCRoundPerEpoch          int
	SimState                 int
	StoragePaymentEpoch      int
	PayPercentOfTransactions float64
	configs                  []Config
	Sidechains               []string

	ShardCount          int
	FaultyContractsRate float64
}

// Configure various internal variables
func (d *Localhost) Configure(pcs []Config) {
	d.Lock()
	defer d.Unlock()
	pwd, _ := os.Getwd()
	d.runDir = pwd + "/build"
	os.RemoveAll(d.runDir)
	log.ErrFatal(os.Mkdir(d.runDir, 0770))
	d.configs = pcs
	log.LLvl1(">>>>%+v", pcs)
	log.LLvl1(">>>>%+v", d.configs)
}

// Build does nothing, as we're using our own binary, no need to build
func (d *Localhost) Build(build string, arg ...string) error {
	return nil
}

// Cleanup kills all running cothority-binaryes
func (d *Localhost) Cleanup() error {
	//log.Lvlf3("Nothing to clean up")
	return nil
}

// Deploy copies all files to the run-directory
func (d *Localhost) Deploy(rc *RunConfig) error {
	d.Lock()
	defer d.Unlock()
	if runtime.GOOS == "darwin" {
		files, err := exec.Command("ulimit", "-n").Output()
		if err != nil {
			return xerrors.Errorf("ulimit: %v", err)
		}
		filesNbr, err := strconv.Atoi(strings.TrimSpace(string(files)))
		if err != nil {
			return xerrors.Errorf("atoi: %v", err)
		}
		hosts, _ := strconv.Atoi(rc.Get("hosts"))
		if filesNbr < hosts*2 {
			maxfiles := 10000 + hosts*2
			return xerrors.Errorf("Maximum open files is too small. Please run the following command:\n"+
				"sudo sysctl -w kern.maxfiles=%d\n"+
				"sudo sysctl -w kern.maxfilesperproc=%d\n"+
				"ulimit -n %d\n"+
				"sudo sysctl -w kern.ipc.somaxconn=2048\n",
				maxfiles, maxfiles, maxfiles)
		}
	}

	// --------------------------------------------
	// Coder: initializing main chain's blockchain -------------------------
	log.LLvl1("Initializing main chain's blockchain")
	var expConf blockchain.ExperimentConfig
	md, err := toml.Decode(string(rc.Toml()), &expConf)
	if err != nil {
		panic(err)
	}
	fmt.Print(md.Undecoded())
	blockchain.InitializeMainChainBC(expConf)
	// Coder: initializing side chain's blockchain -------------------------
	blockchain.InitializeSideChainBC(expConf.SideChains)
	// --------------------------------------------

	// ------------------------------------------------------------------
	// Check for PreScript and copy it to the deploy-dir
	// d.PreScript = rc.Get("PreScript")
	// if d.PreScript != "" {
	// 	// added next 2 line
	// 	//ToDoCoder: what?!!!!!!
	// 	pwd := "/ResearchLabab_users/GitHub/chainBoostScale/ChainBoost/simulation/chainBoostFiles/"
	// 	pwd = pwd + d.PreScript
	// 	_, err := os.Stat(pwd /*d.PreScript*/)
	// 	if !os.IsNotExist(err) {
	// 		if err := app.Copy(d.runDir, pwd /*d.PreScript*/); err != nil {
	// 			return xerrors.Errorf("copying: %v", err)
	// 		}
	// 	}
	// }
	// ------------------------------------------------------------------

	d.servers, _ = strconv.Atoi(rc.Get("servers"))
	//log.Lvlf3("Localhost: Deploying and writing config-files for", d.servers, "servers")
	sim, err := onet.NewSimulation(d.Simulation, string(rc.Toml()))
	if err != nil {
		return xerrors.Errorf("simulation error: %v", err)
	}
	d.addresses = make([]string, d.servers)
	for i := range d.addresses {
		d.addresses[i] = "127.0.0." + strconv.Itoa(i+1)
	}
	d.sc, err = sim.Setup(d.runDir, d.addresses)
	if err != nil {
		return xerrors.Errorf("simulation setup: %v", err)
	}
	d.sc.Config = string(rc.Toml())
	if err := d.sc.Save(d.runDir); err != nil {
		return xerrors.Errorf("saving folder: %v", err)
	}
	//log.Lvlf3("Localhost: Done deploying")
	d.wgRun.Add(d.servers)
	// add one to the channel length to indicate it's done
	d.errChan = make(chan error, d.servers+1)
	return nil
}

// Start will execute one cothority-binary for each server
// configured
func (d *Localhost) Start(args ...string) error {
	d.Lock()
	defer d.Unlock()
	if err := os.Chdir(d.runDir); err != nil {
		return err
	}
	//log.Lvlf3("Localhost: chdir into", d.runDir)
	//ex := d.runDir + "/" + d.Simulation
	d.running = true
	//log.Lvlf3("Starting", d.servers, "applications of", ex)
	time.Sleep(100 * time.Millisecond)

	//todo: we don't need prescript to creat bc files,
	// it will be created in deploy inside the InitializeMainChainBC and InitializeSideChainBCfunction invokation
	// but we can use this option whenever we need to runn a command before starting the simulation

	//log.Lvlf3("If PreScript is defined, running the appropriate script_before_the simulation")
	// if d.PreScript != "" {
	// 	out, err := exec.Command("sh", "-c", "./"+d.PreScript+" localhost").CombinedOutput()
	// 	outStr := strings.TrimRight(string(out), "\n")
	// 	if err != nil {
	// 		return xerrors.Errorf("error deploying PreScript: " + err.Error() + " " + outStr)
	// 	}
	// 	////log.Lvlf3(outStr)
	// }

	for index := 0; index < d.servers; index++ {
		////log.Lvlf3("Starting server number: ", index)
		host := "127.0.0." + strconv.Itoa(index+1)
		go func(i int, h string) {
			//log.Lvlf3("Localhost: will start host", i, h)
			//log.Lvlf3(": adding some other system-wide configurations")

			err := Simulate(d.configs, d.Suite, host, d.Simulation)
			if err != nil {
				log.Error("Error running localhost", h, ":", err)
				d.errChan <- err
			}
			d.wgRun.Done()
			log.Lvlf3("host (index", i, ")", h, "done")
		}(index, host)
	}
	return nil
}

// Wait for all processes to finish
func (d *Localhost) Wait() error {
	d.Lock()
	defer d.Unlock()
	//log.Lvlf3("Waiting for processes to finish")

	wait, err := time.ParseDuration(d.RunWait)
	if err != nil || wait == 0 {
		wait = 600 * time.Second
		err = nil
	}

	go func() {
		d.wgRun.Wait()
		//log.Lvlf3("WaitGroup is 0")
		// write to error channel when done:
		d.errChan <- nil
	}()

	// if one of the hosts fails, stop waiting and return the error:
	select {
	case e := <-d.errChan:
		//log.Lvlf3("Finished waiting for hosts:", e)
		if e != nil {
			if err := d.Cleanup(); err != nil {
				log.Errorf("Couldn't cleanup running instances: %+v", err)
			}
			err = xerrors.Errorf("localhost error: %v", err)
		}
	case <-time.After(wait):
		//log.Lvlf3("Quitting after waiting", wait)
	}

	errCleanup := os.Chdir(d.localDir)
	if errCleanup != nil {
		log.Error("Fail to restore the cwd: " + errCleanup.Error())
	}
	log.Lvlf3("Processes finished")
	return err
}
