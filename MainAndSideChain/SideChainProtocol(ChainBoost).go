package MainAndSideChain

import (
	"math"
	"math/rand"
	"os"
	"time"

	"github.com/chainBoostScale/ChainBoost/MainAndSideChain/BLSCoSi"
	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"github.com/chainBoostScale/ChainBoost/onet/network"
	"golang.org/x/xerrors"
)

/* ----------------------------------------- TYPES -------------------------------------------------
------------------------------------------------------------------------------------------------  */

// channel used by side chain's leader in each side chain's round
type RtLSideChainNewRound struct {
	SCRoundNumber            int
	CommitteeNodesTreeNodeID []onet.TreeNodeID
	blocksize                int
}
type RtLSideChainNewRoundChan struct {
	*onet.TreeNode
	RtLSideChainNewRound
}
type LtRSideChainNewRound struct { //fix this
	NewRound      bool
	SCRoundNumber int
	SCSig         BLSCoSi.BlsSignature
}
type LtRSideChainNewRoundChan struct {
	*onet.TreeNode
	LtRSideChainNewRound
}

type RtLSideChainNewRoundChain2 struct {
	SCRoundNumber            int
	CommitteeNodesTreeNodeID []onet.TreeNodeID
	blocksize                int
}
type RtLSideChainNewRoundChanChain2 struct {
	*onet.TreeNode
	RtLSideChainNewRoundChain2
}
type LtRSideChainNewRoundChain2 struct { //fix this
	NewRound      bool
	SCRoundNumber int
	SCSig         BLSCoSi.BlsSignature
}
type LtRSideChainNewRoundChanChain2 struct {
	*onet.TreeNode
	LtRSideChainNewRoundChain2
}

type RtLSideChainNewRoundChain3 struct {
	SCRoundNumber            int
	CommitteeNodesTreeNodeID []onet.TreeNodeID
	blocksize                int
}
type RtLSideChainNewRoundChanChain3 struct {
	*onet.TreeNode
	RtLSideChainNewRoundChain3
}
type LtRSideChainNewRoundChain3 struct { //fix this
	NewRound      bool
	SCRoundNumber int
	SCSig         BLSCoSi.BlsSignature
}
type LtRSideChainNewRoundChanChain3 struct {
	*onet.TreeNode
	LtRSideChainNewRoundChain3
}

type RtLSideChainNewRoundChain4 struct {
	SCRoundNumber            int
	CommitteeNodesTreeNodeID []onet.TreeNodeID
	blocksize                int
}
type RtLSideChainNewRoundChanChain4 struct {
	*onet.TreeNode
	RtLSideChainNewRoundChain4
}
type LtRSideChainNewRoundChain4 struct { //fix this
	NewRound      bool
	SCRoundNumber int
	SCSig         BLSCoSi.BlsSignature
}
type LtRSideChainNewRoundChanChain4 struct {
	*onet.TreeNode
	LtRSideChainNewRoundChain4
}

/* ----------------------------------- FUNCTIONS -------------------------------------------------
------------------------------------------------------------------------------------------------  */

/*
	 ----------------------------------------------------------------------
		DispatchProtocol listen on the different channels in side chain protocol

------------------------------------------------------------------------
*/
func (bz *ChainBoost) DispatchProtocol() error {

	running := true
	var err error

	for running {
		select {

		case msg := <-bz.ChainBoostDone:
			bz.simulationDone = msg.IsSimulationDone
			os.Exit(0)
			return nil

		// --------------------------------------------------------
		// message recieved from BLSCoSi (SideChain):
		// ******* just the current side chain's "LEADER" recieves this msg
		// note that other messages communicated in BlsCosi protocol are handled by
		// func (p *SubBlsCosi) Dispatch() which is called when the startSubProtocol in
		// Blscosi.go, create subprotocols => hence calls func (p *SubBlsCosi) Dispatch()
		// --------------------------------------------------------
		case sig := <-bz.BlsCosi.Load().FinalSignature:
			if bz.simulationDone {
				return nil
			}

			if err := BLSCoSi.BdnSignature(sig).Verify(bz.BlsCosi.Load().Suite, bz.BlsCosi.Load().Msg, bz.BlsCosi.Load().SubTrees[0].Roster.Publics()); err == nil {
				log.Lvl1("final result SC:", bz.Name(), " : ", bz.BlsCosi.Load().BlockType, "with side chain's round number", bz.SCRoundNumber, "Confirmed in Side Chain")
				err := bz.SendTo(bz.Root(), &LtRSideChainNewRound{
					NewRound:      true,
					SCRoundNumber: int(bz.SCRoundNumber["por"].Load()),
					SCSig:         sig,
				})
				if err != nil {
					return xerrors.New("can't send new round msg to root" + err.Error())
				}
			} else {
				return xerrors.New("error in running this round of blscosi:  " + err.Error())
			}
		}
	}
	return err
}

// SideChainLeaderPreNewRound is run by the side chain's leader
func (bz *ChainBoost) SideChainLeaderPreNewRound(name string, iMsg interface{}) error {
	var roundNbr int
	var CommitteeNodesTreeNodeID []onet.TreeNodeID
	var blockSize int
	if name == "por" {
		msg := iMsg.(RtLSideChainNewRoundChan)
		roundNbr = msg.SCRoundNumber
		CommitteeNodesTreeNodeID = msg.CommitteeNodesTreeNodeID
		blockSize = msg.blocksize
	} else if name == "por2" {
		msg := iMsg.(RtLSideChainNewRoundChanChain4)
		roundNbr = msg.SCRoundNumber
		CommitteeNodesTreeNodeID = msg.CommitteeNodesTreeNodeID
		blockSize = msg.blocksize
	} else if name == "dispute" {
		msg := iMsg.(RtLSideChainNewRoundChanChain3)
		roundNbr = msg.SCRoundNumber
		CommitteeNodesTreeNodeID = msg.CommitteeNodesTreeNodeID
		blockSize = msg.blocksize
	} else {
		msg := iMsg.(RtLSideChainNewRoundChanChain2)
		roundNbr = msg.SCRoundNumber
		CommitteeNodesTreeNodeID = msg.CommitteeNodesTreeNodeID
		blockSize = msg.blocksize
	}

	var err error
	bz.SCRoundNumber[name].Store(int64(roundNbr))
	blsCosi := bz.BlsCosi.Load()
	blsCosi.Msg = []byte{0xFF}
	bz.BlsCosi.Store(blsCosi)
	takenTime := time.Now()
	// -----------------------------------------------
	// --- updating the next side chain's leader
	// -----------------------------------------------

	var CommitteeNodesServerIdentity []*network.ServerIdentity
	if bz.SCRoundNumber[name].Load() == 1 {
		// just in the first sc round it wont be nil, else this leader has already the info
		if name == "por" {
			bz.CommitteeNodesTreeNodeID1 = CommitteeNodesTreeNodeID
			//todo: a out of range bug happens sometimes!
			//log.LLvl1(": debug:", bz.CommitteeWindow-1)
			log.Lvl2(": debug:", bz.CommitteeWindow)
			log.Lvl1("log :bz.CommitteeWindow:", bz.CommitteeWindow)
			for _, a := range bz.CommitteeNodesTreeNodeID1[0 : bz.CommitteeWindow-1] {
				//for _, a := range bz.CommitteeNodesTreeNodeID[0:bz.CommitteeWindow] {
				CommitteeNodesServerIdentity = append(CommitteeNodesServerIdentity, bz.Tree().Search(a).ServerIdentity)
			}
			log.Lvl1("final result SC: ", bz.Name(), " is running next side chain's epoch with new committee")
			for i, a := range bz.CommitteeNodesTreeNodeID1 {
				log.Lvl3("final result SC: BlsCosi: next side chain's epoch committee number ", i, ":", bz.Tree().Search(a).Name())
			}
		} else if name == "dispute" {
			bz.CommitteeNodesTreeNodeID3 = CommitteeNodesTreeNodeID
			//todo: a out of range bug happens sometimes!
			//log.LLvl1(": debug:", bz.CommitteeWindow-1)
			log.Lvl2(": debug:", bz.CommitteeWindow)
			log.Lvl1("log :bz.CommitteeWindow:", bz.CommitteeWindow)
			for _, a := range bz.CommitteeNodesTreeNodeID3[0 : bz.CommitteeWindow-1] {
				//for _, a := range bz.CommitteeNodesTreeNodeID[0:bz.CommitteeWindow] {
				CommitteeNodesServerIdentity = append(CommitteeNodesServerIdentity, bz.Tree().Search(a).ServerIdentity)
			}
			log.Lvl1("final result SC: ", bz.Name(), " is running next side chain's epoch with new committee")
			for i, a := range bz.CommitteeNodesTreeNodeID3 {
				log.Lvl3("final result SC: BlsCosi: next side chain's epoch committee number ", i, ":", bz.Tree().Search(a).Name())
			}
		} else if name == "por2" {
			bz.CommitteeNodesTreeNodeID4 = CommitteeNodesTreeNodeID
			//todo: a out of range bug happens sometimes!
			//log.LLvl1(": debug:", bz.CommitteeWindow-1)
			log.Lvl2(": debug:", bz.CommitteeWindow)
			log.Lvl1("log :bz.CommitteeWindow:", bz.CommitteeWindow)
			for _, a := range bz.CommitteeNodesTreeNodeID4[0 : bz.CommitteeWindow-1] {
				//for _, a := range bz.CommitteeNodesTreeNodeID[0:bz.CommitteeWindow] {
				CommitteeNodesServerIdentity = append(CommitteeNodesServerIdentity, bz.Tree().Search(a).ServerIdentity)
			}
			log.Lvl1("final result SC: ", bz.Name(), " is running next side chain's epoch with new committee")
			for i, a := range bz.CommitteeNodesTreeNodeID4 {
				log.Lvl3("final result SC: BlsCosi: next side chain's epoch committee number ", i, ":", bz.Tree().Search(a).Name())
			}
		} else {
			bz.CommitteeNodesTreeNodeID2 = CommitteeNodesTreeNodeID
			//todo: a out of range bug happens sometimes!
			//log.LLvl1(": debug:", bz.CommitteeWindow-1)
			log.Lvl2(": debug:", bz.CommitteeWindow)
			log.Lvl1("log :bz.CommitteeWindow:", bz.CommitteeWindow)
			for _, a := range bz.CommitteeNodesTreeNodeID2[0 : bz.CommitteeWindow-1] {
				//for _, a := range bz.CommitteeNodesTreeNodeID[0:bz.CommitteeWindow] {
				CommitteeNodesServerIdentity = append(CommitteeNodesServerIdentity, bz.Tree().Search(a).ServerIdentity)
			}
			log.Lvl1("final result SC: ", bz.Name(), " is running next side chain's epoch with new committee")
			for i, a := range bz.CommitteeNodesTreeNodeID2 {
				log.Lvl3("final result SC: BlsCosi: next side chain's epoch committee number ", i, ":", bz.Tree().Search(a).Name())
			}
		}
	} else {
		//log.LLvl1(len(bz.CommitteeNodesTreeNodeID))
		//log.LLvl1(bz.CommitteeNodesTreeNodeID[len(bz.CommitteeNodesTreeNodeID)-(bz.CommitteeWindow):])
		// todo: check that the case with changing committee after choosing next sc leader (me!) doesnt happen and if it does , it doesnt affect my committee!
		if name == "por" {
			for _, a := range bz.CommitteeNodesTreeNodeID1[0 : bz.CommitteeWindow-1] {
				CommitteeNodesServerIdentity = append(CommitteeNodesServerIdentity, bz.Tree().Search(a).ServerIdentity)
			}
		} else if name == "por2" {
			for _, a := range bz.CommitteeNodesTreeNodeID4[0 : bz.CommitteeWindow-1] {
				CommitteeNodesServerIdentity = append(CommitteeNodesServerIdentity, bz.Tree().Search(a).ServerIdentity)
			}
		} else if name == "dispute" {
			for _, a := range bz.CommitteeNodesTreeNodeID3[0 : bz.CommitteeWindow-1] {
				CommitteeNodesServerIdentity = append(CommitteeNodesServerIdentity, bz.Tree().Search(a).ServerIdentity)
			}
		} else {
			for _, a := range bz.CommitteeNodesTreeNodeID2[0 : bz.CommitteeWindow-1] {
				CommitteeNodesServerIdentity = append(CommitteeNodesServerIdentity, bz.Tree().Search(a).ServerIdentity)
			}
		}
		log.Lvl1("final result SC: ", bz.Name(), " is running next side chain's epoch with the same committee members:")
		for i, a := range CommitteeNodesServerIdentity {
			log.Lvl3("final result SC: BlsCosi: next side chain's epoch committee number ", i, ":", a.Address)
		}
	}
	if bz.SCRoundNumber[name].Load() == 0 {
		blsCosi := bz.BlsCosi.Load()
		blsCosi.BlockType = "Summary Block"
		bz.BlsCosi.Store(blsCosi)
	} else {
		blsCosi := bz.BlsCosi.Load()
		blsCosi.BlockType = "Meta Block"
		bz.BlsCosi.Store(blsCosi)
	}
	committeeRoster := onet.NewRoster(CommitteeNodesServerIdentity)
	// --- root should have root index of 0 (this is based on what happens in gen_tree.go)
	var x = *bz.TreeNode()
	x.RosterIndex = 0
	// ---
	blsCosi = bz.BlsCosi.Load()
	blsCosi.SubTrees, err = BLSCoSi.NewBlsProtocolTree(onet.NewTree(committeeRoster, &x), bz.NbrSubTrees)
	bz.BlsCosi.Store(blsCosi)
	if err == nil {
		if bz.SCRoundNumber[name].Load() == 1 {
			log.Lvl3("final result SC: Next bls cosi tree is: ", bz.BlsCosi.Load().SubTrees[0].Roster.List,
				" with ", bz.Name(), " as Root \n running BlsCosi sc round number", bz.SCRoundNumber)
		}
	} else {
		return xerrors.New("Problem in cosi protocol run:   " + err.Error())
	}
	// ---
	// from bc: update msg size with next block size on side chain
	s := make([]byte, blockSize)
	blsCosi = bz.BlsCosi.Load()
	blsCosi.Msg = append(blsCosi.Msg, s...) // Msg is the meta block
	bz.BlsCosi.Store(blsCosi)
	// ----
	//go func() error {
	bz.BlsCosi.Load().Start()
	bz.cosiQueue.enqueue(name)
	bz.consensusTimeStart = time.Now()
	log.Lvl1("Chain:", name, "SideChainLeaderPreNewRound took:", time.Since(takenTime).String(), "for sc round number", bz.SCRoundNumber)
	//	if err != nil {
	//		return xerrors.New("Problem in cosi protocol run:   " + err.Error())
	//	}
	return nil
	//}()
	//return xerrors.New("Problem in cosi protocol run: should not get here")
}

func (bz *ChainBoost) SideChainRootPostNewRound(name string, iMsg interface{}) error {
	var err error
	var leaderName string
	if name == "por" {
		msg := iMsg.(LtRSideChainNewRoundChan)
		bz.SCSig[name] = msg.SCSig
		bz.SCRoundNumber[name].Store(int64(msg.SCRoundNumber))
		leaderName = msg.Name()
	} else if name == "por2" {
		msg := iMsg.(LtRSideChainNewRoundChanChain4)
		bz.SCSig[name] = msg.SCSig
		bz.SCRoundNumber[name].Store(int64(msg.SCRoundNumber))
		leaderName = msg.Name()
	} else if name == "dispute" {
		msg := iMsg.(LtRSideChainNewRoundChanChain3)
		bz.SCSig[name] = msg.SCSig
		bz.SCRoundNumber[name].Store(int64(msg.SCRoundNumber))
		leaderName = msg.Name()
	} else {
		msg := iMsg.(LtRSideChainNewRoundChanChain2)
		bz.SCSig[name] = msg.SCSig
		bz.SCRoundNumber[name].Store(int64(msg.SCRoundNumber))
		leaderName = msg.Name()
	}
	var blocksize int
	//--- maybe locking would work, but if possible! I prefer to not lock here to avoid extra complexity
	var tempCommitteeNodesTreeNodeID []onet.TreeNodeID
	// --------------------------------------------------------------------
	if bz.MCRoundPerEpoch*(bz.MCRoundDuration/bz.SCRoundDuration) == int(bz.SCRoundNumber[name].Load()) {
		blsCosi := bz.BlsCosi.Load()
		blsCosi.BlockType = "Summary Block"
		bz.BlsCosi.Store(blsCosi) // just to know!
		// ---
		bz.BCLock.Lock()
		// ----------------------------------------------------------------------------------------------------------------
		// ---------------------------------------------- Blockchain Operations -------------------------------------------
		// ----------------------------------------------------------------------------------------------------------------
		// issueing a sync transaction from last submitted summary block to the main chain
		blocksize = bz.syncMainChainBCTransactionQueueCollect(name)
		if name == "marketmatch" {
			bz.ActivateContracts()
		}
		// todoCoder: add description
		if bz.StoragePaymentEpoch != 0 && (int(bz.MCRoundNumber.Load())/bz.MCRoundPerEpoch%bz.StoragePaymentEpoch) == 0 {
			err = bz.StoragePaymentMainChainBCTransactionQueueCollect()
			if err != nil {
				return xerrors.New("can't issue StoragePayment at the end of epoch" + err.Error())
			}
		}
		// ----------------------------------------------------------------------------------------------------------------
		// 									-------------------------------------------------
		// ----------------------------------------------------------------------------------------------------------------
		//update the last row in round table with summary block's size
		// in this round in which a summary block will be generated, new transactions will be added to the queue but not taken
		bz.updateSideChainBCRound(name, leaderName, blocksize)
		// ---
		bz.BCLock.Unlock()
		// ---
		// reset side chain round number
		bz.SCRoundNumber[name].Store(1) // in side chain round number zero the summary blocks are published in side chain
		// ------------- Epoch changed -----------
		// i.e. the current published block on side chain is summary block
		log.Lvl1("Final result SC: BlsCosi: the Summary Block was for epoch number: ", int(bz.MCRoundNumber.Load())/bz.MCRoundPerEpoch)

		// -----------------------------------------------
		// -----------------------------------------------
		log.Lvl1("Coder Debug: wgSCRound.Done")
		//bz.wgSCRound.Done()
		bz.SCTools[name].WgSyncScRound.Done(name + "bz.wgSyncScRound")
		// --------------------------------------------------------------------
		log.Lvl1("Coder Debug: wgMCRound.Wait")
		//bz.wgMCRound.Wait()
		log.Lvl1("Coder Debug: wgMCRound.Wait: PASSED")
		// each epoch this number of mc rounds should be passed
		log.Lvl1("Coder Debug: wgMCRound.Add(", bz.MCRoundPerEpoch, ")")
		//bz.wgMCRound.Add(bz.MCRoundPerEpoch)
		// -----------------------------------------------
		// -----------------------------------------------

		if name == "por" {
			// change next epoch's committee:
			//tempCommitteeNodesTreeNodeID = bz.CommitteeNodesTreeNodeID[len(bz.CommitteeNodesTreeNodeID)-(bz.CommitteeWindow):]
			tempCommitteeNodesTreeNodeID = bz.CommitteeNodesTreeNodeID1[0 : bz.CommitteeWindow-1]
			// changing next side chain's leader for the next epoch rounds from the last miner in the main chain's window of miners
			bz.NextSideChainLeader1 = tempCommitteeNodesTreeNodeID[0]
			// changing side chain's committee to last miners in the main chain's window of miners
			bz.CommitteeNodesTreeNodeID1 = tempCommitteeNodesTreeNodeID
			log.Lvl1("Final result SC: BlsCosi: next side chain's epoch leader is: ", bz.Tree().Search(bz.NextSideChainLeader1).Name())
			for i, a := range bz.CommitteeNodesTreeNodeID1 {
				log.Lvl3("Final result SC: BlsCosi: next side chain's epoch committee number ", i, ":", bz.Tree().Search(a).Name())
			}
		} else if name == "por2" {
			// change next epoch's committee:
			//tempCommitteeNodesTreeNodeID = bz.CommitteeNodesTreeNodeID[len(bz.CommitteeNodesTreeNodeID)-(bz.CommitteeWindow):]
			tempCommitteeNodesTreeNodeID = bz.CommitteeNodesTreeNodeID4[0 : bz.CommitteeWindow-1]
			// changing next side chain's leader for the next epoch rounds from the last miner in the main chain's window of miners
			bz.NextSideChainLeader4 = tempCommitteeNodesTreeNodeID[0]
			// changing side chain's committee to last miners in the main chain's window of miners
			bz.CommitteeNodesTreeNodeID4 = tempCommitteeNodesTreeNodeID
			log.Lvl1("Final result SC: BlsCosi: next side chain's epoch leader is: ", bz.Tree().Search(bz.NextSideChainLeader4).Name())
			for i, a := range bz.CommitteeNodesTreeNodeID1 {
				log.Lvl3("Final result SC: BlsCosi: next side chain's epoch committee number ", i, ":", bz.Tree().Search(a).Name())
			}
		} else if name == "dispute" {
			// change next epoch's committee:
			//tempCommitteeNodesTreeNodeID = bz.CommitteeNodesTreeNodeID[len(bz.CommitteeNodesTreeNodeID)-(bz.CommitteeWindow):]
			tempCommitteeNodesTreeNodeID = bz.CommitteeNodesTreeNodeID3[0 : bz.CommitteeWindow-1]
			// changing next side chain's leader for the next epoch rounds from the last miner in the main chain's window of miners
			bz.NextSideChainLeader3 = tempCommitteeNodesTreeNodeID[0]
			// changing side chain's committee to last miners in the main chain's window of miners
			bz.CommitteeNodesTreeNodeID3 = tempCommitteeNodesTreeNodeID
			log.Lvl1("Final result SC: BlsCosi: next side chain's epoch leader is: ", bz.Tree().Search(bz.NextSideChainLeader3).Name())
			for i, a := range bz.CommitteeNodesTreeNodeID3 {
				log.Lvl3("Final result SC: BlsCosi: next side chain's epoch committee number ", i, ":", bz.Tree().Search(a).Name())
			}
		} else {
			// change next epoch's committee:
			//tempCommitteeNodesTreeNodeID = bz.CommitteeNodesTreeNodeID[len(bz.CommitteeNodesTreeNodeID)-(bz.CommitteeWindow):]
			tempCommitteeNodesTreeNodeID = bz.CommitteeNodesTreeNodeID2[0 : bz.CommitteeWindow-1]
			// changing next side chain's leader for the next epoch rounds from the last miner in the main chain's window of miners
			bz.NextSideChainLeader2 = tempCommitteeNodesTreeNodeID[0]
			// changing side chain's committee to last miners in the main chain's window of miners
			bz.CommitteeNodesTreeNodeID2 = tempCommitteeNodesTreeNodeID
			log.Lvl1("Final result SC: BlsCosi: next side chain's epoch leader is: ", bz.Tree().Search(bz.NextSideChainLeader2).Name())
			for i, a := range bz.CommitteeNodesTreeNodeID2 {
				log.Lvl3("Final result SC: BlsCosi: next side chain's epoch committee number ", i, ":", bz.Tree().Search(a).Name())
			}
		}
	} else {
		// ---
		bz.BCLock.Lock()
		// ---
		// next meta block on side chain blockchian is added by the root node
		bz.updateSideChainBCRound(name, leaderName, 0) // we dont use blocksize param bcz when we are generating meta block
		// the block size is measured and added in the func: updateSideChainBCTransactionQueueTake
		blocksize = bz.updateSideChainBCTransactionQueueTake(name)
		blsCosi := bz.BlsCosi.Load()
		blsCosi.BlockType = "Meta Block"
		bz.BlsCosi.Store(blsCosi) // just to know!
		// ---
		bz.BCLock.Unlock()
		// ---
		// increase side chain round number
		bz.SCRoundNumber[name].Inc()
		log.Lvl1("Coder Debug: wgSCRound.Done")
		//bz.wgSCRound.Done()
		bz.SCTools[name].WgSyncScRound.Done(name + "bz.wgSyncScRound")
	}

	if !bz.simulationDone && int(bz.SCRoundNumber[name].Load()-1)%(bz.SCRoundPerEpoch/bz.MCRoundPerEpoch) == 0 {
		bz.wgSyncMcRound.Wait("bz.wgSyncMcRound")
		bz.wgSyncMcRound.Add("bz.wgSyncMcRound", 1)
	}

	// --------------------------------------------------------------------
	//triggering next side chain round leader to run next round of blscosi
	// --------------------------------------------------------------------
	if name == "por" {

		err = bz.SendTo(bz.Tree().Search(bz.NextSideChainLeader1), &RtLSideChainNewRound{
			SCRoundNumber:            int(bz.SCRoundNumber[name].Load()),
			CommitteeNodesTreeNodeID: tempCommitteeNodesTreeNodeID,
			blocksize:                blocksize,
		})
		if err != nil {
			log.LLvl1(bz.Name(), "can't send new side chain round msg to", bz.Tree().Search(bz.NextSideChainLeader1).Name())
			return xerrors.New("can't send new side chain round msg to next leader" + err.Error())
		}
	} else if name == "por2" {

		err = bz.SendTo(bz.Tree().Search(bz.NextSideChainLeader4), &RtLSideChainNewRoundChain4{
			SCRoundNumber:            int(bz.SCRoundNumber[name].Load()),
			CommitteeNodesTreeNodeID: tempCommitteeNodesTreeNodeID,
			blocksize:                blocksize,
		})
		if err != nil {
			log.LLvl1(bz.Name(), "can't send new side chain round msg to", bz.Tree().Search(bz.NextSideChainLeader1).Name())
			return xerrors.New("can't send new side chain round msg to next leader" + err.Error())
		}
	} else if name == "dispute" {
		err = bz.SendTo(bz.Tree().Search(bz.NextSideChainLeader3), &RtLSideChainNewRoundChain3{
			SCRoundNumber:            int(bz.SCRoundNumber[name].Load()),
			CommitteeNodesTreeNodeID: tempCommitteeNodesTreeNodeID,
			blocksize:                blocksize,
		})
		if err != nil {
			log.LLvl1(bz.Name(), "can't send new side chain round msg to", bz.Tree().Search(bz.NextSideChainLeader1).Name())
			return xerrors.New("can't send new side chain round msg to next leader" + err.Error())
		}
	} else {
		err = bz.SendTo(bz.Tree().Search(bz.NextSideChainLeader2), &RtLSideChainNewRoundChain2{
			SCRoundNumber:            int(bz.SCRoundNumber[name].Load()),
			CommitteeNodesTreeNodeID: tempCommitteeNodesTreeNodeID,
			blocksize:                blocksize,
		})
		if err != nil {
			log.LLvl1(bz.Name(), "can't send new side chain round msg to", bz.Tree().Search(bz.NextSideChainLeader2).Name())
			return xerrors.New("can't send new side chain round msg to next leader" + err.Error())
		}
	}

	return nil
}

// ----------------------------------------------------------------------------------------------
// ---------------- BLS CoSi protocol (Initialization for root node) --------
// ----------------------------------------------------------------------------------------------
func (bz *ChainBoost) StartSideChainProtocol() {
	var err error

	//bz.BCLock.Lock()
	//bz.updateSideChainBCTransactionQueueCollect()
	//bz.BCLock.Unlock()

	// -----------------------------------------------
	// --- initializing side chain's msg and side chain's committee roster index for the second run and the next runs
	// -----------------------------------------------
	rand := rand.New(rand.NewSource(int64(bz.SimulationSeed)))
	var r []int
	var d int
	for i := 0; i < len(bz.Sidechains)*bz.CommitteeWindow; i++ {
		d = int(math.Abs(float64(int(rand.Uint64())))) % len(bz.Tree().List())
		if !contains(r, d) {
			r = append(r, d)
		} else {
			i--
		}
	}

	for i := 0; i < bz.CommitteeWindow; i++ {
		d := r[i]
		bz.CommitteeNodesTreeNodeID1 = append(bz.CommitteeNodesTreeNodeID1, bz.Tree().List()[d].ID)
		d = r[i+bz.CommitteeWindow]
		bz.CommitteeNodesTreeNodeID2 = append(bz.CommitteeNodesTreeNodeID2, bz.Tree().List()[d].ID)
		d = r[i+2*bz.CommitteeWindow]
		bz.CommitteeNodesTreeNodeID3 = append(bz.CommitteeNodesTreeNodeID3, bz.Tree().List()[d].ID)
		d = r[i+3*bz.CommitteeWindow]
		bz.CommitteeNodesTreeNodeID4 = append(bz.CommitteeNodesTreeNodeID4, bz.Tree().List()[d].ID)
	}
	for i, a := range bz.CommitteeNodesTreeNodeID1 {
		log.Lvl1("final result SC: Chain 1 Initial BlsCosi committee queue: ", i, ":", bz.Tree().Search(a).Name())
	}
	for i, a := range bz.CommitteeNodesTreeNodeID2 {
		log.Lvl1("final result SC: Chain 2 Initial BlsCosi committee queue: ", i, ":", bz.Tree().Search(a).Name())
	}
	for i, a := range bz.CommitteeNodesTreeNodeID3 {
		log.Lvl1("final result SC: Chain 3 Initial BlsCosi committee queue: ", i, ":", bz.Tree().Search(a).Name())
	}
	for i, a := range bz.CommitteeNodesTreeNodeID4 {
		log.Lvl1("final result SC: Chain 4 Initial BlsCosi committee queue: ", i, ":", bz.Tree().Search(a).Name())
	}
	bz.NextSideChainLeader1 = bz.CommitteeNodesTreeNodeID1[0]
	bz.NextSideChainLeader2 = bz.CommitteeNodesTreeNodeID2[0]
	bz.NextSideChainLeader3 = bz.CommitteeNodesTreeNodeID3[0]
	bz.NextSideChainLeader4 = bz.CommitteeNodesTreeNodeID4[0]
	// -----------------------------------------------
	// --- initializing next side chain's leader
	// -----------------------------------------------
	//bz.CommitteeNodesTreeNodeID = append([]onet.TreeNodeID{bz.Tree().List()[bz.CommitteeWindow+1].ID}, bz.CommitteeNodesTreeNodeID[:bz.CommitteeWindow-1]...)
	//bz.NextSideChainLeader = bz.Tree().List()[bz.CommitteeWindow+1].ID
	// --------------------------------------------------------------------
	// triggering next side chain round leader to run next round of blscosi
	// --------------------------------------------------------------------
	blsCosi := bz.BlsCosi.Load()
	blsCosi.Msg = []byte{0xFF}
	bz.BlsCosi.Store(blsCosi)
	err = bz.SendTo(bz.Tree().Search(bz.NextSideChainLeader1), &RtLSideChainNewRound{
		SCRoundNumber:            int(bz.SCRoundNumber["por"].Load()),
		CommitteeNodesTreeNodeID: bz.CommitteeNodesTreeNodeID1,
	})
	if err != nil {
		log.LLvl1(bz.Name(), "can't send new side chain round msg to", bz.Tree().Search(bz.NextSideChainLeader1).Name())
		panic("can't send new side chain round msg to the first leader")
	}

	err = bz.SendTo(bz.Tree().Search(bz.NextSideChainLeader2), &RtLSideChainNewRoundChain2{
		SCRoundNumber:            int(bz.SCRoundNumber["marketmatch"].Load()),
		CommitteeNodesTreeNodeID: bz.CommitteeNodesTreeNodeID2,
	})

	if err != nil {
		log.LLvl1(bz.Name(), "can't send new side chain round msg to", bz.Tree().Search(bz.NextSideChainLeader2).Name())
		panic("can't send new side chain round msg to the first leader")
	}

	err = bz.SendTo(bz.Tree().Search(bz.NextSideChainLeader3), &RtLSideChainNewRoundChain3{
		SCRoundNumber:            int(bz.SCRoundNumber["dispute"].Load()),
		CommitteeNodesTreeNodeID: bz.CommitteeNodesTreeNodeID3,
	})
	if err != nil {
		log.LLvl1(err)
		log.LLvl1(bz.Name(), "can't send new side chain round msg to", bz.Tree().Search(bz.NextSideChainLeader3).Name())
		panic("can't send new side chain round msg to the first leader")
	}

	err = bz.SendTo(bz.Tree().Search(bz.NextSideChainLeader4), &RtLSideChainNewRoundChain4{
		SCRoundNumber:            int(bz.SCRoundNumber["por2"].Load()),
		CommitteeNodesTreeNodeID: bz.CommitteeNodesTreeNodeID4,
	})
	if err != nil {
		log.LLvl1(err)
		log.LLvl1(bz.Name(), "can't send new side chain round msg to", bz.Tree().Search(bz.NextSideChainLeader3).Name())
		panic("can't send new side chain round msg to the first leader")
	}
}

/*
	-----------------------------------------------

dynamically change the side chain's committee with last main chain's leader
the committee nodes is shifted by one and the new leader is added to be used for next epoch's side chain's committee
Note that: for now we are considering the last w distinct leaders in the committee which means
if a leader is selected multiple times during an epoch, he will not be added multiple times,
-----------------------------------------------
*/

func (bz *ChainBoost) UpdateSCCommittee(array []onet.TreeNodeID, toAdd onet.TreeNodeID) []onet.TreeNodeID {
	t := 0
	for _, a := range array {
		if a != toAdd {
			t = t + 1
			continue
		} else {
			break
		}
	}
	if t != len(array) {

		NextSideChainLeaderTreeNodeID := toAdd
		var NewCommitteeNodesTreeNodeID []onet.TreeNodeID

		//for _, a := range bz.CommitteeNodesTreeNodeID {
		for i := len(array) - 1; i >= 0; i-- {
			if array[i] != toAdd {
				NewCommitteeNodesTreeNodeID = append([]onet.TreeNodeID{array[i]}, NewCommitteeNodesTreeNodeID...)
			}
		}
		array = append([]onet.TreeNodeID{NextSideChainLeaderTreeNodeID}, NewCommitteeNodesTreeNodeID...)
		// ------------------------------------
		log.Lvl1("final result SC:", bz.Tree().Search(toAdd).Name(), "is already in the committee")
		for i, a := range array {
			log.Lvl1("Chain: final result SC: BlsCosi committee queue: ", i, ":", bz.Tree().Search(a).Name())
		}
		// ------------------------------------
	} else {
		NextSideChainLeaderTreeNodeID := toAdd
		array = append([]onet.TreeNodeID{NextSideChainLeaderTreeNodeID}, array...)
		// ------------------------------------
		log.Lvl1("final result SC:", bz.Tree().Search(toAdd).Name(), "is added to side chain for the next epoch's committee")
		for i, a := range array {
			log.Lvl1("final result SC: BlsCosi committee queue: ", i, ":", bz.Tree().Search(a).Name())
		}
		// ------------------------------------
	}
	return array
}

func (bz *ChainBoost) UpdateSideChainCommittee(msg MainChainNewLeaderChan) {
	rand := rand.New(rand.NewSource(int64(bz.SimulationSeed)))
	var r []int
	var d int
	for i := 0; i < len(bz.Sidechains)*bz.CommitteeWindow; i++ {
		d = int(math.Abs(float64(int(rand.Uint64())))) % len(bz.Tree().List())
		if !contains(r, d) {
			r = append(r, d)
		} else {
			i--
		}
	}

	for i := 0; i < bz.CommitteeWindow; i++ {
		d := r[i]
		bz.CommitteeNodesTreeNodeID1 = append(bz.CommitteeNodesTreeNodeID1, bz.Tree().List()[d].ID)
		d = r[i+bz.CommitteeWindow]
		bz.CommitteeNodesTreeNodeID2 = append(bz.CommitteeNodesTreeNodeID2, bz.Tree().List()[d].ID)
		d = r[i+2*bz.CommitteeWindow]
		bz.CommitteeNodesTreeNodeID3 = append(bz.CommitteeNodesTreeNodeID3, bz.Tree().List()[d].ID)
		d = r[i+3*bz.CommitteeWindow]
		bz.CommitteeNodesTreeNodeID4 = append(bz.CommitteeNodesTreeNodeID4, bz.Tree().List()[d].ID)
	}
}

// this code block was used in hello ChainBoost to start the side chain protocol -- its here to keep back-up in case we noticed an unresolved issue in side chain run
// ----------------------------------------------------------------------------------------------
// ---------------- BLS CoSi protocol (running for the very first time) --------
// ----------------------------------------------------------------------------------------------
// this node has been set to start running blscosi in simulation level. (runsimul.go):
/* 	if bz.Tree().List()[bz.CommitteeWindow] == bz.TreeNode() {
	// -----------------------------------------------
	// --- initializing side chain's msg and side chain's committee roster index for the first run
	// -----------------------------------------------
	for i := 0; i < bz.CommitteeWindow; i++ {
		bz.CommitteeNodesTreeNodeID = append(bz.CommitteeNodesTreeNodeID, bz.Tree().List()[i].ID)
	}
	bz.BlsCosi.Msg = []byte{0xFF}

	bz.CommitteeNodesTreeNodeID = append([]onet.TreeNodeID{bz.Tree().List()[bz.CommitteeWindow].ID}, bz.CommitteeNodesTreeNodeID[:bz.CommitteeWindow-1]...)
	var CommitteeNodesServerIdentity []*network.ServerIdentity
	for _, a := range bz.CommitteeNodesTreeNodeID {
		CommitteeNodesServerIdentity = append(CommitteeNodesServerIdentity, bz.Tree().Search(a).ServerIdentity)
	}
	committeeRoster := onet.NewRoster(CommitteeNodesServerIdentity)
	// --- root should have root index of 0 (this is based on what happens in gen_tree.go)
	var x = *bz.Tree().List()[bz.CommitteeWindow]
	x.RosterIndex = 0
	// ---
	bz.BlsCosi.SubTrees, err = protocol.NewBlsProtocolTree(onet.NewTree(committeeRoster, &x), bz.NbrSubTrees)
	if err == nil {
		log.LLvl1("final result: BlsCosi: First bls cosi tree is: ", bz.BlsCosi.SubTrees[0].Roster.List,
			" with ", bz.BlsCosi.Name(), " as Root starting BlsCosi")
	} else {
		log.LLvl1("Coder: error: ", err)
	}
	// ------------------------------------------
	err = bz.BlsCosi.Start() // first leader in side chain is this node
	if err != nil {
		log.LLvl1(bz.Info(), "couldn't start side chain")
	}
} */

// utility
func contains(s []int, e int) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}
