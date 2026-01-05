// ToDoCoder: it seems that simstate + some other config params aren't required to be sent to all nodes

package MainAndSideChain

import (
	"os"
	"sync"
	"time"

	"github.com/chainBoostScale/ChainBoost/MainAndSideChain/BLSCoSi"
	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"github.com/chainBoostScale/ChainBoost/por"
	"github.com/chainBoostScale/ChainBoost/vrf"
	"go.dedis.ch/kyber/v3/pairing"
	"go.uber.org/atomic"
	"golang.org/x/xerrors"
)

type WaitGroupCount struct {
	sync.WaitGroup
	count   atomic.Int64
	localWg sync.WaitGroup
}

func (wg *WaitGroupCount) Add(name string, delta int) {
	wg.localWg.Wait()
	wg.count.Add(int64(delta))
	log.LLvlf1(">>>> WG %s Incremented by, %d, Count %d\n", name, delta, wg.count.Load())
	wg.WaitGroup.Add(delta)
}

func (wg *WaitGroupCount) Done(name string) {
	wg.count.Sub(1)
	log.LLvlf1(">>>> WG %s Decremented by, %d, Count %d\n", name, 1, wg.count.Load())
	wg.WaitGroup.Done()
}

func (wg *WaitGroupCount) Wait(name string) {
	wg.localWg.Add(1)
	log.LLvlf1(">>>> WG %s Waiting Count %d\n", name, wg.count.Load())
	wg.WaitGroup.Wait()
	log.LLvlf1(">>>> WG %s Waiting Done %d\n", name, wg.count.Load())
	wg.localWg.Done()
}

func (wg *WaitGroupCount) GetCount() int {
	return int(wg.count.Load())
}

/* ----------------------------------------- TYPES -------------------------------------------------
------------------------------------------------------------------------------------------------  */

// Hello is sent down the tree from the root node, every node who gets it starts the protocol and send it to its children
type HelloChainBoost struct {
	SimulationRounds                 int
	PercentageTxPay                  int
	MCRoundDuration                  int
	SideChainBlockSize               int
	MainChainBlockSize               int
	SectorNumber                     int
	NumberOfPayTXsUpperBound         int
	NumberOfActiveContractsPerServer int
	SimulationSeed                   int
	SCRoundDuration                  int
	CommitteeWindow                  int
	MCRoundPerEpoch                  int
	SCRoundPerEpoch                  int
	// bls cosi config
	NbrSubTrees int
	Threshold   int
	// simulation
	SimState            int
	StoragePaymentEpoch int
	Sidechains          []string
	CountNodes          int

	FaultyContractsRate float64
}
type HelloChan struct {
	*onet.TreeNode
	HelloChainBoost
}

// Channel to notify nodes that the simulation has completed
type ChainBoostDone struct {
	*onet.TreeNode
	SimulationDone
}

type SimulationDone struct {
	IsSimulationDone bool
}

// joined is sent to root to let the root node that this node is joined to the simulation
type JoinedWGChan struct {
	*onet.TreeNode
	Joined
}
type Joined struct {
	IsJoined bool
}

// to let the  first leader run main chain protocol and ignore the rest
type MCLeader struct {
	HasLeader bool
	MCPLock   sync.Mutex
}

type CoSiQueue struct {
	atomic.Pointer[[]string]
}

func (q *CoSiQueue) enqueue(element string) {
	qq := *q.Load()
	qq = append(qq, element)
	q.Store(&qq)
}

func (q *CoSiQueue) dequeue() string {
	qq := *q.Load()
	element, qq := qq[0], qq[1:]
	q.Store(&qq)
	return element
}

// ChainBoost is the main struct that has required parameters for running both protocols ------------
type ChainBoost struct {
	// what if we could have two types of node structures: root node and simple nodes
	// then the simple nodes would be much lighter.
	// ToDoCoder: test it to communicate from a root node struct to simple node and vice versa
	// ToDoCoder: Lots of these items can be transfered into outside! or even removed from the structure

	// the node we are represented-in
	*onet.TreeNodeInstance
	ECPrivateKey vrf.VrfPrivkey

	ChainBoostDone chan ChainBoostDone
	// channel used to let all servers that the protocol has started
	HelloChan chan HelloChan
	// channel used by each round's leader to let all servers that a new round has come
	MainChainNewRoundChan chan MainChainNewRoundChan
	// channel to let nodes that the next round's leader has been specified
	MainChainNewLeaderChan chan MainChainNewLeaderChan
	NumMCLeader            atomic.Int64
	// the suite we use
	// suite network.Suite
	// to match the suit in blscosi
	Suite *pairing.SuiteBn256
	// onDoneCallback is the callback that will be called at the end of the protocol
	// onDoneCallback func() //ToDoCoder: define this function and call it when you want to finish the protocol + check when should it be called
	// channel to notify when the root node is done and that the number of rounds completed == SimulationRounds
	// when a message is sent through this channel the runsimul.go file will catch it and finish the protocol.
	DoneRootNode chan bool
	// ---------------------------------

	// to avoid conflict while modifying bc files
	BCLock sync.Mutex
	// to let the  first leader run main chain protocol and ignore the rest
	MCLeader *MCLeader
	// for root node to wait for a specific number of side chain rounds before proceeding to next main chain's round
	//wgSCRound sync.WaitGroup
	// for root node to wait for a specific number of main chain rounds before proceeding to next side chain's round
	//wgMCRound sync.WaitGroup

	wgSyncMcRound WaitGroupCount
	// for root node to wait for all nodes join the simulation before starting the ptotocols
	CalledWG sync.WaitGroup
	JoinedWG sync.WaitGroup

	MCRoundSyncWg sync.WaitGroup
	// channel that all nodes can use to announce root node they have joined the simulation
	JoinedWGChan chan JoinedWGChan
	// channel to notify leader elected
	LeaderProposeChan chan bool
	MCRoundNumber     atomic.Int64

	// --- just root node use these - these are used for delay evaluation
	FirstQueueWait  int
	SecondQueueWait int
	// sc
	SideChainQueueWait int
	// side chain queue wait
	FirstSCQueueWait int
	/* ------------------------------------------------------------------
	     -----  system-wide configurations params from the config file
	   ------------------------------------------------------------------
		these  params get initialized
		for the root node: "after" NewMainAndSideChain call (in func: Simulate in file: runsimul.go)
		for the rest of nodes node: while joining protocol by the HelloChainBoost message
	--------------------------------------------------------------------- */
	PercentageTxPay                  int
	MCRoundDuration                  int
	MainChainBlockSize               int
	SideChainBlockSize               int
	SectorNumber                     int
	NumberOfPayTXsUpperBound         int
	NumberOfActiveContractsPerServer int
	// ---
	SimulationRounds    int
	SimulationSeed      int
	SimState            int
	StoragePaymentEpoch int
	// -- blscosi related config params
	NbrSubTrees     int
	Threshold       int
	SCRoundDuration int
	CommitteeWindow int //ToDoCoder: go down
	// ---
	maxFileSize int
	/* ------------------------------------------------------------------
	 ---------------------------  bls cosi protocol  ---------------
	--------------------------------------------------------------------- */
	BlsCosi         atomic.Pointer[BLSCoSi.BlsCosi]
	MCRoundPerEpoch int
	SCRoundPerEpoch int

	// channel used by root node to trigger side chain's leader to run a new round of blscosi for side chain
	RtLSideChainNewRoundChan chan RtLSideChainNewRoundChan
	LtRSideChainNewRoundChan chan LtRSideChainNewRoundChan
	// it is initiated in the start function by root node
	BlsCosiStarted bool
	// -- meta block temp summary.
	// server agreement ID --> number of not summerized submitted PoRs in the meta blocks for this agreement

	SCSig map[string]BLSCoSi.BlsSignature

	simulationDone bool

	consensusTimeStart       time.Time
	PayPercentOfTransactions float64
	Sidechains               []string
	cosiQueue                CoSiQueue
	SCRoundNumber            map[string]*atomic.Int64

	SCTools                   map[string]SideChain
	CommitteeNodesTreeNodeID1 []onet.TreeNodeID
	NextSideChainLeader1      onet.TreeNodeID
	CountNodes                int
	FaultyContractsRate       float64
}

type GlobalTracker struct {
	MMSCAbsoluteRoundNumber atomic.Int64
	MMActivationTimes       []int
	MMContractsToActivate   []int
}

type SideChain struct {
	SummPoRTxs    map[int]int
	SummDisputes  map[int]int
	WgSyncScRound *WaitGroupCount
}

var gt GlobalTracker

/* ----------------------------------- FUNCTIONS -------------------------------------------------
------------------------------------------------------------------------------------------------  */

/*
	 ----------------------------------------------------------------------
		//Start: starts the protocol by sending hello msg to all nodes

------------------------------------------------------------------------
*/
func (bz *ChainBoost) Start() error {

	gt.MMActivationTimes = make([]int, bz.NumberOfActiveContractsPerServer*bz.CountNodes)
	gt.MMContractsToActivate = make([]int, 0)

	bz.BCLock.Lock()
	log.Lvl1("Updating the mainchainbc file with created nodes' information")
	bz.finalMainChainBCInitialization()
	bz.BCLock.Unlock()
	//---
	bz.MCLeader = &MCLeader{}
	//---
	bz.BlsCosiStarted = true
	// ------------------------------------------------------------------------------
	// config params are sent from the leader to the other nodes after
	// creating the protocols and the tree out of all nodes in simulate function in runsimul.go
	bz.helloChainBoost()

	//------- testing message sending ------
	// err := bz.SendTo(bz.Root(), &HelloChainBoost{MCRoundPerEpoch: 0})
	// if err != nil {
	// 	return err
	// }
	return nil
}

/*
	 ----------------------------------------------------------------------
				 Dispatch listen on the different channels in main chain protocol

------------------------------------------------------------------------
*/
func (bz *ChainBoost) Dispatch() error {

	// todo: some where in byzcoin the running variable was set to false. look for it later!
	running := true
	var err error
	numberOfJoinedNodes := len(bz.Tree().List())

	log.Lvlf5("starting Dispatch in chainboost simulation on:", bz.TreeNode().Name())
	for running {
		select {

		case msg := <-bz.ChainBoostDone:
			bz.simulationDone = msg.IsSimulationDone
			os.Exit(0)
			return nil

		// -----------------------------------------------------------------------------
		// ******* ALL nodes recieve this message to join the protocol and get the config values set
		// -----------------------------------------------------------------------------
		case msg := <-bz.HelloChan:
			bz.PercentageTxPay = msg.PercentageTxPay
			bz.MCRoundDuration = msg.MCRoundDuration
			bz.MainChainBlockSize = msg.MainChainBlockSize
			bz.SideChainBlockSize = msg.SideChainBlockSize
			bz.SectorNumber = msg.SectorNumber
			bz.NumberOfPayTXsUpperBound = msg.NumberOfPayTXsUpperBound
			bz.NumberOfActiveContractsPerServer = msg.NumberOfActiveContractsPerServer
			bz.SimulationSeed = msg.SimulationSeed
			bz.SCRoundDuration = msg.SCRoundDuration
			bz.CommitteeWindow = msg.CommitteeWindow
			bz.MCRoundPerEpoch = msg.MCRoundPerEpoch
			bz.SCRoundPerEpoch = msg.SCRoundPerEpoch
			// bls cosi config
			bz.NbrSubTrees = msg.NbrSubTrees
			blsCosi := bz.BlsCosi.Load()
			blsCosi.Threshold = msg.Threshold
			bz.BlsCosi.Store(blsCosi)
			bz.SimState = msg.SimState
			bz.StoragePaymentEpoch = msg.StoragePaymentEpoch
			bz.Sidechains = msg.Sidechains
			bz.CountNodes = msg.CountNodes
			bz.FaultyContractsRate = msg.FaultyContractsRate
			if msg.NbrSubTrees > 0 {
				blsCosi := bz.BlsCosi.Load()
				err := blsCosi.SetNbrSubTree(msg.NbrSubTrees)
				if err != nil {
					return err
				}
				bz.BlsCosi.Store(blsCosi)
			}
			qq := make([]string, 0)
			bz.cosiQueue.Store(&qq)
			go bz.helloChainBoost()
		// -----------------------------------------------------------------------------
		// ******* ALL nodes that recieve helloChainBoost send this message to the ROOT node to let it know they have joined the protocol
		// -----------------------------------------------------------------------------
		case msg := <-bz.JoinedWGChan:
			if bz.IsRoot() && msg.IsJoined {
				numberOfJoinedNodes--
				log.Lvl1(msg.TreeNode.Name(), "has joined the simulation:", numberOfJoinedNodes, "is remained.")
				bz.JoinedWG.Done()
			}
		// -----------------------------------------------------------------------------
		// *** MC *** ALL nodes recieve this message to sync rounds
		// -----------------------------------------------------------------------------
		case msg := <-bz.MainChainNewRoundChan:
			bz.MCRoundNumber.Inc()
			log.Lvl3(bz.Name(), " mc round number ", bz.MCRoundNumber, " started at ", time.Now().Format(time.RFC3339))
			go bz.MainChainCheckLeadership(msg)
		// -----------------------------------------------------------------------------
		// *** MC *** just the ROOT NODE (blockchain layer one) recieve this msg
		// -----------------------------------------------------------------------------
		case msg := <-bz.MainChainNewLeaderChan:
			bz.NumMCLeader.Inc()
			if int64(msg.MCRoundNumber) == bz.MCRoundNumber.Load() && (!bz.MCLeader.HasLeader || msg.TreeNode == bz.TreeNode()) {
				go func() {
					bz.MCLeader.MCPLock.Lock()
					if bz.MCLeader.HasLeader && msg.TreeNode != bz.TreeNode() {
						bz.MCLeader.MCPLock.Unlock()
						return
					}
					log.Lvl1("the first leader for mc round number", bz.MCRoundNumber, " is:", msg.TreeNode.Name())
					bz.MCLeader.HasLeader = true
					log.Lvl1("MC round number:", bz.MCRoundNumber.Load()-1, "had ", bz.NumMCLeader, "proposed leaderrs")
					bz.NumMCLeader.Store(1)
					bz.MCLeader.MCPLock.Unlock()
					// ---
					//time.Sleep(time.Duration(bz.MCRoundDuration) * time.Second)
					log.LLvlf5("DEBUG: the msg is: %v, the channel has %d messages left", msg, len(bz.MainChainNewLeaderChan))
					bz.RootPreNewRound(msg)

					//
					// log.Lvl1(len(bz.MainChainNewLeaderChan), "leaders were selected for round number", bz.MCRoundNumber, "too late!")
					// for len(bz.MainChainNewLeaderChan) > 0 {
					// 	a := <-bz.MainChainNewLeaderChan
					// 	log.Lvl5(a.LeaderTreeNodeID, " for round", a.MCRoundNumber, "popped out")
					// }
				}()
			}
		// -----------------------------------------------------------------------------
		// *** SC *** next side chain's leader recieves this message
		// -----------------------------------------------------------------------------
		case msg := <-bz.RtLSideChainNewRoundChan:
			go bz.SideChainLeaderPreNewRound("por", msg)

		// -----------------------------------------------------------------------------
		// *** SC *** just the ROOT NODE (blockchain layer one) recieve this msg
		// -----------------------------------------------------------------------------
		case msg := <-bz.LtRSideChainNewRoundChan:
			go func() {
				//time.Sleep(time.Duration(bz.SCRoundDuration) * time.Second)
				bz.SideChainRootPostNewRound("por", msg)
			}()

		case sig := <-bz.BlsCosi.Load().FinalSignature:
			log.Lvl1("Time Taken for Consensus:", time.Since(bz.consensusTimeStart).String())

			if bz.simulationDone {
				return nil
			}
			//			if err := BLSCoSi.BdnSignature(sig).Verify(bz.BlsCosi.Load().Suite, bz.BlsCosi.Load().Msg, bz.BlsCosi.Load().SubTrees[0].Roster.Publics()); err == nil {
			str := bz.cosiQueue.dequeue()
			log.Lvl1("Chain:", str, "final result SC:", bz.Name(), " : ", bz.BlsCosi.Load().BlockType, "with side chain's round number", bz.SCRoundNumber[str], "Confirmed in Side Chain")

			err := bz.SendTo(bz.Root(), &LtRSideChainNewRound{
				NewRound:      true,
				SCRoundNumber: int(bz.SCRoundNumber["por"].Load()),
				SCSig:         sig,
			})
			if err != nil {
				return xerrors.New("can't send new round msg to root" + err.Error())
			}

			/*} else {
				return xerrors.New("error in running this round of blscosi:  " + err.Error())
			}*/ //XXX: Disabled Until Sidechain Committee Selection is implemented.
		}
	}
	return err
}

/* ----------------------------------------------------------------------
 helloChainBoost
------------------------------------------------------------------------ */

func (bz *ChainBoost) helloChainBoost() {
	if !bz.IsRoot() {
		log.Lvl5(bz.TreeNode().Name(), " joined to the protocol")
		// all nodes get here and start to listen for blscosi protocol messages
		// go func() {
		// 	err := bz.DispatchProtocol()
		// 	if err != nil {
		// 		log.LLvl1("protocol dispatch calling error: " + err.Error())
		// 		//panic("protocol dispatch calling error")
		// 	}
		// }()
		err := bz.SendTo(bz.Root(), &Joined{IsJoined: true})
		if err != nil {
			log.Lvl1(bz.Name(), "can't announce joining to root node")
		}
	}

	if bz.IsRoot() && bz.BlsCosiStarted {
		bz.JoinedWG.Wait()
		log.Lvl1("All nodes joined  the simulation, Done waiting, The Root node will start the protocol(s)")
		if bz.SimState == 2 {
			//--------------------------------------------------
			// this equation result has to be int!
			if int(bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration)) != bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration) {
				log.LLvl1("Panic Raised:\n\n")
				panic("err in setting config params: {bz.MCRoundPerEpoch*(bz.MCRoundDuration / bz.SCRoundDuration)} has to be int")
			}
			//--------------------------------------------------
			// each epoch this number of sc rounds should be passed
			log.Lvl1("Coder Debug: wgSCRound.Add(", bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration), ")")
			//bz.wgSCRound.Add(bz.MCRoundPerEpoch * (bz.MCRoundDuration / bz.SCRoundDuration))
			//--------------------------------------------------
			// each epoch this number of sc rounds should be passed
			log.Lvl1("Coder Debug: wgMCRound.Add(", bz.MCRoundPerEpoch, ")")
			//bz.wgMCRound.Add(bz.MCRoundPerEpoch + 1)
			bz.wgSyncMcRound.Add("bz.wgSyncMcRound", len(bz.Sidechains))
			//--------------------------------------------------
			go bz.StartMainChainProtocol()
			//go bz.StartSideChainProtocol()
		} else if bz.SimState == 1 {
			bz.StartMainChainProtocol()
		} else {
			panic("sim state config param is not set correctly")
		}
	}
}

// -------------------------------------- timeouts -------------------------------------- //

/*
	------------------------------------------------------------------------

this function will be called just by ROOT NODE when:
- at the end of a round duration (when no "I am a leader message is recieved): startTimer starts the timer to detect the rounds that dont have any leader elected (if any)
and publish an empty block in those rounds
------------------------------------------------------------------------
*/
func (bz *ChainBoost) startTimer(MCRoundNumber int) {
	select {
	case <-time.After(time.Duration(bz.MCRoundDuration) * time.Second):
		// bz.IsRoot() is here just to make sure!,
		// bz.MCRoundNumber == MCRoundNumber is for when the round number has changed!,
		// bz.hasLeader is for when the round number has'nt changed but the leader has been announced
		bz.MCLeader.MCPLock.Lock()
		if bz.IsRoot() && bz.MCRoundNumber.Load() == int64(MCRoundNumber) && !bz.MCLeader.HasLeader {
			log.Lvl1("No leader for mc round number ", bz.MCRoundNumber, "an empty block is added")
			bz.MCLeader.HasLeader = true
			log.Lvl5("DEBUG: StarTimer: Bz.TreeNodeId()", bz.TreeNode().ID, "bz.MCRoundNumber: ", bz.MCRoundNumber, "MCRoundNumber: ", MCRoundNumber)
			bz.MainChainNewLeaderChan <- MainChainNewLeaderChan{bz.TreeNode(), NewLeader{LeaderTreeNodeID: bz.TreeNode().ID, MCRoundNumber: int(bz.MCRoundNumber.Load())}}
		}
		bz.MCLeader.MCPLock.Unlock()
	}
}

// ------------   Sortition Algorithm from ALgorand: ---------------------
// ⟨hash,π⟩←VRFsk(seed||role)
// p←τ/W
// j←0
// while hash/ 2^hashleng </ [ sigma(k=0,j) (B(k,w,p),  sigma(k=0,j+1) (B(k,w,p)] do
//	----	 j++
// return <hash,π, j>
// ----------------------------------------------------------------------

/* ----------------------------------------------------------------------
//checkLeadership
------------------------------------------------------------------------ */

// func (bz *ChainBoost) checkLeadership() {
// 	if bz.leaders[int(math.Mod(float64(bz.MCRoundNumber), float64(len(bz.Roster().List))))] == bz.ServerIdentity().String() {
// 		bz.LeaderPropose <- true
// 	}
// }
/* func (bz *ChainBoost) checkLeadership(power uint64, seed string) {
	for {
		select {
		case <-bz.MainChainNewLeaderChan:
			return
		default:

			//power := 1
			//var seed []byte
			var vrfOutput [64]byte

			toBeHashed := []byte(seed)
			proof, ok := bz.ECPrivateKey.ProveBytes(toBeHashed[:])
			if !ok {
				log.LLvl1("error while generating proof")
			}
			_, vrfOutput = bz.ECPrivateKey.Pubkey().VerifyBytes(proof, toBeHashed[:])

			//------------------------- working with big int ----------------------
			//"math/big" imported
			//func generateRandomValuesBigInt(nodes int) [] *big.Int {
			//	var bigIntlist [] *big.Int
			//	for i := 1; i<= nodes;i++{
			//		bigIntlist = append(bigIntlist, new(big.Int).Rand(rand.New(rand.NewSource(time.Now().UnixNano())),<some big int value> ))
			//	}
			//	return bigIntlist
			//}
			// --------------------------------------------------------------------
			//For future refrence: the next commented line is a wrong way to convert a big number to int - it wont raise overflow but the result is incorrect
			//t := binary.BigEndian.Uint64(vrfOutput[:])
			//--------------------------------------------------
			//For future refrence: the next commented line is converting a big number to big int (built in type in go)
			//var bi *big.Int
			//bi = new(big.Int).SetBytes(vrfOutput[:])
			//--------------------------------------------------

			var vrfoutputInt64 uint64
// 			buf := bytes.NewReader(vrfOutput[:])
// 			err := binary.Read(buf, binary.LittleEndian, &vrfoutputInt64)
// 			if err != nil {
// 				log.LLvl1("Panic Raised:\n\n")
// 				panic(err)
// 			}
// 			// I want to ask the previous leader to find next leader based on its output and announce it to her via next round msg
// 			if vrfoutputInt64 < power {
// 				// let other know i am the leader
// 				for _, b := range bz.Tree().List() {
// 					err := bz.SendTo(b, &NewLeader{})
// 					if err != nil {
// 						log.LLvl1(bz.Info(), "can't send new round msg to", b.Name())
// 					}
// 				}
// 				bz.LeaderProposeChan <- true
// 			}
// 			//else {
// 			// 	log.LLvl1("my power:", power, "is", vrfoutputInt64-power, "less than my vrf output :| ")
// 			// }
// 		}
// 	}
// }
*/

// Testpor
func (bz *ChainBoost) Testpor() {

	sk, pk := por.RandomizedKeyGeneration()
	Tau, pf := por.RandomizedFileStoring(sk, por.GenerateFile(bz.SectorNumber), bz.SectorNumber)
	p := por.CreatePoR(pf, bz.SectorNumber, bz.SimulationSeed)
	d, _ := por.VerifyPoR(pk, Tau, p, bz.SectorNumber, bz.SimulationSeed)
	if !d {
		log.LLvl1(d)
	}
}
