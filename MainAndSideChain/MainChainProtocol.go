package MainAndSideChain

// ----------------------------------------------------------------------------------------------
// -------------------------- main chain's protocol ---------------------------------------
// ----------------------------------------------------------------------------------------------

import (
	"bytes"
	"encoding/binary"
	"math"
	"math/rand"
	"time"

	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"github.com/chainBoostScale/ChainBoost/vrf"
	"golang.org/x/xerrors"
)

/* ----------------------------------------- TYPES -------------------------------------------------
------------------------------------------------------------------------------------------------  */

type NewLeader struct {
	LeaderTreeNodeID onet.TreeNodeID
	MCRoundNumber    int
}
type MainChainNewLeaderChan struct {
	*onet.TreeNode
	NewLeader
}

type NewRound struct {
	Seed        string
	Power       int
	MaxFileSize int
}
type MainChainNewRoundChan struct {
	*onet.TreeNode
	NewRound
}

/* ----------------------------------- FUNCTIONS -------------------------------------------------
------------------------------------------------------------------------------------------------  */

func (bz *ChainBoost) StartMainChainProtocol() {
	// the root node is filling the first block in first round
	log.LLvl1(bz.Name(), " :the root node is filling the first block in mc round number: ", bz.MCRoundNumber)
	// for the first round we have the root node set as a round leader, so  it is true! and he takes txs from the queue
	//----
	if bz.SimState == 2 {
		log.Lvl1("Coder Debug: wgMCRound.Done")
		//bz.wgMCRound.Done()
	}
	if bz.SimState == 2 && bz.MCRoundNumber.Load() != 1 && bz.MCRoundNumber.Load()-1%int64(bz.MCRoundPerEpoch) == 0 {
		eachFunctionTakenTime := time.Now()
		log.Lvl1("Coder Debug: wgSCRound.Wait")
		//bz.wgSCRound.Wait()
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: bz.wgSCRound.Wait() took:", time.Since(eachFunctionTakenTime).String())
		log.Lvl1("Coder Debug: wgSCRound.Wait: PASSED")
		// this equation result has to be int!
		if int(bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration)) != bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration) {
			log.LLvl1("Panic Raised:\n\n")
			panic("err in setting config params: {bz.MCRoundPerEpoch*(bz.MCRoundDuration / bz.SCRoundDuration)} has to be int")
		}
		// each epoch this number of sc rounds should be passed
		log.Lvl1("Coder Debug: wgSCRound.Add(", bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration), ")")
		//bz.wgSCRound.Add(bz.MCRoundPerEpoch * (bz.MCRoundDuration / bz.SCRoundDuration))
	}
	if bz.SimState == 2 {
		for i, _ := range bz.SCTools {
			bz.SCTools[i].WgSyncScRound.Add(i+"bz.wgSyncScRound", bz.MCRoundDuration/bz.SCRoundDuration)
		}
	}
	bz.BCLock.Lock()
	// ---
	bz.updateBCPowerRound(bz.TreeNode().Name(), true)
	bz.updateMainChainBCTransactionQueueCollect()
	bz.updateMainChainBCTransactionQueueTake()
	if bz.SimState == 2 {
		go bz.StartSideChainProtocol()
	}
	//time.Sleep(time.Duration(bz.MCRoundDuration) * time.Second)
	bz.readBCAndSendtoOthers()
	bz.BCLock.Unlock()
	log.Lvl1("new round is announced")
}
func (bz *ChainBoost) RootPreNewRound(msg MainChainNewLeaderChan) {
	RootPreNewRoundTakenTime := time.Now()
	var eachFunctionTakenTime time.Time

	// -----------------------------------------------------
	// rounds without a leader =>
	// in this case the leader info is filled with root node's info, transactions are going to be collected normally but
	// since the block is empty, no transaction is going to be taken from queues => leader = false
	// -----------------------------------------------------
	log.Lvl5("DEBUG: RootPreNewRound(Entry):", msg.Name(), " msg.LeaderTreeNodeID: ", msg.LeaderTreeNodeID, "bz.TreeNode().Id: ", bz.TreeNode().ID, "bz.MCRoundNumber: ", bz.MCRoundNumber, "bz.MCLeader.HasLeader: ", bz.MCLeader.HasLeader, "msg.MCRoundNumber: ", msg.MCRoundNumber)
	if msg.LeaderTreeNodeID == bz.TreeNode().ID && bz.MCRoundNumber.Load() != 1 && bz.MCLeader.HasLeader && int64(msg.MCRoundNumber) == bz.MCRoundNumber.Load() {
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: start of a round without a leader")
		if bz.SimState == 2 {
			log.Lvl1("Coder Debug: wgMCRound.Done")
			//bz.wgMCRound.Done()
		}
		// -----------------------------------------------------
		// wait for MCDuration/SCduration number of go routines
		// to be Done (Done is triggered after finishing SideChainRootPostNewRound)
		if bz.SimState == 2 && bz.MCRoundNumber.Load() != 1 && bz.MCRoundNumber.Load()-1%int64(bz.MCRoundPerEpoch) == 0 {
			eachFunctionTakenTime = time.Now()
			log.Lvl1("Coder Debug: wgSCRound.Wait")
			//bz.wgSCRound.Wait()
			log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: bz.wgSCRound.Wait() took:", time.Since(eachFunctionTakenTime).String())
			log.Lvl1("Coder Debug: wgSCRound.Wait: PASSED")
			// this equation result has to be int!
			if int(bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration)) != bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration) {
				log.LLvl1("Panic Raised:\n\n")
				panic("err in setting config params: {bz.MCRoundPerEpoch*(bz.MCRoundDuration / bz.SCRoundDuration)} has to be int")
			}
			// each epoch this number of sc rounds should be passed
			log.Lvl1("Coder Debug: wgSCRound.Add(", bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration), ")")
			//bz.wgSCRound.Add(bz.MCRoundPerEpoch * (bz.MCRoundDuration / bz.SCRoundDuration))
		}
		if bz.SimState == 2 {
			for i, _ := range bz.SCTools {
				bz.SCTools[i].WgSyncScRound.Wait(i + "wgSyncScRound")
				bz.SCTools[i].WgSyncScRound.Add(i+"bz.wgSyncScRound", bz.MCRoundDuration/bz.SCRoundDuration)
			}
		}
		// -----------------------------------------------------
		bz.BCLock.Lock()
		//----
		eachFunctionTakenTime = time.Now()
		bz.updateBCPowerRound(bz.Tree().Search(msg.LeaderTreeNodeID).Name(), false)
		bz.UpdateFaultyContracts()
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: updateBCPowerRound took:", time.Since(eachFunctionTakenTime).String())
		// in the case of a leader-less round
		log.Lvl1("Final result MC: leader TreeNodeID: ROOT NODE filled mc round number", bz.MCRoundNumber, "with empty block")
		// ---
		eachFunctionTakenTime = time.Now()
		bz.updateMainChainBCTransactionQueueCollect()
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: updateMainChainBCTransactionQueueCollect took:", time.Since(eachFunctionTakenTime).String())
		// ---
		//bz.updateSideChainBCTransactionQueueCollect()
		// ---
		eachFunctionTakenTime = time.Now()
		bz.readBCAndSendtoOthers()
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: readBCAndSendtoOthers took:", time.Since(eachFunctionTakenTime).String())
		log.Lvl1("new round is announced")
		//----
		bz.BCLock.Unlock()
		// -----------------------------------------------------
		// normal rounds with a leader => leader = true
		// -----------------------------------------------------
	} else if int64(msg.MCRoundNumber) == bz.MCRoundNumber.Load() && bz.MCRoundNumber.Load() != 1 && bz.MCLeader.HasLeader {
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: start of a round with a leader")
		// dynamically change the side chain's committee with last main chain's leader
		if bz.SimState == 2 { // i.e. if side chain running is set in simulation
			eachFunctionTakenTime = time.Now()
			bz.UpdateSideChainCommittee(msg)
			log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: UpdateSideChainCommittee took:", time.Since(eachFunctionTakenTime).String())
		}

		// -----------------------------------------------
		// -----------------------------------------------
		if bz.SimState == 2 {
			log.Lvl1("Coder Debug: wgMCRound.Done")
			//bz.wgMCRound.Done()
		}
		// -----------------------------------------------------
		// wait for MCDuration/SCduration number of go routines
		// to be Done (Done is triggered after finishing SideChainRootPostNewRound)
		if bz.SimState == 2 && bz.MCRoundNumber.Load() != 1 && bz.MCRoundNumber.Load()-1%int64(bz.MCRoundPerEpoch) == 0 {
			eachFunctionTakenTime = time.Now()
			log.Lvl1("Coder Debug: wgSCRound.Wait")
			//bz.wgSCRound.Wait()
			log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: bz.wgSCRound.Wait() took:", time.Since(eachFunctionTakenTime).String())
			log.Lvl1("Coder Debug: wgSCRound.Wait: PASSED")
			// this equation result has to be int!
			if int(bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration)) != bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration) {
				log.LLvl1("Panic Raised:\n\n")
				panic("err in setting config params: {bz.MCRoundPerEpoch*(bz.MCRoundDuration / bz.SCRoundDuration)} has to be int")
			}
			// each epoch this number of sc rounds should be passed
			log.Lvl1("Coder Debug: wgSCRound.Add(", bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration), ")")
			//bz.wgSCRound.Add(bz.MCRoundPerEpoch * (bz.MCRoundDuration / bz.SCRoundDuration))
		}
		if bz.SimState == 2 {
			for i, _ := range bz.SCTools {
				bz.SCTools[i].WgSyncScRound.Wait(i + "wgSyncScRound")
				bz.SCTools[i].WgSyncScRound.Add(i+"bz.wgSyncScRound", bz.MCRoundDuration/bz.SCRoundDuration)
			}
		}
		// -----------------------------------------------
		// -----------------------------------------------

		// -----------------------------------------------------

		// ToDoCoder: later validate the leadership proof
		eachFunctionTakenTime = time.Now()
		log.LLvl1("Final result MC: leader: ", bz.Tree().Search(msg.LeaderTreeNodeID).Name(), " is the round leader for mc round number ", bz.MCRoundNumber)
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: Search took:", time.Since(eachFunctionTakenTime).String())
		// -----------------------------------------------
		bz.BCLock.Lock()
		// ---
		eachFunctionTakenTime = time.Now()
		bz.updateBCPowerRound(bz.Tree().Search(msg.LeaderTreeNodeID).Name(), true)
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: updateBCPowerRound took:", time.Since(eachFunctionTakenTime).String())
		// ---
		eachFunctionTakenTime = time.Now()
		bz.updateMainChainBCTransactionQueueCollect()
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: updateMainChainBCTransactionQueueCollect took:", time.Since(eachFunctionTakenTime).String())
		// ---
		//bz.updateSideChainBCTransactionQueueCollect()
		// ---
		eachFunctionTakenTime = time.Now()
		bz.updateMainChainBCTransactionQueueTake()
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: updateMainChainBCTransactionQueueTake took:", time.Since(eachFunctionTakenTime).String())
		// ---
		// announce new round and give away required checkleadership info to nodes
		eachFunctionTakenTime = time.Now()
		bz.readBCAndSendtoOthers()
		log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: readBCAndSendtoOthers took:", time.Since(eachFunctionTakenTime).String())
		log.Lvl1("new round is announced")
		//----
		bz.BCLock.Unlock()
	} else if int64(msg.MCRoundNumber) == bz.MCRoundNumber.Load() {
		log.Lvl1("this round already has a leader!")
	} else {
		log.Lvl5("DEBUG: None of the Conditions is true:", msg.Name(), " msg.LeaderTreeNodeID: ", msg.LeaderTreeNodeID, "bz.TreeNode().Id: ", bz.TreeNode().ID, "bz.MCRoundNumber: ", bz.MCRoundNumber, "bz.MCLeader.HasLeader: ", bz.MCLeader.HasLeader, "msg.MCRoundNumber: ", msg.MCRoundNumber)
	}
	if bz.SimState == 2 {
		for _, _ = range bz.Sidechains {
			bz.wgSyncMcRound.Done("bz.wgSyncMcRound")
		}
	}
	log.Lvl1("RootPreNewRound in mcroundnumber ", bz.MCRoundNumber, " time report: the whole function took:", time.Since(RootPreNewRoundTakenTime).String())
}
func (bz *ChainBoost) MainChainCheckLeadership(msg MainChainNewRoundChan) error {
	var vrfOutput [64]byte
	toBeHashed := []byte(msg.Seed)

	proof, ok := bz.ECPrivateKey.ProveBytesGo(toBeHashed[:])
	if !ok {
		log.LLvl1("error while generating proof")
	}
	//---
	rand.Seed(int64(bz.TreeNodeInstance.Index()))
	seed := make([]byte, 32)
	rand.Read(seed)
	tempSeed := (*[32]byte)(seed[:32])
	//log.LLvl1(":debug:seed for the VRF is:", seed, "the tempSeed value is:", tempSeed)
	Pubkey, _ := vrf.VrfKeygenFromSeedGo(*tempSeed)
	//---
	_, vrfOutput = Pubkey.VerifyBytesGo(proof, toBeHashed[:])

	//ToDoCoder: a random 64 byte instead of vrf output
	//vrfOutput := make([]byte, 64)
	//rand.Read(vrfOutput)
	//log.LLvl1("Coder: the random gerenrated number is:", vrfOutput)

	var vrfoutputInt64 uint64
	buf := bytes.NewReader(vrfOutput[:])
	err := binary.Read(buf, binary.LittleEndian, &vrfoutputInt64)
	if err != nil {
		log.LLvl1("Panic Raised:\n\n")
		// panic(err)
		return xerrors.New("problem created after recieving msg from MainChainNewRoundChan:   " + err.Error())
	}
	log.Lvl4(bz.TreeNode().Name(), "'s VRF output:", vrfoutputInt64)
	//-----------
	vrfoutputInt := int(vrfoutputInt64) % msg.MaxFileSize
	vrfoutputInt = msg.MaxFileSize - int(math.Abs(float64(vrfoutputInt)))
	//the criteria for selecting potential leaders
	if vrfoutputInt < msg.Power {
		// -----------
		log.Lvl2(bz.Name(), "I may be elected for mc round number ", bz.MCRoundNumber, "with power: ", msg.Power, "and vrf output of:", vrfoutputInt)
		log.Lvl5("DEBUG: [Election]:", bz.Name(), "bz.TreeNode().Id: ", bz.TreeNode().ID, "bz.MCRoundNumber: ", bz.MCRoundNumber)
		bz.SendTo(bz.Root(), &NewLeader{LeaderTreeNodeID: bz.TreeNode().ID, MCRoundNumber: int(bz.MCRoundNumber.Load())})
	}
	return nil
}
