// ResearchLabab is responsible for setting up everything to test the application
// on ResearchLabab.net
// Given a list of hostnames, it will create an overlay
// tree topology, using all but the last node. It will create multiple
// nodes per server and run timestamping processes. The last node is
// reserved for the logging server, which is forwarded to localhost:8081
//
// Creates the following directory structure:
// build/ - where all cross-compiled executables are stored
// remote/ - directory to be copied to the ResearchLabab server
//
// The following apps are used:
//   ResearchLab - runs on the user-machine in ResearchLabab and launches the others
//   forkexec - runs on the other servers and launches the app, so it can measure its cpu usage

package platform

import (
	"bufio"
	"io/ioutil"
	"os"
	"os/exec"
	"os/user"
	"path"
	"path/filepath"
	"runtime"

	//"strconv"
	"strings"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	//"github.com/chainBoostScale/ChainBoost/onet/app"
	"github.com/chainBoostScale/ChainBoost/MainAndSideChain/blockchain"
	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/app"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"golang.org/x/xerrors"
)

// ResearchLabab holds all fields necessary for a ResearchLabab-run
type ResearchLabab struct {
	// *** ResearchLabab-related configuration
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
	// Directory where the ResearchLabab-users-file is held
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
	sshCss chan string
	// Whether the simulation is started
	started bool
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
	Sidechains               []string
	FaultyContractsRate                 float64
}

var simulConfig *onet.SimulationConfig

// Configure initialises the directories and loads the saved config
// for ResearchLabab
func (d *ResearchLabab) Configure(pcs []Config) {
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
	d.loadAndCheckResearchLababVars()

	// Setting up channel
	d.sshCss = make(chan string)
}

type pkg struct {
	name      string
	processor string
	system    string
	path      string
}

// Build prepares all binaries for the ResearchLabab-simulation.
// If 'build' is empty, all binaries are created, else only
// the ones indicated. Either "simul" or "users"
func (d *ResearchLabab) Build(build string, arg ...string) error {
	log.LLvl1("Building for", d.Login, d.Host, d.Project, build, "simulDir=", d.simulDir)
	start := time.Now()

	var wg sync.WaitGroup

	if err := os.RemoveAll(d.buildDir); err != nil {
		return xerrors.Errorf("removing folders: %v", err)
	}
	if err := os.Mkdir(d.buildDir, 0777); err != nil {
		return xerrors.Errorf("making folder: %v", err)
	}

	// start building the necessary binaries - it's always the same,
	// but built for another architecture.
	packages := []pkg{
		//: changed
		// ResearchLab has an amd64, linux architecture
		{"simul", "arm64", "darwin", path.Join("/Users/Coder/Documents/github.com/chainBoostScale/ChainBoost/simulation/manage", "simulation")},
		{"users", "amd64", "linux", path.Join("/Users/Coder/Documents/github.com/chainBoostScale/ChainBoost/simulation/platform", "ResearchLabab_users")},
		{"simul", "amd64", "linux", path.Join("/Users/Coder/Documents/github.com/chainBoostScale/ChainBoost/simulation/manage", "simulation")},
		//{"simul", "amd64", "linux", d.simulDir},
		//{"simul", "arm64", "linux", "/go/src/github.com/chainBoostScale/ChainBoost/simulation/manage/simulation"},
		//{"users", "arm64", "darwin", d.simulDir},
		//{"users", "arm64", "linux", d.simulDir},
		//{"users", "386", "freebsd", path.Join(d.platformDir, "ResearchLabab_users")},
		//{"users", "arm64", "linux", path.Join(d.platformDir, "ResearchLabab_users")},
	}
	if build == "" {
		build = "simul,users"
	}
	var tags []string
	if d.Tags != "" {
		tags = append([]string{"-tags"}, strings.Split(d.Tags, " ")...)
	}
	log.LLvl1("Starting to build all executables", packages)
	for _, p := range packages {
		if !strings.Contains(build, p.name) {
			log.LLvl1("Skipping build of", p.name)
			continue
		}
		log.LLvl1("Building", p)
		wg.Add(1)
		go func(p pkg) {
			defer wg.Done()
			dst := path.Join(d.buildDir, p.name)
			//
			var path string
			var err error

			d.simulDir = "/Users/Coder/Documents/github.com/chainBoostScale/ChainBoost/simulation/manage/simulation"
			d.platformDir = "/Users/Coder/Documents/github.com/chainBoostScale/ChainBoost/simulation/platform"

			path, err = filepath.Rel(d.simulDir, p.path)
			log.ErrFatal(err)

			var out string
			if p.name == "simul" {
				log.LLvl1("Building: simul")
				out, err = Build(path, dst,
					p.processor, p.system, append(arg, tags...)...)
			} else {
				log.LLvl1("Building: users")
				out, err = Build(path, dst,
					p.processor, p.system, arg...)
			}
			if err != nil {
				KillGo()
				log.Error(out)
				log.Fatal(err)
			}
		}(p)
	}
	// wait for the build to finish
	wg.Wait()
	log.LLvl1("Build is finished after", time.Since(start))
	return nil
}

// Cleanup kills all eventually remaining processes from the last Deploy-run
func (d *ResearchLabab) Cleanup() error {
	// Cleanup eventual ssh from the proxy-forwarding to the logserver
	err := exec.Command("pkill", "-9", "-f", "ssh -nNTf").Run()
	if err != nil {
		log.LLvl1("Error stopping ssh:", err)
	}

	// SSH to the ResearchLabab-server and end all running users-processes
	log.LLvl1("Going to kill everything")
	var sshKill chan string
	sshKill = make(chan string)
	go func() {
		// Cleanup eventual residues of previous round - users and sshd
		if _, err := SSHRun(d.Login, d.Host, "killall -9 users sshd"); err != nil {
			log.LLvl1("Error while cleaning up:", err)
		}

		err := SSHRunStdout(d.Login, d.Host, "test -f remote/users && ( cd remote; ./users -kill )")
		if err != nil {
			log.LLvl1("NOT-Normal error from cleanup", err.Error())
			sshKill <- "error"
		}
		sshKill <- "stopped"
	}()

	for {
		select {
		case msg := <-sshKill:
			if msg == "stopped" {
				log.LLvl1("Users stopped")
				return nil
			}
			log.LLvl1("Received other command", msg, "probably the app didn't quit correctly")
		case <-time.After(time.Second * 20):
			log.LLvl1("Timeout error when waiting for end of ssh")
			return nil
		}
	}
}

// Deploy creates the appropriate configuration-files and copies everything to the
// ResearchLabab-installation.
func (d *ResearchLabab) Deploy(rc *RunConfig) error {
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

	log.LLvl1("ResearchLabab: Deploying and writing config-files")
	sim, err := onet.NewSimulation(d.Simulation, string(rc.Toml()))
	if err != nil {
		return xerrors.Errorf("simulation: %v", err)
	}
	// Initialize the ResearchLab-struct with our current structure (for debug-levels
	// and such), then read in the app-configuration to overwrite eventual
	// 'Machines', 'ppm', '' or other fields
	ResearchLab := *d
	ResearchLabConfig := d.deployDir + "/ResearchLab.toml"
	_, err = toml.Decode(string(rc.Toml()), &ResearchLab)
	if err != nil {
		return xerrors.Errorf("decoding toml: %v", err)
	}
	//-----------------------------------
	// ToDoCoder: filling 2 attributes in ResearchLab struct: ResearchLab.Virt, ResearchLab.Phys
	// by an "string array of IPs and DNS resolvable host names"

	// createHosts and parseHost functions are ResearchLabab API specific.
	// ResearchLab.createHosts()
	// Phys: DNS-resolvable names, Virt: VLAN-IP names (physical machines)
	log.LLvl1("Getting the hosts")
	ResearchLab.Phys = []string{}
	ResearchLab.Virt = []string{}
	//---
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.220:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.220")
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.221:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.221")
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.222:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.222")
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.223:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.223")
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.224:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.224")
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.225:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.225")
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.226:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.226")
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.227:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.227")
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.228:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.228")
	ResearchLab.Phys = append(ResearchLab.Phys, "192.168.3.229:22")
	ResearchLab.Virt = append(ResearchLab.Virt, "192.168.3.229")

	//-----------------------------------

	log.LLvl1("Writing the config file :", ResearchLab)
	onet.WriteTomlConfig(ResearchLab, ResearchLabConfig, d.deployDir)

	simulConfig, err = sim.Setup(d.deployDir, ResearchLab.Virt)
	if err != nil {
		return xerrors.Errorf("simulation setup: %v", err)
	}
	simulConfig.Config = string(rc.Toml())
	log.LLvl1("Saving configuration")
	if err := simulConfig.Save(d.deployDir); err != nil {
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
	err = exec.Command("cp", d.simulDir+"/"+"mainchainbc.xlsx", d.deployDir).Run()
	err = exec.Command("cp", d.simulDir+"/"+"mainchain.db", d.deployDir).Run()
	if err != nil {
		log.Fatal("error copying mainchainbc.xlsx: ", err)
	}
	err = exec.Command("cp", d.simulDir+"/"+"sidechainbc.xlsx", d.deployDir).Run()
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

	err = exec.Command("cp", d.simulDir+"/"+"simul.go", d.deployDir).Run()
	if err != nil {
		log.Fatal("error copying chainBoost.toml-file:", d.simulDir, d.Simulation+".toml", d.deployDir, err)
	}

	// Copying build-files to deploy-directory
	build, _ := ioutil.ReadDir(d.buildDir)
	for _, file := range build {
		err = exec.Command("cp", d.buildDir+"/"+file.Name(), d.deployDir).Run()
		if err != nil {
			log.Fatal("error copying build-file:", d.buildDir, file.Name(), d.deployDir, err)
		}
	}

	// Copy everything over to research-lab's gateway server
	log.LLvl3("Copying over to", d.Login, "@", d.Host)

	// todo: it works with out id_rsa now but I am not sure how I am authenticated to the gateway, will I need it or not!, I will keep it for now
	SSHString := "ssh -i '/Users/Coder/.ssh/id_rsa'"
	//ToDoCoder: fix this later
	err = Rsync(d.Login, d.Host, SSHString, d.deployDir+"/", "~/remote/")
	if err != nil {
		log.Fatal(err)
	}
	log.LLvl1("Done copying")

	return nil
}

// Start contacts the ResearchLabab server to run the simulation
func (d *ResearchLabab) Start(args ...string) error {
	d.started = true
	//----------
	// ToDoCoder: let's not call ./user locally
	// go func() {
	// 	err := SSHRunStdout(d.Login, d.Host, "cd remote; ./users -suite="+d.Suite)
	// 	if err != nil {
	// 		log.LLvl1(err)
	// 	}
	// 	d.sshCss <- "finished"
	// }()

	return nil
}

// Wait for the process to finish
func (d *ResearchLabab) Wait() error {
	wait, err := time.ParseDuration(d.RunWait)
	if err != nil || wait == 0 {
		wait = 600 * time.Second
		err = nil
	}
	if d.started {
		log.LLvl1("Simulation is started")
		select {
		case msg := <-d.sshCss:
			if msg == "finished" {
				log.LLvl1("Received finished-message, not killing users")
				return nil
			}
			log.LLvl1("Received out-of-line message", msg)
		case <-time.After(wait):
			log.LLvl1("Quitting after waiting", wait)
			d.started = false
		}
		d.started = false
	}
	return nil
}

// Checks whether host, login and project are defined. If any of them are missing, it will
// ask on the command-line.
// For the login-variable, it will try to set up a connection to d.Host and copy over the
// public key for a more easy communication
func (d *ResearchLabab) loadAndCheckResearchLababVars() {
	ResearchLab := ResearchLabab{}
	err := onet.ReadTomlConfig(&ResearchLab, "ResearchLab.toml")
	d.Host, d.Login, d.Project, d.Experiment = ResearchLab.Host, ResearchLab.Login, ResearchLab.Project, ResearchLab.Experiment

	if err != nil {
		log.LLvl1("Couldn't read config-file - asking for default values")
	}

	if d.Host == "" {
		d.Host = readString("Please enter the hostname of ResearchLabab", "research-lab-url:30")
	}

	login, err := user.Current()
	log.ErrFatal(err)

	if d.Login == "" {
		d.Login = readString("Please enter the login-name on "+d.Host, login.Username)
	}

	if d.Project == "" {
		d.Project = readString("Please enter the project on ResearchLabab", "SAFER")
	}

	if d.Experiment == "" {
		d.Experiment = readString("Please enter the Experiment on "+d.Project, "Dissent-CS")
	}

	onet.WriteTomlConfig(*d, "ResearchLab.toml")
}

// Shows a messages and reads in a string, eventually returning a default (dft) string
func readString(msg, dft string) string {
	log.LLvl1("%s [%s]:", msg, dft)

	reader := bufio.NewReader(os.Stdin)
	strnl, _ := reader.ReadString('\n')
	str := strings.TrimSpace(strnl)
	if str == "" {
		return dft
	}
	return str
}

const simulConnectionsConf = `
# This is for the onet-ResearchLabab testbed, which can use up an awful lot of connections

* soft nofile 128000
* hard nofile 128000
`

// Write the hosts.txt file automatically
// from project name and number of servers
// func (d *ResearchLabab) createHosts() {
// 	// Query ResearchLabab's API for servers
// 	log.LLvl1("Querying ResearchLabab's API to retrieve server names and addresses")
// 	command := fmt.Sprintf("/usr/testbed/bin/expinfo -l -e %s,%s", d.Project, d.Experiment)
// 	apiReply, err := SSHRun(d.Login, d.Host, command)
// 	if err != nil {
// 		log.Fatal("Error while querying ResearchLabab:", err)
// 	}
// 	log.ErrFatal(d.parseHosts(string(apiReply)))
// }
// func (d *ResearchLabab) parseHosts(str string) error {
// 	// Get the link-information, which is the second block in `expinfo`-output
// 	infos := strings.Split(str, "\n\n")
// 	if len(infos) < 2 {
// 		return xerrors.New("didn't recognize output of 'expinfo'")
// 	}
// 	linkInfo := infos[1]
// 	// Test for correct version in case the API-output changes
// 	if !strings.HasPrefix(linkInfo, "Virtual Lan/Link Info:") {
// 		return xerrors.New("didn't recognize output of 'expinfo'")
// 	}
// 	linkLines := strings.Split(linkInfo, "\n")
// 	if len(linkLines) < 5 {
// 		return xerrors.New("didn't recognice output of 'expinfo'")
// 	}
// 	nodes := linkLines[3:]
// 	d.Phys = []string{}
// 	d.Virt = []string{}
// 	names := make(map[string]bool)
// 	for i, node := range nodes {
// 		if i%2 == 1 {
// 			continue
// 		}
// 		matches := strings.Fields(node)
// 		if len(matches) != 6 {
// 			return xerrors.New("expinfo-output seems to have changed")
// 		}
// 		// Convert client-0:0 to client-0
// 		name := strings.Split(matches[1], ":")[0]
// 		ip := matches[2]
// 		fullName := fmt.Sprintf("%s.%s.%s.isi.ResearchLabab.net", name, d.Experiment, d.Project)
// 		log.LLvl1("Discovered", fullName, "on ip", ip)
// 		if _, exists := names[fullName]; !exists {
// 			d.Phys = append(d.Phys, fullName)
// 			d.Virt = append(d.Virt, ip)
// 			names[fullName] = true
// 		}
// 	}
// 	log.LLvl1("Physical:", d.Phys)
// 	log.LLvl1("Internal:", d.Virt)
// 	return nil
// }
