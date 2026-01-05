package platform

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"

	"github.com/BurntSushi/toml"
	MainAndSideChain "github.com/chainBoostScale/ChainBoost/MainAndSideChain"
	"github.com/chainBoostScale/ChainBoost/MainAndSideChain/BLSCoSi"
	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"github.com/chainBoostScale/ChainBoost/onet/network"
	"github.com/chainBoostScale/ChainBoost/vrf"
	"go.dedis.ch/kyber/v3/pairing"
	"go.uber.org/atomic"
	"golang.org/x/sync/errgroup"
	"golang.org/x/xerrors"
)

const ChainBoostDefaultJoinTimeOut = 100

type simulInit struct{}
type simulInitDone struct{}

var batchSize = 1000

var abspath, _ = os.Getwd()

// Simulate starts the server and will setup the protocol.
// adding some other system-wide configurations
func Simulate(configurations []Config,
	suite, serverAddress, simul string) error {
	scs, err := onet.LoadSimulationConfig(suite, ".", serverAddress)
	if err != nil {
		// We probably are not needed
		log.LLvl1(err, serverAddress)
		return err
	}
	sims := make([]onet.Simulation, len(scs))
	simulInitID := network.RegisterMessage(simulInit{})
	simulInitDoneID := network.RegisterMessage(simulInitDone{})
	var rootSC *onet.SimulationConfig
	var rootSim onet.Simulation
	// having a waitgroup so the binary stops when all servers are closed
	var wgServer, wgSimulInit sync.WaitGroup
	var ready = make(chan bool)
	if len(scs) > 0 {
		cfg := &conf{}
		_, err := toml.Decode(scs[0].Config, cfg)
		if err != nil {
			return xerrors.New("error while decoding config: " + err.Error())
		}
	}
	for i, sc := range scs {
		// Starting all servers for that server
		server := sc.Server

		log.Lvl4("in function simulate: ", serverAddress, "Starting server", server.ServerIdentity.Address)
		// Launch a server and notifies when it's done
		wgServer.Add(1)
		go func(c *onet.Server) {
			ready <- true
			defer wgServer.Done()
			log.Lvl2(": starting a server:", c.ServerIdentity.Address)
			c.Start()
			log.LLvl1(serverAddress, "Simulation closed server", c.ServerIdentity)
		}(server)

		// wait to be sure the goroutine started
		<-ready
		log.Lvl5("simul flag value is:", simul)
		sim, err := onet.NewSimulation(simul, sc.Config)
		if err != nil {
			return xerrors.New("couldn't create new simulation: " + err.Error())
		}
		sims[i] = sim
		// Need to store sc in a tmp-variable so it's correctly passed
		// to the Register-functions.
		scTmp := sc
		server.RegisterProcessorFunc(simulInitID, func(env *network.Envelope) error {
			// The node setup must be done in a goroutine to prevent the connection
			// from the root to this node to stale forever.
			go func() {
				err = sim.Node(scTmp)
				log.ErrFatal(err)
				_, err := scTmp.Server.Send(env.ServerIdentity, &simulInitDone{})
				log.ErrFatal(err)
			}()
			return nil
		})

		server.RegisterProcessorFunc(simulInitDoneID, func(env *network.Envelope) error {
			wgSimulInit.Done()
			return nil
		})
		if server.ServerIdentity.ID.Equal(sc.Tree.Root.ServerIdentity.ID) {
			log.LLvl5(serverAddress, "will start protocol")
			rootSim = sim
			rootSC = sc
		}
	}

	var simError error
	var ChainBoostProtocol *MainAndSideChain.ChainBoost

	if rootSim != nil {
		for run, configuration := range configurations {
			log.Lvl1("====================================================================================================================================================")
			log.Lvl1("======= Starting run number: ", run+1)
			log.Lvl1("====================================================================================================================================================")
			// creating new db file for this run, will be renamed to contain run number later

			// If this cothority has the root-server, it will start the simulation
			// ---------------------------------------------------------------
			//              ---------- BLS CoSi protocol -------------
			// ---------------------------------------------------------------
			// initialization of committee members in side chain
			committeeNodes := rootSC.Tree.Roster.List[:configuration.CommitteeWindow-1]
			committeeNodes = append([]*network.ServerIdentity{rootSC.Tree.List()[configuration.CommitteeWindow].ServerIdentity}, committeeNodes...)
			committee := onet.NewRoster(committeeNodes)
			var x = *rootSC.Tree.List()[configuration.CommitteeWindow]
			x.RosterIndex = 0
			BlsCosiSubTrees, _ := BLSCoSi.NewBlsProtocolTree(onet.NewTree(committee, &x), configuration.NbrSubTrees)
			pi, err := rootSC.Overlay.CreateProtocol("bdnCoSiProto", BlsCosiSubTrees[0], onet.NilServiceID)
			if err != nil {
				return xerrors.New("couldn't create protocol: " + err.Error())
			}
			cosiProtocol := pi.(*BLSCoSi.BlsCosi)
			cosiProtocol.CreateProtocol = rootSC.Overlay.CreateProtocol
			/*
			   cosiProtocol.CreateProtocol = rootService.CreateProtocol //: it used to be initialized by this function call
			   params from config file: */
			cosiProtocol.Threshold = configuration.Threshold
			if configuration.NbrSubTrees > 0 {
				err := cosiProtocol.SetNbrSubTree(configuration.NbrSubTrees)
				if err != nil {
					return err
				}
			}
			// ---------------------------------------------------------------
			//              ------   ChainBoost protocol  ------
			// ---------------------------------------------------------------
			// calling CreateProtocol() => calling Dispatch()
			p, err := rootSC.Overlay.CreateProtocol("ChainBoost", rootSC.Tree, onet.NilServiceID)
			if err != nil {
				return xerrors.New("couldn't create protocol: " + err.Error())
			}
			ChainBoostProtocol = p.(*MainAndSideChain.ChainBoost)
			// passing our system-wide configurations to our protocol
			ChainBoostProtocol.PercentageTxPay = configuration.PercentageTxPay
			ChainBoostProtocol.MCRoundDuration = configuration.MCRoundDuration
			ChainBoostProtocol.MainChainBlockSize = configuration.MainChainBlockSize
			ChainBoostProtocol.SideChainBlockSize = configuration.SideChainBlockSize
			ChainBoostProtocol.SectorNumber = configuration.SectorNumber
			ChainBoostProtocol.NumberOfPayTXsUpperBound = configuration.NumberOfPayTXsUpperBound
			ChainBoostProtocol.NumberOfActiveContractsPerServer = configuration.NumberOfActiveContractsPerServer
			ChainBoostProtocol.SimulationRounds = configuration.SimulationRounds
			ChainBoostProtocol.SimulationSeed = configuration.SimulationSeed
			ChainBoostProtocol.NbrSubTrees = configuration.NbrSubTrees
			ChainBoostProtocol.Threshold = configuration.Threshold
			ChainBoostProtocol.SCRoundDuration = configuration.SCRoundDuration
			ChainBoostProtocol.CommitteeWindow = configuration.CommitteeWindow
			ChainBoostProtocol.MCRoundPerEpoch = configuration.MCRoundPerEpoch
			ChainBoostProtocol.SCRoundPerEpoch = configuration.SCRoundPerEpoch
			ChainBoostProtocol.SimState = configuration.SimState
			ChainBoostProtocol.StoragePaymentEpoch = configuration.StoragePaymentEpoch
			ChainBoostProtocol.PayPercentOfTransactions = configuration.PayPercentOfTransactions
			ChainBoostProtocol.Sidechains = configuration.Sidechains
			ChainBoostProtocol.CountNodes = configuration.Nodes
			ChainBoostProtocol.FaultyContractsRate = configuration.FaultyContractsRate
			log.Lvl1("passing our system-wide configurations to the protocol",
				"\n  PercentageTxPay: ", configuration.PercentageTxPay,
				"\n  MCRoundDuration: ", configuration.MCRoundDuration,
				"\n MainChainBlockSize: ", configuration.MainChainBlockSize,
				"\n SideChainBlockSize: ", configuration.SideChainBlockSize,
				"\n SectorNumber: ", configuration.SectorNumber,
				"\n NumberOfPayTXsUpperBound: ", configuration.NumberOfPayTXsUpperBound,
				"\n NumberOfActiveContractsPerServer: ", configuration.NumberOfActiveContractsPerServer,
				"\n SimulationRounds: ", configuration.SimulationRounds,
				"\n SimulationSeed of: ", configuration.SimulationSeed,
				"\n nbrSubTrees of: ", configuration.NbrSubTrees,
				"\n threshold of: ", configuration.Threshold,
				"\n SCRoundDuration: ", configuration.SCRoundDuration,
				"\n CommitteeWindow: ", configuration.CommitteeWindow,
				"\n MCRoundPerEpoch: ", configuration.MCRoundPerEpoch,
				"\n SimState: ", configuration.SimState,
				"\n StoragePaymentEpoch: ", configuration.StoragePaymentEpoch,
				"\n PayPercentOfTransactions", configuration.PayPercentOfTransactions,
			)
			ChainBoostProtocol.BlsCosi = atomic.Pointer[BLSCoSi.BlsCosi]{} //.Store(&cosiProtocol)
			ChainBoostProtocol.BlsCosi.Store(cosiProtocol)
			// --------------------------------------------------------------
			ChainBoostProtocol.JoinedWG.Add(len(rootSC.Tree.Roster.List))
			ChainBoostProtocol.JoinedWG.Done()
			// ----

			ChainBoostProtocol.CalledWG.Add(len(rootSC.Tree.Roster.List))
			ChainBoostProtocol.CalledWG.Done()

			//Exit when first error occurs in a goroutine: https://stackoverflow.com/a/61518963
			//Pass arguments into errgroup: https://forum.golangbridge.org/t/pass-arguments-to-errgroup-within-a-loop/19999
			errs, _ := errgroup.WithContext(context.Background())

			// --------------------------------------------------------------
			// Inviting other nodes to joinn the protocol, in parralel!
			for i := 0; i <= len(rootSC.Tree.List())/batchSize; i++ {
				id := i
				errs.Go(func() error {
					return sendMsgToJoinChainBoostProtocol(id, ChainBoostProtocol, rootSC)
				})
			}
			if err := errs.Wait(); err != nil {
				return xerrors.New("Error from the initial simulation joining  phase: " + err.Error())
			}
			ChainBoostProtocol.CalledWG.Wait()
			log.LLvl1("Wait is passed which means all nodes have recieved the invitatioon msg")
			// --------------------------------------------------------------
			go func() {
				err := ChainBoostProtocol.DispatchProtocol()
				if err != nil {
					log.Lvl1("protocol dispatch calling error: " + err.Error())
				}
			}()
			ChainBoostProtocol.Start()
			px := <-ChainBoostProtocol.DoneRootNode
			log.LLvl1(rootSC.Server.ServerIdentity.Address, ": Final result is", px)
			time.Sleep(time.Second * 10) // wait for thread to finish writing to the db
			break
			/*mcFilename := fmt.Sprintf(abspath+"/mainchain_%d.db", run)
			scFilename := fmt.Sprintf(abspath+"/sidechain_%d.db", run)
			os.Rename(abspath+"/mainchain.db", mcFilename)
			os.Rename(abspath+"/sidechain.db", scFilename)*/
		}

		// what is this for?
		/*err = ioutil.WriteFile(abspath+"/sidechain.db", inputSideChain, 0644)
		if err != nil {
			log.Fatal(err)
		}*/

		// --------------------------------------------------------------
		log.LLvl1(rootSC.Server.ServerIdentity.Address, ": close all other nodes!")

		for _, child := range rootSC.Tree.List() {
			err := ChainBoostProtocol.SendTo(child, &MainAndSideChain.SimulationDone{
				IsSimulationDone: true,
			})
			if err != nil {
				log.Lvl1(ChainBoostProtocol.Info(), "couldn't send ChainBoostDone msg to child", child.Name(), "with err:", err)
			}
		}

		// Test if all ServerIdentities are used in the tree, else we'll run into
		// troubles with CloseAll
		if !rootSC.Tree.UsesList() {
			log.Error("The tree doesn't use all ServerIdentities from the list!\n" +
				"This means that the CloseAll will fail and the experiment never ends!")
		}
		// Recreate a tree out of the original roster, to be sure all nodes are included and
		// that the tree is easy to close.
		closeTree := rootSC.Roster.GenerateBinaryTree()
		piC, err := rootSC.Overlay.CreateProtocol("CloseAll", closeTree, onet.NilServiceID)
		if err != nil {
			return xerrors.New("couldn't create closeAll protocol: " + err.Error())
		}
		log.Lvl1("close all nodes by  the returned root node at the end of simulation")
		piC.Start()
	}

	log.LLvl1(serverAddress, scs[0].Server.ServerIdentity, "is waiting for all servers to close")
	wgServer.Wait()
	os.Exit(0)
	log.LLvl1(serverAddress, "has all servers closed")

	// Give a chance to the simulation to stop the servers and clean up but returns the simulation error anyway.
	if simError != nil {
		return xerrors.New("error from simulation run: " + simError.Error())
	}
	log.LLvl1("Simulate is returning")
	return nil
}

type conf struct {
	IndividualStats string
}

//  added

func init() {
	network.RegisterMessage(MainAndSideChain.HelloChainBoost{})
	network.RegisterMessage(MainAndSideChain.Joined{})
	network.RegisterMessage(MainAndSideChain.NewRound{})
	network.RegisterMessage(MainAndSideChain.NewLeader{})
	network.RegisterMessage(MainAndSideChain.LtRSideChainNewRound{})
	network.RegisterMessage(MainAndSideChain.RtLSideChainNewRound{})
	network.RegisterMessage(MainAndSideChain.LtRSideChainNewRoundChain2{})
	network.RegisterMessage(MainAndSideChain.RtLSideChainNewRoundChain2{})
	network.RegisterMessage(MainAndSideChain.LtRSideChainNewRoundChain3{})
	network.RegisterMessage(MainAndSideChain.RtLSideChainNewRoundChain3{})
	network.RegisterMessage(MainAndSideChain.LtRSideChainNewRoundChain4{})
	network.RegisterMessage(MainAndSideChain.RtLSideChainNewRoundChain4{})
	onet.GlobalProtocolRegister("ChainBoost", NewChainBoostProtocol)
}

func NewChainBoostProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {

	pi, err := n.Overlay.CreateProtocol("bdnCoSiProto", n.Tree(), onet.NilServiceID)
	if err != nil {
		log.LLvl1("couldn't create protocol: " + err.Error())
	}
	cosiProtocol := pi.(*BLSCoSi.BlsCosi)
	cosiProtocol.CreateProtocol = n.Overlay.CreateProtocol
	cosiProtocol.TreeNodeInstance = n

	bz := &MainAndSideChain.ChainBoost{
		TreeNodeInstance: n,
		Suite:            pairing.NewSuiteBn256(),
		DoneRootNode:     make(chan bool, 1),
		MCRoundNumber:    *atomic.NewInt64(1),
		//CommitteeWindow:    10,
		SCRoundNumber:       make(map[string]*atomic.Int64),
		FirstQueueWait:      0,
		SideChainQueueWait:  0,
		FirstSCQueueWait:    0,
		SecondQueueWait:     0,
		SimulationRounds:    0,
		BlsCosiStarted:      false,
		BlsCosi:             atomic.Pointer[BLSCoSi.BlsCosi]{},
		SimState:            1, // 1: just the main chain - 2: main chain plus side chain = chainBoost
		StoragePaymentEpoch: 0, // 0:  after contracts expires, n: settllement after n epoch
	}
	bz.SCRoundNumber["por"] = atomic.NewInt64(1)
	bz.SCRoundNumber["por2"] = atomic.NewInt64(1)
	bz.SCRoundNumber["dispute"] = atomic.NewInt64(1)
	bz.SCRoundNumber["marketmatch"] = atomic.NewInt64(1)
	bz.BlsCosi.Store(cosiProtocol)
	if err := n.RegisterChannel(&bz.HelloChan); err != nil {
		return bz, err
	}
	if err := n.RegisterChannelLength(&bz.JoinedWGChan, len(bz.Tree().List())); err != nil {
		log.Error("Couldn't register channel:    ", err)
	}
	if err := n.RegisterChannel(&bz.MainChainNewRoundChan); err != nil {
		return bz, err
	}
	if err := n.RegisterChannelLength(&bz.MainChainNewLeaderChan, len(bz.Tree().List())); err != nil {
		log.Error("Couldn't register channel:    ", err)
	}
	if err := n.RegisterChannel(&bz.ChainBoostDone); err != nil {
		return bz, err
	}

	//ToDoCoder: what exactly does this sentence do?! do we need it?! nil array vs empty array?
	bz.CommitteeNodesTreeNodeID1 = make([]onet.TreeNodeID, bz.CommitteeWindow)
	bz.CommitteeNodesTreeNodeID2 = make([]onet.TreeNodeID, bz.CommitteeWindow)
	bz.CommitteeNodesTreeNodeID3 = make([]onet.TreeNodeID, bz.CommitteeWindow)
	bz.CommitteeNodesTreeNodeID4 = make([]onet.TreeNodeID, bz.CommitteeWindow)
	bz.SCTools = map[string]MainAndSideChain.SideChain{
		"por": {
			SummPoRTxs:    make(map[int]int),
			WgSyncScRound: &MainAndSideChain.WaitGroupCount{},
		},
		"por2": {
			SummPoRTxs:    make(map[int]int),
			WgSyncScRound: &MainAndSideChain.WaitGroupCount{},
		},
		"dispute": {
			SummPoRTxs:    make(map[int]int),
			WgSyncScRound: &MainAndSideChain.WaitGroupCount{},
		},
		"marketmatch": {
			SummPoRTxs:    make(map[int]int),
			WgSyncScRound: &MainAndSideChain.WaitGroupCount{},
		},
	}

	bz.SCSig = make(map[string]BLSCoSi.BlsSignature)
	// bls key pair for each node for VRF
	// ToDo: temp commented
	// do I need to bring this seed from config? check what it is used for?
	rand.Seed(int64(bz.TreeNodeInstance.Index()))
	seed := make([]byte, 32)
	rand.Read(seed)
	tempSeed := (*[32]byte)(seed[:32])
	log.Lvlf5(":debug:seed for the VRF is:", seed, "the tempSeed value is:", tempSeed)
	_, bz.ECPrivateKey = vrf.VrfKeygenFromSeedGo(*tempSeed)
	log.Lvlf5(":debug: the ECprivate Key is:", bz.ECPrivateKey)
	// --------------------------------------- blscosi -------------------
	if err := n.RegisterChannel(&bz.RtLSideChainNewRoundChan); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.LtRSideChainNewRoundChan); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.RtLSideChainNewRoundChanChain2); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.LtRSideChainNewRoundChanChain2); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.RtLSideChainNewRoundChanChain3); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.LtRSideChainNewRoundChanChain3); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.RtLSideChainNewRoundChanChain4); err != nil {
		return bz, err
	}
	if err := n.RegisterChannel(&bz.LtRSideChainNewRoundChanChain4); err != nil {
		return bz, err
	}
	return bz, nil
}
func sendMsgToJoinChainBoostProtocol(i int, ChainBoostProtocol *MainAndSideChain.ChainBoost, rootSC *onet.SimulationConfig) error {
	var err error
	var end int
	begin := i * batchSize
	if len(rootSC.Tree.List()) <= i*batchSize+batchSize {
		end = len(rootSC.Tree.List())
	} else {
		end = i*batchSize + batchSize
	}
	for _, child := range rootSC.Tree.List()[begin:end] {
		if child != ChainBoostProtocol.TreeNode() {
			timeout := ChainBoostDefaultJoinTimeOut * time.Minute
			start := time.Now()
			for time.Since(start) < timeout {
				err := ChainBoostProtocol.SendTo(child, &MainAndSideChain.HelloChainBoost{
					SimulationRounds:                 ChainBoostProtocol.SimulationRounds,
					PercentageTxPay:                  ChainBoostProtocol.PercentageTxPay,
					MCRoundDuration:                  ChainBoostProtocol.MCRoundDuration,
					MainChainBlockSize:               ChainBoostProtocol.MainChainBlockSize,
					SideChainBlockSize:               ChainBoostProtocol.SideChainBlockSize,
					SectorNumber:                     ChainBoostProtocol.SectorNumber,
					NumberOfPayTXsUpperBound:         ChainBoostProtocol.NumberOfPayTXsUpperBound,
					NumberOfActiveContractsPerServer: ChainBoostProtocol.NumberOfActiveContractsPerServer,
					SimulationSeed:                   ChainBoostProtocol.SimulationSeed,
					// --------------------- bls cosi ------------------------
					NbrSubTrees:         ChainBoostProtocol.NbrSubTrees,
					Threshold:           ChainBoostProtocol.Threshold,
					CommitteeWindow:     ChainBoostProtocol.CommitteeWindow,
					SCRoundDuration:     ChainBoostProtocol.SCRoundDuration,
					MCRoundPerEpoch:     ChainBoostProtocol.MCRoundPerEpoch,
					SCRoundPerEpoch:     ChainBoostProtocol.SCRoundPerEpoch,
					SimState:            ChainBoostProtocol.SimState,
					StoragePaymentEpoch: ChainBoostProtocol.StoragePaymentEpoch,
					Sidechains:          ChainBoostProtocol.Sidechains,
					CountNodes:          ChainBoostProtocol.CountNodes,
					FaultyContractsRate: ChainBoostProtocol.FaultyContractsRate,
				})
				if err != nil {
					log.Lvl1(ChainBoostProtocol.Info(), "couldn't send HelloChainBoost msg to child", child.Name(), "with err:", err)
				} else {
					ChainBoostProtocol.CalledWG.Done()
					break
				}
			}

			if time.Since(start) >= timeout {
				err = fmt.Errorf("Attempt to send HelloChainBoost msg to child " + child.Name() + " timed out!")
				return err
			}
		}
	}

	return err
}
