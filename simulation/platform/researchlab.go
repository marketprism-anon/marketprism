// This file generates the Simulation.bin files as well as prepopulates
// the tables for the new orchestrator

package platform

import (
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"runtime"

	"github.com/BurntSushi/toml"
	"github.com/chainBoostScale/ChainBoost/MainAndSideChain/blockchain"
	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/app"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"golang.org/x/xerrors"
)

// Researchlab holds all fields necessary for a Researchlab-run
type Researchlab struct {
	// *** Researchlab-related configuration
	// The login on the platform
	Login string
	// The outside host on the platform
	Host string
	// The name of the project
	Project string
	// Name of the Experiment - also name of hosts
	Experiment string
	// Directory holding the simulation-main file
	simulDir string
	// Directory where the Researchlab-users-file is held
	usersDir string
	// Directory where everything is copied into
	deployDir string
	// Directory for building
	buildDir string
	// Directory holding all go-files of onet/simulation/platform
	platformDir string
	// DNS-resolvable names
	Phys []string
	// VLAN-IP names (physical machines)
	Virt []string
	// Channel to communication stopping of experiment
	sshDeter chan string
	// Whether the simulation is started
	started bool

	// ProxyAddress : the proxy will redirect every traffic it
	// receives to this address
	ProxyAddress string
	// MonitorAddress is the address given to clients to connect to the monitor
	// It is actually the Proxy that will listen to that address and clients
	// won't know a thing about it
	MonitorAddress string
	// Port number of the monitor and the proxy
	MonitorPort int

	// Number of available servers
	Servers int
	// Name of the simulation
	Simulation string
	// Number of machines
	Hosts int
	// Debugging-level: 0 is none - 5 is everything
	Debug int
	// RunWait for long simulations
	RunWait string
	// suite used for the simulation
	Suite string
	// PreScript defines a script that is run before the simulation
	PreScript string
	// Tags to use when compiling
	Tags string

	// : adding some other system-wide configurations
	MCRoundDuration          int
	PercentageTxPay          int
	MainChainBlockSize       int
	SideChainBlockSize       int
	SectorNumber             int
	NumberOfPayTXsUpperBound int
	SimulationRounds         int
	SimulationSeed           int
	//-- bls cosi
	NbrSubTrees     int
	Threshold       int
	SCRoundDuration int
	CommitteeWindow int
	MCRoundPerEpoch int
	SimState        int
	Vms             []string
	Sidechains      []string
	FaultyContractsRate                 float64
}

var simulationConfig *onet.SimulationConfig

// Configure initialises the directories and loads the saved config
// for Researchlab
func (d *Researchlab) Configure(pcs []Config) {
	// Directory setup - would also be possible in /tmp
	pwd, _ := os.Getwd()
	d.Suite = pcs[0].Suite
	d.simulDir = pwd
	d.deployDir = pwd + "/deploy"
	d.buildDir = pwd + "/build"
	_, file, _, _ := runtime.Caller(0)
	d.platformDir = path.Dir(file)
	os.RemoveAll(d.deployDir)
	os.Mkdir(d.deployDir, 0770)
	os.Mkdir(d.buildDir, 0770)
	log.LLvl1("Dirs are:", pwd, d.deployDir)

	// Setting up channel
	d.sshDeter = make(chan string)
}

// Build prepares all binaries for the Researchlab-simulation.
// If 'build' is empty, all binaries are created, else only
// the ones indicated. Either "simul" or "users"
func (d *Researchlab) Build(build string, arg ...string) error {
	return nil
}

// Cleanup kills all eventually remaining processes from the last Deploy-run
func (d *Researchlab) Cleanup() error {
	return nil
}

// Deploy creates the appropriate configuration-files and copies everything to the
// Researchlab-installation.
func (d *Researchlab) Deploy(rc *RunConfig) error {
	if err := os.RemoveAll(d.deployDir); err != nil {
		return xerrors.Errorf("removing folders: %v", err)
	}
	if err := os.Mkdir(d.deployDir, 0777); err != nil {
		return xerrors.Errorf("making folder: %v", err)
	}

	// Check for PreScript and copy it to the deploy-dir
	d.PreScript = rc.Get("PreScript")
	if d.PreScript != "" {
		_, err := os.Stat(d.PreScript)
		if !os.IsNotExist(err) {
			if err := app.Copy(d.deployDir, d.PreScript); err != nil {
				return xerrors.Errorf("copying: %v", err)
			}
		}
	}

	// deploy will get rsync to /remote on the NFS

	log.LLvl1("Researchlab: Deploying and writing config-files")
	sim, err := onet.NewSimulation(d.Simulation, string(rc.Toml()))
	if err != nil {
		return xerrors.Errorf("simulation: %v", err)
	}
	// Initialize the deter-struct with our current structure (for debug-levels
	// and such), then read in the app-configuration to overwrite eventual
	// 'Machines', 'ppm', '' or other fields
	deter := *d
	_, err = toml.Decode(string(rc.Toml()), &deter)
	if err != nil {
		return xerrors.Errorf("decoding toml: %v", err)
	}
	deter.Virt = d.Vms

	simulationConfig, err = sim.Setup(d.deployDir, deter.Virt)
	if err != nil {
		return xerrors.Errorf("simulation setup: %v", err)
	}
	simulationConfig.Config = string(rc.Toml())
	log.LLvl1("Saving configuration")
	if err := simulationConfig.Save(d.deployDir); err != nil {
		log.Error("Couldn't save configuration:", err)
	}

	// Copy limit-files for more connections
	ioutil.WriteFile(path.Join(d.deployDir, "simul.conf"),
		[]byte(simulConnectionsConf), 0444)

	// --------------------------------------------

	// Coder: initializing main chain's blockchain -------------------------
	log.LLvl1("Initializing main chain's blockchain")
	var expConf blockchain.ExperimentConfig
	_, err = toml.Decode(string(rc.Toml()), &expConf)
	blockchain.InitializeMainChainBC(expConf)
	// Coder: initializing side chain's blockchain -------------------------
	blockchain.InitializeSideChainBC(expConf.SideChains)

	// --------------------------------------------
	//ToDo : is it the best way to do so?!
	// Copying central bc files to deploy-directory so it gets transferred to distributed servers
	err = exec.Command("cp", d.simulDir+"/"+"mainchain.db", d.deployDir).Run()
	if err != nil {
		log.Fatal("error copying mainchain.db: ", err)
	}
	for _, s := range d.Sidechains {
		err = exec.Command("cp", d.simulDir+"/"+s+".db", d.deployDir).Run()
		if err != nil {
			log.Fatal("error copying sidechainbc.xlsx: ", err)
		}
	}
	//ToDo : is it the best way to do so?!
	// Copying chainBoost.toml file to deploy-directory so it gets transferred to distributed servers
	err = exec.Command("cp", d.simulDir+"/"+d.Simulation+".toml", d.deployDir).Run()
	if err != nil {
		log.Fatal("error copying chainBoost.toml-file:", d.simulDir, d.Simulation+".toml to ", d.deployDir, err)
	}

	// Copying build-files to deploy-directory
	build, _ := ioutil.ReadDir(d.buildDir)
	for _, file := range build {
		err = exec.Command("cp", d.buildDir+"/"+file.Name(), d.deployDir).Run()
		if err != nil {
			log.Fatal("error copying build-file:", d.buildDir, file.Name(), d.deployDir, err)
		}
	}

	return nil
}

// Start creates a tunnel for the monitor-output and contacts the Researchlab-
// server to run the simulation
func (d *Researchlab) Start(args ...string) error {
	return nil
}

// Wait for the process to finish
func (d *Researchlab) Wait() error {
	return nil
}

// Checks whether host, login and project are defined. If any of them are missing, it will
// ask on the command-line.
// For the login-variable, it will try to set up a connection to d.Host and copy over the
// public key for a more easy communication
func (d *Researchlab) loadAndCheckResearchlabVars() {
	deter := Researchlab{}
	err := onet.ReadTomlConfig(&deter, "deter.toml")
	d.Host, d.Login, d.Project, d.Experiment, d.ProxyAddress, d.MonitorAddress =
		deter.Host, deter.Login, deter.Project, deter.Experiment,
		deter.ProxyAddress, deter.MonitorAddress

	if err != nil {
		log.LLvl1("Couldn't read config-file - asking for default values")
	}

	if d.Host == "" {
		d.Host = readString("Please enter the hostname of Researchlab", "research-lab-url")
	}

	login, err := user.Current()
	log.ErrFatal(err)
	if d.Login == "" {
		d.Login = readString("Please enter the login-name on "+d.Host, login.Username)
	}

	if d.Project == "" {
		d.Project = readString("Please enter the project on Researchlab", "SAFER")
	}

	if d.Experiment == "" {
		d.Experiment = readString("Please enter the Experiment on "+d.Project, "Dissent-CS")
	}

	if d.MonitorAddress == "" {
		d.MonitorAddress = readString("Please enter the Monitor address (where clients will connect)", "research-lab-url:22")
	}
	if d.ProxyAddress == "" {
		d.ProxyAddress = readString("Please enter the proxy redirection address", "localhost")
	}

	onet.WriteTomlConfig(*d, "deter.toml")
}
