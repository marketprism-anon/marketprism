package MainAndSideChain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math"
	"math/rand"
	"time"

	"github.com/chainBoostScale/ChainBoost/MainAndSideChain/blockchain"
	"github.com/chainBoostScale/ChainBoost/onet/log"
)

/*
	 ----------------------------------------------------------------------
		finalMainChainBCInitialization initialize the mainchainbc file based on the config params defined in the config file
		(.toml file of the protocol) the info we hadn't before and we have now is nodes' info that this function add to the mainchainbc file

------------------------------------------------------------------------
*/
func (bz *ChainBoost) finalMainChainBCInitialization() {
	var NodeInfoRow []string
	for _, a := range bz.Roster().List {
		NodeInfoRow = append(NodeInfoRow, a.String())
	}
	var err error
	// fill nodes info and get the maximum file size
	maxFileSize, err := blockchain.GetMaxMainChain("MarketMatching", "FileSize")
	if err != nil {
		log.Fatal(err)
	}
	argvars := make([]interface{}, 0, len(NodeInfoRow))
	for _, element := range NodeInfoRow {
		argvars = append(argvars, element)
	}
	// list of servers holding contracts, multiple contracts are considered for one server
	argvars2 := make([]interface{}, 0, len(NodeInfoRow)*bz.NumberOfActiveContractsPerServer)
	for _, element := range NodeInfoRow {
		for i := 1; i <= bz.NumberOfActiveContractsPerServer; i++ {
			argvars2 = append(argvars2, element)
		}
	}
	err = blockchain.AddMoreFieldsIntoTableInMainChain("MarketMatching", "ServerInfo", argvars2...)
	if err != nil {
		log.Fatal(err)
	}
	bz.maxFileSize = int(maxFileSize)

	/*// --- sum of server agreement file size
	_ = f.NewSheet("ExtraInfo")
	if err = f.SetCellValue("ExtraInfo", "B1", "sum of file size"); err != nil {
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}

	FormulaString := "=SUM(MarketMatching!B2:B" + strconv.Itoa(len(NodeInfoRow)+1) + ")"
	err = f.SetCellFormula("ExtraInfo", "B2", FormulaString)
	if err != nil {
		log.LLvl1(err)
	}
	*/ // Can be obtained from Table using formulas
	// --- power table sheet
	err = blockchain.InitialInsertValuesIntoMainChainPowerTable(argvars...)
	if err != nil {
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}
}

/*
	----------------------------------------------------------------------

each round THE ROOT NODE send a msg to all nodes,
let other nodes know that the new round has started and the information they need
from blockchain to check if they are next round's leader
------------------------------------------------------------------------
*/
func (bz *ChainBoost) readBCAndSendtoOthers() {
	if bz.MCRoundNumber.Load() == int64(bz.SimulationRounds) {
		bz.MCRoundNumber.Inc()
		log.LLvl1("ChainBoost simulation has passed the number of simulation rounds:", bz.SimulationRounds, "\n returning back to RunSimul")
		bz.DoneRootNode <- true
		return
	}
	takenTime := time.Now()
	powers, seed := bz.readBCPowersAndSeed()
	bz.MCRoundNumber.Inc()
	// ---
	//bz.MCLeader.MCPLock.Lock()
	bz.MCLeader.HasLeader = false
	//bz.MCLeader.MCPLock.Unlock()
	// ---
	for _, b := range bz.Tree().List() {
		power, found := powers[b.ServerIdentity.String()]
		//log.LLvl1(power, "::", found)
		if found && !b.IsRoot() {
			err := bz.SendTo(b, &NewRound{
				Seed:        seed,
				Power:       power,
				MaxFileSize: bz.maxFileSize,
			})
			if err != nil {
				log.LLvl1(bz.Info(), "can't send new round msg to", b.Name())
				panic(err)
			} else {
				log.Lvl5(b.Name(), "recieved NewRound from", bz.TreeNode().Name(), "with maxFileSize value of:", bz.maxFileSize)
			}
		}
	}
	// detecting leader-less in next round
	go bz.startTimer(int(bz.MCRoundNumber.Load()))
	log.Lvl1("readBCAndSendtoOthers took:", time.Since(takenTime).String(), "for mc round number", bz.MCRoundNumber)
}

/* ----------------------------------------------------------------------*/
func (bz *ChainBoost) readBCPowersAndSeed() (minerspowers map[string]int, seed string) {
	var err error
	takenTime := time.Now()
	seed, err = blockchain.MainChainGetLastRoundSeed()
	if err != nil {
		log.Fatal(err)
	}

	minerspowers, err = blockchain.MainChainGetPowerTable()
	if err != nil {
		log.Fatal(err)
	}

	log.Lvl1("readBCPowersAndSeed took:", time.Since(takenTime).String(), "for mc round number", bz.MCRoundNumber.Load()+1)
	return minerspowers, seed
}

/*
	 ----------------------------------------------------------------------
		updateBC: by leader
		each round, adding one row in power table based on the information in market matching sheet,
		assuming that servers are honest and have honestly publish por for their actice (not expired) ServAgrs,
		for each storage server and each of their active ServAgr, add the stored file size to their current power
	 ----------------------------------------------------------------------
*/

func (bz *ChainBoost) UpdateFaultyContracts() {
	err := blockchain.UpdateActiveFaultyContracts(bz.FaultyContractsRate)
	if err != nil {
		panic(err)
	}
}

func (bz *ChainBoost) updateBCPowerRound(LeaderName string, leader bool) {
	var seed string
	var err error
	takenTime := time.Now()
	seed, err = blockchain.MainChainGetLastRoundSeed()
	nextRoundNumber := int(bz.MCRoundNumber.Load() + 1)
	data := fmt.Sprintf("%v", seed)
	sha := sha256.New()
	if _, err = sha.Write([]byte(data)); err != nil {
		log.Error("Couldn't hash header:", err)
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}
	hash := sha.Sum(nil)
	nextSeed := hex.EncodeToString(hash)

	err = blockchain.InsertIntoMainChainRoundTable(nextRoundNumber, nextSeed, 0, LeaderName, 0, 0, 0, 0, 0, time.Now(), 0, 0, 0, false, false, false)
	if err != nil {
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}
	// Each round, adding one row in power table based on the information in market matching sheet,
	// assuming that servers are honest  and have honestly publish por for their actice (not expired)
	// ServAgrs,for each storage server and each of their active contracst,
	// add the stored file size to their current power
	rowsMarketMatching, err := blockchain.MainChainGetMarketMatchingRows()
	if err != nil {
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}
	MinerServers := make(map[string]int)
	for _, item := range rowsMarketMatching {
		if bz.MCRoundNumber.Load()-int64(item.StartedMcRoundNumber) <= int64(item.ServerAgrDuration) {
			MinerServers[item.MinerServer] = MinerServers[item.MinerServer] + item.FileSize //if each server one ServAgr
		} else {
			// MinerServers[item.MinerServer] = 0
			// Do Nothing!

			//ToDO: later add power just when the server has submitted por
			log.Lvl3(item.MinerServer, "has no acctive contract in this round")
		}
	}
	//
	// ---------------------------------------------------------------------
	// --- Power Table sheet  ----------------------------------------------
	// ---------------------------------------------------------------------
	/* todo: Power has been  added without considering por tx.s not published (waiting in queue yet)
	=> fix it: use TxPayable column (first set it to 2 when taken and second change its name to TXonQ), so if
	contract is publlished (1) but TxonQ is taken (2) then add power  */

	err = blockchain.MainChainUpdatePowerTable(MinerServers)

	if err != nil {
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	} else {
		log.Lvl2("bc Successfully closed")
		log.Lvl1("updateBCPowerRound took:", time.Since(takenTime).String(), "for mc round number", bz.MCRoundNumber)

	}
}

/*
	----------------------------------------------------------------------
	   updateBC: this is a connection between first layer of blockchain - ROOT NODE - on the second layer - xlsx file -

------------------------------------------------------------------------
*/
func (bz *ChainBoost) updateMainChainBCTransactionQueueCollect() {
	takenTime := time.Now()
	var err error

	rowsMarketMatching, err := blockchain.MainChainGetMarketMatchingRows()
	if err != nil {
		log.Fatal(err)
	}
	// ----------------------------------------------------------------------
	// ------ add 5 types of transactions into transaction queue sheet -----
	// ----------------------------------------------------------------------
	/* each transaction has the following column stored on the transaction queue sheet:
	0) name
	1) size
	2) time
	3) issuedMCRoundNumber
	4) ServAgrId */
	//-----------------------------------------

	// this part can be moved to protocol initialization
	var PorTxSize, ServAgrProposeTxSize, PayTxSize, StoragePayTxSize, ServAgrCommitTxSize, FinalCommitSize uint32
	PorTxSize, ServAgrProposeTxSize, PayTxSize, StoragePayTxSize, ServAgrCommitTxSize, FinalCommitSize = blockchain.TransactionMeasurement(bz.SectorNumber, bz.SimulationSeed)

	var numOfPoRTxsMC, numOfPoRTxsSC, numOfServAgrProposeTxs, numOfStoragePaymentTxs, numOfServAgrCommitTxs, numOfRegularPaymentTxs, numTx int

	// -------------------------------------------------------------------------------
	mcFirstQueueTxs := make([]blockchain.MainChainFirstQueueEntry, 0)
	scFirstQueueTxs := make(map[string][]blockchain.SideChainFirstQueueEntry)
	for _, sc := range bz.Sidechains {
		scFirstQueueTxs[sc] = make([]blockchain.SideChainFirstQueueEntry, 0)
	}
	// --- check for contracts states and add appropriate tx row on top of the stream ----
	for i := range rowsMarketMatching {
		//--------------------------------------------------------------------------------------------------------------------------------------
		// --------------------------------------------- when the servic agreement is inactive ---------------------------------------------
		//--------------------------------------------------------------------------------------------------------------------------------------
		log.LLvl1("Round: ", bz.MCRoundNumber.Load(), "Row: ", rowsMarketMatching[i])
		if !rowsMarketMatching[i].Published && !rowsMarketMatching[i].TxIssued {

			epoch := -1
			if bz.MCRoundNumber.Load() != 1 {
				epoch = int(bz.MCRoundNumber.Load()-2) / bz.MCRoundPerEpoch
			}
			issuedRoundNumber := (int(bz.SCRoundNumber["marketmatch"].Load()-1) % bz.SCRoundPerEpoch)
			if issuedRoundNumber == 0 {
				issuedRoundNumber = bz.SCRoundPerEpoch
			}

			if gt.MMActivationTimes[i] != 0 {
				tx := blockchain.SideChainFirstQueueEntry{Name: "TxServAgrPropose", Size: int(ServAgrProposeTxSize), Time: time.Now(), IssuedScRoundNumber: issuedRoundNumber, ServAgrId: i + 1, Epoch: epoch}
				scFirstQueueTxs["marketmatch"] = append(scFirstQueueTxs["marketmatch"], tx)
				numOfServAgrProposeTxs++
				numTx++
				tx = blockchain.SideChainFirstQueueEntry{Name: "TxServAgrCommit", Size: int(ServAgrCommitTxSize), Time: time.Now(), IssuedScRoundNumber: issuedRoundNumber, ServAgrId: i + 1, Epoch: epoch}
				scFirstQueueTxs["marketmatch"] = append(scFirstQueueTxs["marketmatch"], tx)
				numOfServAgrCommitTxs++
				numTx++
				gt.MMActivationTimes[i]--
			} else {
				tx := blockchain.SideChainFirstQueueEntry{Name: "TxServDeal", Size: int(FinalCommitSize), Time: time.Now(), IssuedScRoundNumber: issuedRoundNumber, ServAgrId: i + 1, Epoch: epoch}
				scFirstQueueTxs["marketmatch"] = append(scFirstQueueTxs["marketmatch"], tx)
				numTx++
				err = blockchain.MainChainSetTxIssued(i+1, true)
				if err != nil {
					panic(err)
				}

			}

			//--------------------------------------------------------------------------------------------------------------------------------------
			// --------------------------------------------- when the service agreement expires ---------------------------------------------
			//--------------------------------------------------------------------------------------------------------------------------------------
			//-------------------------------------------------------------------
			// bz.StoragePaymentEpoch == 0 means: settle the service payment when the servic agreement expires,
			//-------------------------------------------------------------------
		} else if bz.MCRoundNumber.Load()-int64(rowsMarketMatching[i].StartedMcRoundNumber) > int64(rowsMarketMatching[i].ServerAgrDuration) && rowsMarketMatching[i].Published && rowsMarketMatching[i].TxIssued {
			if bz.StoragePaymentEpoch != 0 {
				// Set ServAgrPublished AND ServAgrTxsIssued to false
				// --------------------------------------
				// i start from index 0, serverAgreementID strats from index 2 which is its rownumber
				err = blockchain.MainChainSetPublished(i+1, false)
				if err != nil {
					panic(err)
				}
				// i start from index 0, serverAgreementID strats from index 2 which is its rownumber
				err = blockchain.MainChainSetTxIssued(i+1, false)
				if err != nil {
					panic(err)
				}
			} else if bz.StoragePaymentEpoch == 0 {
				// Set ServAgrPublished AND ServAgrTxsIssued to false
				// --------------------------------------
				// i start from index 0, serverAgreementID strats from index 2 which is its rownumber
				err = blockchain.MainChainSetPublished(i+1, false)
				if err != nil {
					panic(err)
				}
				// i start from index 0, serverAgreementID strats from index 2 which is its rownumber
				err = blockchain.MainChainSetTxIssued(i+1, false)
				if err != nil {
					panic(err)
				}

				tx := blockchain.MainChainFirstQueueEntry{Name: "TxStoragePayment", Size: StoragePayTxSize, Time: time.Now(), RoundIssued: int(bz.MCRoundNumber.Load()), ServAgrId: i + 1}
				mcFirstQueueTxs = append(mcFirstQueueTxs, tx)
				numTx++
				gt.MMActivationTimes[i] = rowsMarketMatching[i].NegotiationDuration // Go To Negotiation
				numOfStoragePaymentTxs++
			}

			//--------------------------------------------------------------------------------------------------------------------------------------
			// --------------------------------------------- when the ServAgr is not expired => Add TxPor ---------------------------------------------
			//--------------------------------------------------------------------------------------------------------------------------------------
		} else if rowsMarketMatching[i].Published && int(bz.MCRoundNumber.Load())-rowsMarketMatching[i].StartedMcRoundNumber <= rowsMarketMatching[i].ServerAgrDuration && bz.SimState == 1 {
			// --- Add TxPor

			var txName string
			if rowsMarketMatching[i].IsFaulty {
				txName = "TxDispute"
			} else {
				txName = "TxPor"
			}
			tx := blockchain.MainChainFirstQueueEntry{Name: txName, Size: PorTxSize, Time: time.Now(), RoundIssued: int(bz.MCRoundNumber.Load()), ServAgrId: i + 1}

			mcFirstQueueTxs = append(mcFirstQueueTxs, tx)
			numTx++
			numOfPoRTxsMC++
			//-------------------------------------------------------------------
			// simStat == 2 means that side chain is running => por tx.s go to side chain queue
			//-------------------------------------------------------------------
		} else if rowsMarketMatching[i].Published && int(bz.MCRoundNumber.Load())-rowsMarketMatching[i].StartedMcRoundNumber <= rowsMarketMatching[i].ServerAgrDuration && bz.SimState == 2 {
			// -------------------------------------------------------------------------------
			//        -------- updateSideChainBCTransactionQueueCollect  --------
			// -------------------------------------------------------------------------------
			// --- check for eligible contracts and add a por tx row on top of the stream ----
			// ServAgr is not expired => Add TxPor

			var chain string = "por"
			var txName string
			if rowsMarketMatching[i].IsFaulty {
				txName = "TxDispute"
				chain = "dispute"
			} else {
				txName = "TxPor"
				chain = "por"
			}
			epoch := -1
			if bz.MCRoundNumber.Load() != 1 {
				epoch = int(bz.MCRoundNumber.Load()-2) / bz.MCRoundPerEpoch
			}
			issuedRoundNumber := (int(bz.SCRoundNumber[chain].Load()-1) % bz.SCRoundPerEpoch)
			if issuedRoundNumber == 0 {
				issuedRoundNumber = bz.SCRoundPerEpoch
			}
			tx := blockchain.SideChainFirstQueueEntry{Name: txName, Size: int(PorTxSize), Time: time.Now(), IssuedScRoundNumber: issuedRoundNumber, ServAgrId: i + 1, Epoch: epoch}
			scFirstQueueTxs[chain] = append(scFirstQueueTxs[chain], tx)
			numTx++
			//-----------------------------------------------------------
			// end of side chain operations
			// -------------------------------------------------------------------------------
		}
	}
	for i := 0; i < len(mcFirstQueueTxs); i += 2000 {
		limit := math.Min(float64(len(mcFirstQueueTxs)), float64(i+2000))
		err = blockchain.BulkInsertIntoMainChainFirstQueue(mcFirstQueueTxs[i:int(limit)])
		if err != nil {
			panic(err)
		}
	}

	for k, queue := range scFirstQueueTxs {
		if len(queue) == 0 {
			continue
		}
		for i := 0; i < len(queue); i += 2000 {
			limit := math.Min(float64(len(queue)), float64(i+2000))
			err = blockchain.BulkInsertIntoSideChainFirstQueue(k, queue[i:int(limit)])
			if err != nil {
				panic(err)
			}
		}
	}

	// -------------------------------------------------------------------------------
	// ------ add payment transactions into transaction queue payment sheet
	// -------------------------------------------------------------------------------
	rand.Seed(int64(bz.SimulationSeed))
	// avoid having zero regular payment txs
	var numberOfRegPay int
	if bz.PayPercentOfTransactions == 0 || bz.PayPercentOfTransactions > 1 {
		for numberOfRegPay == 0 {
			numberOfRegPay = rand.Intn(bz.NumberOfPayTXsUpperBound)
		}
	} else {
		totalNonPayTx := numTx
		numberOfRegPay = int((bz.PayPercentOfTransactions * float64(totalNonPayTx)) / (1 - bz.PayPercentOfTransactions))
	}
	// -------------------------------------------------------------------
	// ------ add payment transactions into second queue stream writer
	// -------------------------------------------------------------------
	/* each transaction has the following column stored on the transaction queue payment sheet:
	0) size
	1) time
	2) issuedMCRoundNumber */

	mcSecondQueueTxs := make([]blockchain.MainChainSecondQueueEntry, 0)
	for i := 1; i <= numberOfRegPay; i++ {
		tx := blockchain.MainChainSecondQueueEntry{Size: PayTxSize, Time: time.Now(), RoundIssued: int(bz.MCRoundNumber.Load())}
		mcSecondQueueTxs = append(mcSecondQueueTxs, tx)
		numOfRegularPaymentTxs++
	}
	for i := 0; i < len(mcSecondQueueTxs); i += 2000 {
		limit := math.Min(float64(len(mcSecondQueueTxs)), float64(i+2000))
		blockchain.BulkInsertIntoMainChainSecondQueue(mcSecondQueueTxs[i:int(limit)])
		if err != nil {
			panic(err)
		}
	}
	log.Lvl1(bz.Name(), " finished collecting new transactions to mainchain queues in mc round number ", bz.MCRoundNumber, "in total:\n ",
		numOfPoRTxsMC, "numOfPoRTxsMC\n", numOfServAgrProposeTxs, "numOfServAgrProposeTxs\n",
		numOfStoragePaymentTxs, "numOfStoragePaymentTxs\n", numOfServAgrCommitTxs, "numOfServAgrCommitTxs\n",
		numOfRegularPaymentTxs, "numOfRegularPaymentTxs added to the main chain queus\n",
		"and ", numOfPoRTxsSC, "numOfPoRTxsSC added to the side chain queu")
	log.Lvl1("Collecting mc tx.s took:", time.Since(takenTime).String())
	log.Lvl4(bz.Name(), "Final result SC: finished collecting new transactions to side chain queue in sc round number ", bz.SCRoundNumber)
	log.Lvl1(numOfPoRTxsSC, "TxPor added to queue in sc round number: ", bz.SCRoundNumber)
	if err != nil {
		panic(err)
	}
}

/*
	----------------------------------------------------------------------
	   updateBC: this is a connection between first layer of blockchain - ROOT NODE - and the second layer - xlsx file -

------------------------------------------------------------------------
*/
func (bz *ChainBoost) updateMainChainBCTransactionQueueTake() {
	var err error
	// --- reset
	bz.FirstQueueWait = 0
	bz.SecondQueueWait = 0

	var accumulatedTxSize int
	blockIsFull := false
	regPayshareIsFull := false
	numberOfRegPayTx := 0
	BlockSizeMinusTransactions := blockchain.BlockMeasurement()
	var takenTime time.Time
	regPayTxShare := (bz.PercentageTxPay) * (bz.MainChainBlockSize - BlockSizeMinusTransactions)
	/* -----------------------------------------------------------------------------
		-- take regular payment transactions from sheet: SecondQueue
	----------------------------------------------------------------------------- */
	takenTime = time.Now()
	index := 0
	lastRowId := 0
	rows, err := blockchain.MainChainGetSecondQueue()
	if err != nil {
		panic(err)
	}

	for index = 0; index <= len(rows)-1 && !regPayshareIsFull; index++ {
		out := rows[index]

		/* each transaction has the following column stored on the Transaction Queue Payment sheet:
		0) size
		1) time
		2) issuedMCRoundNumber */

		if 100*(accumulatedTxSize+int(out.Size)) <= regPayTxShare {
			accumulatedTxSize = accumulatedTxSize + int(out.Size)

			bz.SecondQueueWait = bz.SecondQueueWait + int(bz.MCRoundNumber.Load()) - int(out.RoundIssued)
			lastRowId = out.RowId
			numberOfRegPayTx++

			log.Lvl2("a regular payment transaction added to block number", bz.MCRoundNumber, " from the queue")
		} else {
			regPayshareIsFull = true
			log.Lvl1("Final result MC: \n regular  payment share is full!")
			blockchain.AddToRoundTableBasedOnRoundNumber("RegSpaceFull", true, int(bz.MCRoundNumber.Load()))
			break
		}
		//}
		//}
	}
	blockchain.MainChainDeleteFromSecondQueue(lastRowId)

	log.Lvl1("reg pay tx.s taking took:", time.Since(takenTime).String())
	allocatedBlockSizeForRegPayTx := accumulatedTxSize
	otherTxsShare := bz.MainChainBlockSize - BlockSizeMinusTransactions - allocatedBlockSizeForRegPayTx
	/* -----------------------------------------------------------------------------
		 -- take 5 types of transactions from sheet: FirstQueue
	----------------------------------------------------------------------------- */
	// reset variables
	blockIsFull = false
	accumulatedTxSize = 0
	// new variables for first queue
	//var ServAgrIdCellMarketMatching []string

	numberOfPoRTx := 0
	numberOfStoragePayTx := 0
	numberOfServAgrProposeTx := 0
	numberOfServAgrCommitTx := 0
	numberOfSyncTx := 0
	SCPoRTx := 0

	txs, err := blockchain.MainChainPopFromSyncTxQueue()
	if err != nil {
		panic(err)
	}
	if txs == nil || len(txs) == 0 {
		log.LLvl1("Sync Transaction Not Found, MCRound: ", bz.MCRoundNumber.Load())

	}
	for _, tx := range txs {
		log.LLvl1("Sync Transaction Found, MCRound: ", bz.MCRoundNumber.Load())
		accumulatedTxSize = accumulatedTxSize + int(tx.Size)
		numberOfSyncTx++
		SCPoRTx = SCPoRTx + tx.ServAgrId
	}
	takenTime = time.Now()
	fqRows, err := blockchain.MainChainGetFirstQueue()
	if err != nil {
		panic(err)
	}
	index = 0
	lastRowId = 0
	for index = 0; index <= len(fqRows)-1 && !blockIsFull; index++ {

		out := fqRows[index]

		/* each transaction has the following column stored on the transaction queue sheet:

		0) name
		1) size
		2) time
		3) issuedMCRoundNumber
		4) ServAgrId (In case that the transaction type is a Sync transaction,
		the 4th column will be the number of por transactions summerized in
		the sync tx) */

		if accumulatedTxSize+int(out.Size) <= otherTxsShare {
			accumulatedTxSize = accumulatedTxSize + int(out.Size)
			/* transaction name in transaction queue can be
			"TxServAgrCommit",
			"TxServAgrPropose",
			"TxStoragePayment", or
			"TxPor"
			in case of "TxServAgrCommit":
			1) The corresponding ServAgr in marketmatching should be updated to published //ToDoCoder: replace the word "published" with "active"
			2) set start round number to current round
			other transactions are just removed from queue and their size are added to included transactions' size in block */

			switch out.Name {
			case "TxServAgrCommit":
				/* when tx TxServAgrCommit left queue:
				1) set ServAgrPublished to True
				2) set start round number to current round */

				err = blockchain.MainChainSetPublishedAndStartRoundOnServAgrId(out.ServAgrId, true, int(bz.MCRoundNumber.Load()))
				if err != nil {
					panic(err)
				}
				numberOfServAgrCommitTx++

			case "TxStoragePayment":
				log.Lvl4("a TxStoragePayment tx added to block number", bz.MCRoundNumber, " from the queue")
				numberOfStoragePayTx++
				// && bz.SimState == 1 is for backup check - the first condition shouldn't be true if the second one isn't
			case "TxPor":
				if bz.SimState == 1 {
					log.Lvl4("a por tx added to block number", bz.MCRoundNumber, " from the queue")
					numberOfPoRTx++
				}
			case "TxServAgrPropose":
				log.Lvl4("a TxServAgrPropose tx added to block number", bz.MCRoundNumber, " from the queue")
				numberOfServAgrProposeTx++
			case "TxSync":
				log.Lvl4("a sync tx added to block number", bz.MCRoundNumber, " from the queue")
				numberOfSyncTx++
				SCPoRTx = SCPoRTx + out.ServAgrId
			default:
				log.Lvl1("Panic Raised:\n\n")
				panic("the type of transaction in the queue is un-defined")
			}

			bz.FirstQueueWait = bz.FirstQueueWait + int(bz.MCRoundNumber.Load()) - out.RoundIssued
			lastRowId = out.RowId
			log.Lvl2("1 other tx is taken.")
		} else {
			blockIsFull = true
			log.Lvl1("Final result MC: \n block is full! ")
			err = blockchain.AddToRoundTableBasedOnRoundNumber("BlockSpaceFull", true, int(bz.MCRoundNumber.Load()))
			if err != nil {
				panic(err)
			}
			break
		}
	}
	err = blockchain.MainChainDeleteFromFirstQueue(lastRowId)
	if err != nil {
		panic(err)
	}
	log.Lvl1("other tx.s taking took:", time.Since(takenTime).String())

	log.Lvl2("In total in mc round number ", bz.MCRoundNumber,
		"\n number of published PoR transactions is", numberOfPoRTx,
		"\n number of published Storage payment transactions is", numberOfStoragePayTx,
		"\n number of published Propose ServAgr transactions is", numberOfServAgrProposeTx,
		"\n number of published Commit ServAgr transactions is", numberOfServAgrCommitTx,
		"\n number of published regular payment transactions is", numberOfRegPayTx,
	)

	TotalNumTxsInBothQueue := numberOfPoRTx + numberOfStoragePayTx + numberOfServAgrProposeTx + numberOfServAgrCommitTx + numberOfRegPayTx + numberOfSyncTx
	TotalNumTxsInFirstQueue := numberOfPoRTx + numberOfStoragePayTx + numberOfServAgrProposeTx + numberOfServAgrCommitTx

	var avg1stWaitQueue, avg2ndWaitQueue float64 = 0, 0
	if TotalNumTxsInFirstQueue != 0 {
		avg1stWaitQueue = float64(bz.FirstQueueWait) / float64(TotalNumTxsInFirstQueue)
	}

	if numberOfRegPayTx != 0 {
		avg2ndWaitQueue = float64(bz.SecondQueueWait) / float64(numberOfRegPayTx)
	}

	ConfirmationTime := float64(bz.FirstQueueWait+bz.SecondQueueWait) / float64(TotalNumTxsInBothQueue)

	BlockSize := accumulatedTxSize + allocatedBlockSizeForRegPayTx + BlockSizeMinusTransactions
	err = blockchain.AddStatsToRoundTableBasedOnRoundNumber(BlockSize, numberOfRegPayTx, numberOfPoRTx, numberOfStoragePayTx, numberOfServAgrProposeTx, numberOfServAgrCommitTx,
		TotalNumTxsInBothQueue, avg1stWaitQueue,
		avg2ndWaitQueue, numberOfSyncTx, SCPoRTx, ConfirmationTime, int(bz.MCRoundNumber.Load()))
	log.Lvl2("Final result MC: \n Block size allocation:\n", allocatedBlockSizeForRegPayTx,
		" for regular payment txs,\n and ", accumulatedTxSize, " for other types of txs")
	if err != nil {
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}

	log.Lvl1("In total in mc round number ", bz.MCRoundNumber,
		"\n number of all types of submitted txs is", TotalNumTxsInBothQueue)

	// ---- Filling Overall Evaluation Sheet ----

	SumBCSize, err := blockchain.GetSumMainChain("RoundTable", "BCSize")
	if err != nil {
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}

	SumRegPayTx, err := blockchain.GetSumMainChain("RoundTable", "RegPayTx")
	if err != nil {
		log.LLvl1(err)
	}

	SumPoRTx, err := blockchain.GetSumMainChain("RoundTable", "PoRTx")
	if err != nil {
		log.LLvl1(err)
	}

	SumStorjTx, err := blockchain.GetSumMainChain("RoundTable", "StorjPayTx")
	if err != nil {
		log.LLvl1(err)
	}

	SumCntPropTx, err := blockchain.GetSumMainChain("RoundTable", "CntPropTx")
	if err != nil {
		log.LLvl1(err)
	}

	SumCntCmtTx, err := blockchain.GetSumMainChain("RoundTable", "CntCmtTx")
	if err != nil {
		log.LLvl1(err)
	}

	AvgWaitForOtherTxs, err := blockchain.GetAvgMainChain("RoundTable", "AveWaitOtherTxs")
	if err != nil {
		log.LLvl1(err)
	}

	AvgWaitRegPay, err := blockchain.GetAvgMainChain("RoundTable", "AveWaitRegPay")
	if err != nil {
		log.LLvl1(err)
	}

	err = blockchain.InsertIntoMainChainOverallEvaluationTable(int(bz.MCRoundNumber.Load()), SumBCSize, SumRegPayTx, SumPoRTx, SumStorjTx,
		SumCntPropTx, SumCntCmtTx, AvgWaitForOtherTxs, AvgWaitRegPay, 0)
	if err != nil {
		panic(err)
	}
	/*
		FormulaString = "=SUM(RoundTable!O2:O" + NextRow + ")"
		err = f.SetCellFormula("OverallEvaluation", axisOverallBlockSpaceFull, FormulaString)
		if err != nil {
			log.LLvl1(err)
		}*/

	log.Lvl1(bz.Name(), " Finished taking transactions from queue (FIFO) into new block in mc round number ", bz.MCRoundNumber)

}

func (bz *ChainBoost) syncMainChainBCTransactionQueueCollect(name string) (blocksize int) {
	// ----------------------------------------------------------------------
	// ------ adding sync transaction into transaction queue sheet -----
	// ----------------------------------------------------------------------
	/* each transaction has the following column stored on the transaction queue sheet:
	0) name
	1) size
	2) time
	3) issuedMCRoundNumber
	4) ServAgrId */
	takenTime := time.Now()
	var SummTxNum int
	totalPoR := 0
	for i, v := range bz.SCTools[name].SummPoRTxs {
		totalPoR = totalPoR + v
		if v != 0 {
			SummTxNum = SummTxNum + 1 // counting the number of active contracts to measure the length of summary tx
		}
		bz.SCTools[name].SummPoRTxs[i] = 0
	}
	//measuring summary block size
	SummaryBlockSizeMinusTransactions, _ := blockchain.SCBlockMeasurement()
	// --------------- adding bls signature size  -----------------
	log.Lvl4("Size of bls signature:", len(bz.SCSig[name]))
	SummaryBlockSizeMinusTransactions = SummaryBlockSizeMinusTransactions + len(bz.SCSig[name])
	// ------------------------------------------------------------
	SummTxsSizeInSummBlock := 0
	if name == "por" {
		SummTxsSizeInSummBlock = blockchain.SCSummaryTxMeasurement(SummTxNum)
	} else if name == "marketmatch" {
		SummTxsSizeInSummBlock = blockchain.SCSummaryActivationTxMeasurement(len(gt.MMContractsToActivate))
	} else {
		SummTxsSizeInSummBlock = blockchain.SCSummaryDisputeTxMeasurement(SummTxNum)
	}
	blocksize = SummaryBlockSizeMinusTransactions + SummTxsSizeInSummBlock
	// ---
	// for now sync transaction and summary transaction are the same, we should change it when  they differ
	SyncTxSize := SummTxsSizeInSummBlock

	err := blockchain.InsertIntoMainChainSyncTxQueue("TxSync", uint32(SyncTxSize), time.Now(), int(bz.MCRoundNumber.Load()), totalPoR)

	if err != nil {
		panic(err)
	}
	log.Lvl1("Final result MC:", bz.Name(), " finished collecting new sync transactions to queue in mc round number ", bz.MCRoundNumber)
	log.Lvl1("syncMainChainBCTransactionQueueCollect took:", time.Since(takenTime).String(), "for mc round number", bz.MCRoundNumber)

	return blocksize
}

func (bz *ChainBoost) StoragePaymentMainChainBCTransactionQueueCollect() error {
	// a simplifying assumpption: since all. contracts will be activated once expired, we assume that there is no contract that remain expired for an
	// entire duration of n epoch
	var err error
	_, _, _, StoragePayTxSize, _, _ := blockchain.TransactionMeasurement(bz.SectorNumber, bz.SimulationSeed)
	mcFirstQueueTxs := make([]blockchain.MainChainFirstQueueEntry, 0)

	for i := range bz.Roster().List {
		for j := 1; j <= bz.NumberOfActiveContractsPerServer; j++ {
			tx := blockchain.MainChainFirstQueueEntry{Name: "TxStoragePayment", Size: StoragePayTxSize, Time: time.Now(), RoundIssued: int(bz.MCRoundNumber.Load()), ServAgrId: i + 1}
			mcFirstQueueTxs = append(mcFirstQueueTxs, tx)
		}
	}
	for i := 0; i < len(mcFirstQueueTxs); i += 2000 {
		limit := math.Min(float64(len(mcFirstQueueTxs)), float64(i+2000))
		err = blockchain.BulkInsertIntoMainChainFirstQueue(mcFirstQueueTxs[i:int(limit)])

		if err != nil {
			return err
		}
	}
	return nil
}
