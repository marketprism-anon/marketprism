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

func updateSideChainBCTransactionQueueTake(cfg platform.Config, MCRoundNbr int, SCRoundNbr int, SCSigLen int) bool {
	var err error
	//var epochNumber = int(math.Floor(float64(bz.MCRoundNumber) / float64(bz.MCRoundPerEpoch)))
	// --- reset
	SideChainQueueWait := 0

	var accumulatedTxSize int
	blockIsFull := false
	_, MetaBlockSizeMinusTransactions := blockchain.SCBlockMeasurement()
	// --------------- adding bls signature size  -----------------
	MetaBlockSizeMinusTransactions = MetaBlockSizeMinusTransactions + SCSigLen
	// ------------------------------------------------------------
	//var TakeTime time.Time

	/* -----------------------------------------------------------------------------
		 -- take por transactions from sheet: FirstQueue
	----------------------------------------------------------------------------- */
	// --------------------------------------------------------------------
	// looking for last round's number in the round table sheet in the sidechainbc file
	// --------------------------------------------------------------------
	// finding the last row in side chain bc file, in round table sheet
	blockIsFull = false
	accumulatedTxSize = 0

	numberOfPoRTx := 0

	rows, err := blockchain.SideChainGetFirstQueue()
	if err != nil {
		panic(err)
	}
	index := 0
	lastRowId := 0
	// len(rows) gives number of rows - the length of the "external" array
	if len(rows) == 0 {
		return false
	}
	for index = 0; index <= len(rows)-1 && !blockIsFull; index++ {
		row := rows[index]
		/* each transaction has the following column stored on the transaction queue sheet:
		0) name
		1) size
		2) time
		3) issuedMCRoundNumber
		4) ServAgrId */
		if accumulatedTxSize+row.Size <= cfg.SideChainBlockSize-MetaBlockSizeMinusTransactions {
			accumulatedTxSize = accumulatedTxSize + row.Size
			if row.Name == "TxPor" {
				numberOfPoRTx++
			} else {
				panic("the type of transaction in the queue is un-defined")
			}
			currentEpoch := int(MCRoundNbr-2) / cfg.MCRoundPerEpoch
			SideChainQueueWait = SideChainQueueWait + int(SCRoundNbr) - int(row.IssuedScRoundNumber) + (currentEpoch-row.Epoch)*cfg.SCRoundPerEpoch
			lastRowId = row.RowId

		} else {
			blockIsFull = true
			err = blockchain.SideChainRoundTableSetBlockSpaceIsFull(int(SCRoundNbr))
			if err != nil {
				panic(err)
			}
			break
		}
	}
	err = blockchain.SideChainDeleteFromFirstQueue(lastRowId)
	if err != nil {
		panic(err)
	}
	var avgWait float64 = 0
	if numberOfPoRTx != 0 {
		avgWait = float64(SideChainQueueWait) / float64(numberOfPoRTx)
	}
	err = blockchain.SideChainRoundTableSetFinalRoundInfo(accumulatedTxSize+MetaBlockSizeMinusTransactions,
		numberOfPoRTx,
		avgWait,
		int(SCRoundNbr),
		len(rows)-numberOfPoRTx)
	if err != nil {
		panic(err)
	}

	log.Printf("Round: %d Number of PoR Transactions Added into the SideChain = %d", SCRoundNbr, numberOfPoRTx)
	return true
}

func main() {
	cfg, _ := getPlatformConfigs("ChainBoost.toml")
	next_sc_round := (blockchain.SideChainEndPosition() + 1) % cfg[0].SCRoundPerEpoch
	_, metablockSize := blockchain.SCBlockMeasurement()
	mbSCSigLen := blockchain.SideChainGetLastRowSigLen() - metablockSize
	run := true
	rnd_nbr := cfg[0].SimulationRounds + 1
	for run {

		if next_sc_round%cfg[0].SCRoundPerEpoch == 0 {
			blockchain.InsertIntoSideChainRoundTable(cfg[0].SCRoundPerEpoch, 0, "aftermath", 0, time.Now(), 0, 0, 0, 0, rnd_nbr)
			run = true
		} else {
			blockchain.InsertIntoSideChainRoundTable(next_sc_round, 0, "aftermath", 0, time.Now(), 0, 0, 0, 0, rnd_nbr)
			run = updateSideChainBCTransactionQueueTake(cfg[0], rnd_nbr, next_sc_round, mbSCSigLen)
		}
		if (next_sc_round)%(cfg[0].SCRoundPerEpoch/cfg[0].MCRoundPerEpoch) == 0 {
			rnd_nbr++
		}
		next_sc_round = (next_sc_round + 1) % cfg[0].SCRoundPerEpoch
	}
}
