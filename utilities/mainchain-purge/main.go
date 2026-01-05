package main

import (
	"log"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/chainBoostScale/ChainBoost/MainAndSideChain/blockchain"
	"github.com/chainBoostScale/ChainBoost/simulation/platform"
)

func getPlatformConfigs(configFile string) ([]platform.Config, error) {
	platformDst := "Researchlabab"
	deployP := platform.NewPlatform(platformDst)
	if deployP == nil {
		log.Fatal("Platform not recognized.", platformDst)
	}
	runconfigs := platform.ReadRunFile(deployP, configFile)
	platformConfigs := make([]platform.Config, 0)
	for _, rc := range runconfigs {
		cfg := platform.Config{}
		_, err := toml.Decode(string(rc.Toml()), &cfg)
		if err != nil {
			return nil, err
		}
		platformConfigs = append(platformConfigs, cfg)
	}
	return platformConfigs, nil
}

func updateMainChainBCTransactionQueueTake(cfg platform.Config, MCRoundNbr int) bool {
	var err error
	// --- reset
	FirstQueueWait := 0
	SecondQueueWait := 0

	var accumulatedTxSize int
	blockIsFull := false
	regPayshareIsFull := false
	numberOfRegPayTx := 0
	_ = blockchain.InsertIntoMainChainRoundTable(MCRoundNbr, "Aftermath-seed", 0, "aftermath-leader", 0, 0, 0, 0, 0, time.Now(), 0, 0, 0, false, false, false)
	BlockSizeMinusTransactions := blockchain.BlockMeasurement()

	regPayTxShare := (cfg.PercentageTxPay) * (cfg.MainChainBlockSize - BlockSizeMinusTransactions)
	/* -----------------------------------------------------------------------------
		-- take regular payment transactions from sheet: SecondQueue
	----------------------------------------------------------------------------- */

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

			SecondQueueWait = SecondQueueWait + int(MCRoundNbr) - int(out.RoundIssued)
			lastRowId = out.RowId
			numberOfRegPayTx++

		} else {
			regPayshareIsFull = true
			blockchain.AddToRoundTableBasedOnRoundNumber("RegSpaceFull", true, MCRoundNbr)
			break
		}
		//}
		//}
	}
	blockchain.MainChainDeleteFromSecondQueue(lastRowId)

	allocatedBlockSizeForRegPayTx := accumulatedTxSize
	otherTxsShare := cfg.MainChainBlockSize - BlockSizeMinusTransactions - allocatedBlockSizeForRegPayTx
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

				err = blockchain.MainChainSetPublishedAndStartRoundOnServAgrId(out.ServAgrId, true, int(MCRoundNbr))
				if err != nil {
					panic(err)
				}
				numberOfServAgrCommitTx++

			case "TxStoragePayment":
				numberOfStoragePayTx++
				// && bz.SimState == 1 is for backup check - the first condition shouldn't be true if the second one isn't
			case "TxPor":
				numberOfPoRTx++

			case "TxServAgrPropose":
				numberOfServAgrProposeTx++

			case "TxSync":
				numberOfSyncTx++
				SCPoRTx = SCPoRTx + out.ServAgrId
			default:
				panic("the type of transaction in the queue is un-defined")
			}

			FirstQueueWait = FirstQueueWait + int(MCRoundNbr) - out.RoundIssued
			lastRowId = out.RowId
		} else {
			blockIsFull = true
			err = blockchain.AddToRoundTableBasedOnRoundNumber("BlockSpaceFull", true, int(MCRoundNbr))
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
	if len(rows) == 0 && len(fqRows) == 0 {
		return false
	}

	log.Print("In total in mc round number ", MCRoundNbr,
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
		avg1stWaitQueue = float64(FirstQueueWait) / float64(TotalNumTxsInFirstQueue)
	}

	if numberOfRegPayTx != 0 {
		avg2ndWaitQueue = float64(SecondQueueWait) / float64(numberOfRegPayTx)
	}

	ConfirmationTime := float64(FirstQueueWait+SecondQueueWait) / float64(TotalNumTxsInBothQueue)

	BlockSize := accumulatedTxSize + allocatedBlockSizeForRegPayTx + BlockSizeMinusTransactions

	err = blockchain.AddStatsToRoundTableBasedOnRoundNumber(BlockSize, numberOfRegPayTx, numberOfPoRTx, numberOfStoragePayTx, numberOfServAgrProposeTx, numberOfServAgrCommitTx,
		TotalNumTxsInBothQueue, avg1stWaitQueue,
		avg2ndWaitQueue, numberOfSyncTx, SCPoRTx, ConfirmationTime, int(MCRoundNbr))
	if err != nil {
		panic(err)
	}
	return true
}

func main() {
	cfg, _ := getPlatformConfigs("ChainBoost.toml")
	blockchain.MainChainEndPosition(cfg[0].SimulationRounds)
	run := true
	rnd_nbr := cfg[0].SimulationRounds + 1
	for run {
		run = updateMainChainBCTransactionQueueTake(cfg[0], rnd_nbr)
		rnd_nbr++
	}
}
