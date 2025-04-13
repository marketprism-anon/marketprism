package main

import (
	"flag"
	"os"

	//"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	"runtime"
	"strconv"

	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	simul "github.com/chainBoostScale/ChainBoost/simulation"
	"github.com/chainBoostScale/ChainBoost/simulation/platform"
)

var kill = false

// suite is Ed25519 by default
//var suite string

// Initialize before 'init' so we can directly use the fields as parameters
// to 'Flag'
func init() {
	flag.BoolVar(&kill, "kill", false, "kill everything (and don't start anything)")
}

// ResearchLababUsers is called on the users.ResearchLabab.net-server and will:
// - copy the simulation-files to the server
// - start the simulation
func main() {
	//init with ResearchLab.toml
	ResearchLab := ResearchLabFromConfig()
	flag.Parse()
	// kill old processes
	var wg sync.WaitGroup
	re := regexp.MustCompile(" +")
	// hosts, err := exec.Command("/usr/testbed/bin/node_list", "-e", ResearchLab.Project+","+ResearchLab.Experiment).Output()
	// if err != nil {
	// 	log.Fatal("ResearchLabab experiment", ResearchLab.Project+"/"+ResearchLab.Experiment, "tttt seems not to be swapped in. Aborting.")
	// 	os.Exit(-1)
	// }
	//hosts := "ResearchLab-Url:30"
	//todo: change this list based on list of VMs or make it dynamic later
	// 192.168.3.221:22 192.168.3.222:22 192.168.3.223:22
	hosts := "192.168.3.220:22 192.168.3.221:22 192.168.3.222:22 192.168.3.223:22 192.168.3.224:22 192.168.3.225:22 192.168.3.226:22 192.168.3.227:22 192.168.3.228:22 192.168.3.229:22"
	hostsTrimmed := strings.TrimSpace(re.ReplaceAllString(string(hosts), " "))
	hostlist := strings.Split(hostsTrimmed, " ")
	doneHosts := make([]bool, len(hostlist))
	log.LLvl1("Found the following hosts:", hostlist)
	if kill {
		log.LLvl1("Cleaning up", len(hostlist), "hosts.")
	}

	for i, h := range hostlist {
		wg.Add(1)
		go func(i int, h string) {
			defer wg.Done()
			if kill {
				log.LLvl1("Cleaning up host", h, ".")
				//runSSH(h, "killall -9 simul scp 2>/simout >/simout")
				platform.SSHRunStdout("root", h, "killall -9 simul 2>/simout")
				//s := strings.Split(h, ":")
				//hs := s[0]
				time.Sleep(1 * time.Second)
				// Also kill all other process that start with "./" and are probably
				// locally started processes
				platform.SSHRunStdout("root", h, "pkill -9 -f '\\./' 2>/simout")
				time.Sleep(1 * time.Second)
				if log.DebugVisible() > 3 {
					log.LLvl1("Cleaning report:")
					_ = platform.SSHRunStdout("root", h, "ps aux")
				}
			} else {
				log.LLvl1("Coder: skipping: Setting the file-limit higher")
				//log.LLvl1("Setting the file-limit higher on", h)
				// Copy configuration file to make higher file-limits
				// err := platform.SSHRunStdout("root", h, "sudo cp remote/simul.conf /etc/security/limits.d")
				// if err != nil {
				// 	log.Fatal("Couldn't copy limit-file:", err)
				// } else {
				// 	log.Lvl1("successfully copied limit-file")
				// }
			}
			doneHosts[i] = true
		}(i, h)
	}
	cleanupChannel := make(chan string)
	go func() {
		wg.Wait()
		log.LLvl3("Done waiting")
		cleanupChannel <- "done"
	}()
	select {
	case msg := <-cleanupChannel:
		log.LLvl3("Received msg from cleanupChannel", msg)
	//todo now: is it a good wait out time for us?
	case <-time.After(time.Second * 20000):
		for i, m := range doneHosts {
			if !m {
				log.LLvl1("Missing host:", hostlist[i]) //, "- You should run")
				//log.LLvl1("/usr/testbed/bin/node_reboot", hostlist[i])
			}
		}
		log.Fatal("Didn't receive all replies while cleaning up - aborting.")
	}
	if kill {
		log.LLvl1("Only cleaning up - returning")
		return
	}
	//-------------------
	killing := false
	for i, phys := range ResearchLab.Phys {
		wg.Add(1)
		go func(phys, internal string) {
			defer wg.Done()
			// If PreScript is defined, run the appropriate script _before_ the simulation.
			if ResearchLab.PreScript != "" {
				log.LLvl1(": ResearchLab.PreScript running?")
				err := platform.SSHRunStdout("root", phys, "cd remote; sudo ./"+ResearchLab.PreScript+" ResearchLabab")
				if err != nil {
					log.Fatal("error deploying PreScript: ", err)
				}
			} else {
				log.LLvl1(": ResearchLab.PreScript is empty.")
			}
			// -----------------------------------------
			// Copy everything over
			log.Lvl3("Copying over to", phys)
			err := platform.SSHRunStdout("root", phys, "mkdir -p remote")
			if err != nil {
				log.Fatal(err)
			}
			err = platform.SSHRunStdout("root", phys, "touch simout")
			if err != nil {
				log.Fatal(err)
			}
			err = platform.Rsync("root", phys, "ssh", "/home/user/remote", "/root/")
			if err != nil {
				log.Fatal(err)
			}

			/* -------------------------------------
			PercentageTxPay
			MCRoundDuration       	//sec
			MainChainBlockSize  	//byte
			SideChainBlockSize
			SectorNumber
			NumberOfPayTXsUpperBound
			SimulationRounds
			SimulationSeed
			NbrSubTrees
			Threshold 			    //out of committee nodes
			SCRoundDuration
			CommitteeWindow		    //nodes
			MCRoundPerEpoch
			SimState
			*/

			args := " -address=" + internal +
				" -simul=" + ResearchLab.Simulation +
				//" -debug=" + strconv.Itoa(log.DebugVisible()) +
				" -Debug=" + strconv.Itoa(simul.Debug) +
				" -suite=" + simul.Suite

			log.Lvl1("Args is", args)
			err = platform.SSHRunStdout("root", phys, "cd remote; sudo ./simul "+args)
			// -----------------------------------------
			if err != nil && !killing {
				log.Lvl1("Error starting simul - will kill all others:", err, internal)
				killing = true
				cmd := exec.Command("kill", "-9", "-1")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				err := cmd.Run()
				if err != nil {
					log.Fatal("Couldn't killall listening threads:", err)
				} else {
					log.Lvl1("all listener on VM", internal, "are killed.")
				}
			}
			log.LLvl1("Finished with simul on", internal)
		}(phys, ResearchLab.Virt[i])
	}
	// wait for the servers to finish before stopping
	wg.Wait()
	//prox.Stop()
}

// Reads in the ResearchLabab-config and drops out if there is an error
func ResearchLabFromConfig(name ...string) *platform.ResearchLabab {
	d := &platform.ResearchLabab{}
	configName := "ResearchLab.toml"
	if len(name) > 0 {
		configName = name[0]
	}
	err := onet.ReadTomlConfig(d, configName)
	_, caller, line, _ := runtime.Caller(1)
	who := caller + ":" + strconv.Itoa(line)
	if err != nil {
		log.Fatal("Couldn't read config in", who, ":", err)
	}
	log.SetDebugVisible(d.Debug)
	return d
}

// Runs a command on the remote host and outputs an eventual error if debug level >= 3
func runSSH(host, cmd string) {
	if err := platform.SSHRunStdout("root", host, cmd); err != nil {
		log.Lvl1("Host", host, "got error", err.Error(), "while running", cmd)
	} else {
		log.Lvl1("Host", host, "cleaned up")
	}
}
