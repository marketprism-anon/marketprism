package main

import (
	"errors"
	"fmt"
	"os"
	"os/signal"
	"os/user"
	"path"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/briandowns/spinner"
	"github.com/povsister/scp"
	"github.com/withmandala/go-log"
	"golang.org/x/crypto/ssh"
)

type NetworkConfig struct {
	Servers      []string
	RemoteFolder string
	Files        []string
	Executable   string
	Outputs      []string
	User         string
	Debug        int
}

type ExperimentConfig struct {
	Simulation               string
	Suite                    string
	MCRoundDuration          int
	PercentageTxPay          int
	MainChainBlockSize       int
	SideChainBlockSize       int
	SectorNumber             int
	NumberOfPayTXsUpperBound int
	SimulationRounds         int
	SimulationSeed           int
	//-- bls cosi
	NbrSubTrees                      int
	Threshold                        int
	SCRoundDuration                  int
	CommitteeWindow                  int
	MCRoundPerEpoch                  int
	SimState                         int
	NumberOfActiveContractsPerServer int
	PayPercentOfTransactions         float64
}

type FQSSHClient struct {
	client *ssh.Client
	host   string
}

type SSHSession ssh.Session

func FileCombinedOutput(s *ssh.Session, cmd string, filepath string) error {
	if s.Stdout != nil {
		return errors.New("ssh: Stdout already set")
	}
	if s.Stderr != nil {
		return errors.New("ssh: Stderr already set")
	}
	b, err := os.OpenFile(filepath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer b.Close()
	s.Stdout = b
	s.Stderr = b
	err = s.Run(cmd)
	return err
}

// SSH and SCP Functions

/// Create an SSH Session from the client
func makeSSHSession(client *ssh.Client) (*ssh.Session, error) {
	return client.NewSession()
}

/// Create an SSH Client to a specific host
func createSSHConnection(host string, userName string) (*FQSSHClient, error) {
	logger.Info("Creating SSH Connection to: ", host)
	usr, _ := user.Current()
	dir := usr.HomeDir

	key, err := os.ReadFile(path.Join(dir, ".ssh/id_rsa"))
	if err != nil {
		return nil, err
	}

	// Create the Signer for this private key.
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		return nil, err
	}

	sshConfig := &ssh.ClientConfig{
		User: userName,
		Auth: []ssh.AuthMethod{ssh.PublicKeys(signer)},
	}

	sshConfig.HostKeyCallback = ssh.InsecureIgnoreHostKey()

	client, err := ssh.Dial("tcp", host+":22", sshConfig)
	if err != nil {
		return nil, err
	}
	var sshClient *FQSSHClient
	sshClient = new(FQSSHClient)
	sshClient.client = client
	sshClient.host = host
	return sshClient, nil
}

/// Create an SCP Client to transfer files around
func createScpClient(client *ssh.Client) (*scp.Client, error) {
	return scp.NewClientFromExistingSSH(client, &scp.ClientOption{})
}

/// Run CLI Commands over SSH
func runCommand(client FQSSHClient, cmd string) error {
	logger.Info("Executing(Remote): ", cmd)
	session, err := makeSSHSession(client.client)
	if err != nil {
		return err
	}
	destination := fmt.Sprintf("%s/stdout.txt", client.host)
	err = FileCombinedOutput(session, cmd, destination)
	return err
}

/// Upload Files (Copy File from local to remote)
func uploadFile(client *scp.Client, localPath string, remotePath string) error {
	logger.Info("Uploading ", localPath, " to Remote ", remotePath)
	return client.CopyFileToRemote(localPath, remotePath, &scp.FileTransferOption{})
}

/// Download files (Copy file from remote to local)
func downloadFile(client *scp.Client, remotePath string, localPath string) error {
	logger.Debug("Downloading ", localPath, " from Remote ", remotePath)
	return client.CopyFileFromRemote(remotePath, localPath, &scp.FileTransferOption{})
}

//Experiment functions

/// Terminates one instance of the simulation by sending SIGINT
func closeOneExperiment(client FQSSHClient) {
	runCommand(client, "pkill -9 simul")
}

/// Terminates all the instances of the simulation by sending SIGINT
func forceCloseExperiments(clients []FQSSHClient) {
	for _, client := range clients {
		closeOneExperiment(client)
	}
}

/// Runs the experiment in the remote virtual machines then copies the result back into the local
func runExperiments(sshClients []FQSSHClient, netConf *NetworkConfig) {
	// Start Running the experiments.
	s := spinner.New(spinner.CharSets[38], 100*time.Millisecond) // Build our new spinner
	s.Prefix = "Running the experiment, please wait: "
	s.Start()
	var waitGroup sync.WaitGroup
	var successCount uint64
	var failureCount uint64
	for _, client := range sshClients {
		waitGroup.Add(1)
		go func(client FQSSHClient) {
			failure := false
			defer waitGroup.Done()
			expArgs := fmt.Sprintf(" -address=%s -simul=ChainBoost -suite=bn256.adapter -platform=Researchlabab -debug=%d",
				client.host, netConf.Debug)
			cmd := fmt.Sprintf("cd ~/%s && ./%s %s", netConf.RemoteFolder, netConf.Executable, expArgs)
			err := runCommand(client, cmd)
			if err != nil {
				logger.Errorf("Host %s: %s", client.host, err)
				failure = true
			}
			for _, output := range netConf.Outputs {
				destination := fmt.Sprintf("%s/%s", client.host, output)
				source := fmt.Sprintf("%s/%s", netConf.RemoteFolder, output)
				scpClient, _ := createScpClient(client.client)
				downloadFile(scpClient, source, destination)
			}
			if failure {
				atomic.AddUint64(&failureCount, 1)
			} else {
				atomic.AddUint64(&successCount, 1)
			}
		}(client)
	}
	waitGroup.Wait()
	s.Stop()
	logger.Info("All experiments are over: Total=", len(sshClients), " Success=", successCount, " Failure=", failureCount)
}

// Executable Helper Functions

/// Prints the command used to run this exe
func printHelp(name string) {
	fmt.Printf("usage: %s <path/to/NetworkConfig.toml>", name)
}

/// Reads the experimentation configuration into this program (will be useful when we enable simul to take CLI Args)
func openExperimentConfig(path string) (*ExperimentConfig, error) {
	logger.Info("Opening the Configuration for the experiment at ", path)
	var conf ExperimentConfig
	conffile, err := os.ReadFile(path)
	config := string(conffile)
	if err != nil {
		return nil, err
	}
	_, err = toml.Decode(config, &conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}

/// Takes the output of uname -sm and returns it as GOOS and GOARCH (helpful for multiarch deploy)
func getOSAndArch(line string) (string, string, error) {
	args := strings.Split(line, " ")
	if args[1] == "x86_64" {
		args[1] = "amd64"
	}
	return strings.ToLower(args[0]), args[1], nil
}

/// Builds the network configuration from the TOML file
func openNetworkConfig(path string) (*NetworkConfig, error) {
	logger.Info("Opening the Network Configuration at ", path)
	var conf NetworkConfig
	conffile, err := os.ReadFile(path)
	config := string(conffile)
	if err != nil {
		return nil, err
	}
	_, err = toml.Decode(config, &conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}

/// Writes a string to file (helpful to write the simul stdout into a text file for our debug later)
func writeToFile(destination string, data string) {
	f, err := os.Create(destination)
	if err != nil {
		logger.Fatal(err)
	}
	f.WriteString(data)
	f.Close()
}

// Main Function for this exe

var logger *log.Logger

func main() {
	// Initial Setup

	logger = log.New(os.Stderr).WithColor()
	if len(os.Args) != 2 {
		printHelp(os.Args[0])
		os.Exit(0)
	}

	// Read Config Files
	/*expConf, err := openExperimentConfig(os.Args[1])
	    if err != nil {
	    logger.Fatal(err)
	}*/

	netConf, err := openNetworkConfig(os.Args[1])
	if err != nil {
		logger.Fatal(err)
	}

	//Create an array of SSH Clients
	sshClients := make([]FQSSHClient, len(netConf.Servers))

	// Handle Control+C
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigs
		forceCloseExperiments(sshClients)
	}()

	// Create SSH Connection and SCP Clients
	for i, server := range netConf.Servers {
		os.RemoveAll(server)
		os.Mkdir(server, 0750)
		sshClient, err := createSSHConnection(server, netConf.User)
		if err != nil {
			logger.Fatalf("Server %s: %s", server, err)
		}
		sshClients[i] = *sshClient
		defer sshClients[i].client.Close()

		closeOneExperiment(sshClients[i])
		time.Sleep(5 * time.Second)

		scpClient, err := createScpClient(sshClients[i].client)
		if err != nil {
			logger.Fatalf("Server %s: %s", server, err)
		}
		defer scpClient.Close()

		cmd := fmt.Sprintf("mkdir -p ~/%s", netConf.RemoteFolder)
		err = runCommand(sshClients[i], cmd)
		if err != nil {
			logger.Fatalf("Server %s: %s", server, err)
		}

		// Copy Files to the Remote
		for _, file := range netConf.Files {
			destination := fmt.Sprintf("%s/%s", netConf.RemoteFolder, file)
			err = uploadFile(scpClient, file, destination)
			if err != nil {
				logger.Fatalf("Server %s: %s", server, err)
			}

			if file == netConf.Executable {
				cmd := fmt.Sprintf("chmod +x %s", destination)
				err := runCommand(sshClients[i], cmd)
				if err != nil {
					logger.Fatalf("Server %s: %s", server, err)
				}
			}
		}
	}

	// Run the experiment
	runExperiments(sshClients, netConf)

	// Stop Running the experiments.
	forceCloseExperiments(sshClients)
	os.Exit(0)
}
