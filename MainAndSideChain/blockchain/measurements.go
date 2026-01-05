package blockchain

import (
	"crypto/sha256"
	"math/rand"
	"time"
	"unsafe"

	"github.com/DmitriyVTitov/size"
	"github.com/chainBoostScale/ChainBoost/por"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/kyber/v3/xof/blake2xb"

	//"github.com/chainBoostScale/ChainBoost/vrf"
	// ToDoCoder: later that I brought everything from blscosi package to ChainBoost package, I shoudl add another pacckage with
	// some definitions in it to be imported/used in blockchain(here) and simulation package (instead of using blscosi/protocol)

	"github.com/chainBoostScale/ChainBoost/onet/log"
)

/* -------------------------------------------------------------------- */
//  ----------------  Block and Transactions size measurements -----
/* -------------------------------------------------------------------- */
// BlockMeasurement compute the size of meta data and every thing other than the transactions inside the block
func BlockMeasurement() (BlockSizeMinusTransactions int) {
	// -- Hash Sample ----
	sha := sha256.New()
	if _, err := sha.Write([]byte("a sample seed")); err != nil {
		log.Error("Couldn't hash header:", err)
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}
	hash := sha.Sum(nil)
	var hashSample [32]uint8
	copy(hashSample[:], hash[:])
	// --
	var Version [4]byte
	var cnt [2]byte
	var feeSample, BlockSizeSample [3]byte
	var MCRoundNumberSample [3]byte
	// ---------------- block sample ----------------
	var TxPayArraySample []*MC_TxPay
	var TxPorArraySample []*SC_TxPoR
	var TxServAgrProposeArraySample []*SC_TxServAgrPropose
	var TxServAgrCommitSample []*SC_TxServAgrCommit
	var TxStoragePaySample []*MC_TxStoragePay

	x9 := &TransactionList{
		//---
		TxPays:   TxPayArraySample,
		TxPayCnt: cnt,
		//---
		TxPoRs:   TxPorArraySample,
		TxPoRCnt: cnt,
		//---
		TxServAgrProposes:   TxServAgrProposeArraySample,
		TxServAgrProposeCnt: cnt,
		//---
		TxServAgrCommits:   TxServAgrCommitSample,
		TxServAgrCommitCnt: cnt,
		//---
		TxStoragePay:    TxStoragePaySample,
		TxStoragePayCnt: cnt,

		Fees: feeSample,
	}
	// real! TransactionListSize = size.Of(x9) + sum of size of included transactions
	// --- VRF
	//: ToDoCoder: temp comment
	// t := []byte("first round's seed")
	// VrfPubkey, VrfPrivkey := vrf.VrfKeygen()
	// proof, _ := VrfPrivkey.ProveBytes(t)
	// _, vrfOutput := VrfPubkey.VerifyBytes(proof, t)
	// var nextroundseed [64]byte =  // vrfOutput
	// var VrfProof [80]byte = proof
	// --- time
	ti := []byte(time.Now().String())
	var timeSample [4]byte
	copy(timeSample[:], ti[:])
	// ---

	x10 := &BlockHeader{
		MCRoundNumber: MCRoundNumberSample,
		//RoundSeed:         nextroundseed,
		//LeadershipProof:   VrfProof,
		PreviousBlockHash: hashSample,
		Timestamp:         timeSample,
		MerkleRootHash:    hashSample,
		Version:           Version,
	}
	x11 := &Block{
		BlockSize:       BlockSizeSample,
		BlockHeader:     x10,
		TransactionList: x9,
	}

	log.Lvl5(x11)

	BlockSizeMinusTransactions = len(BlockSizeSample) + //x11
		len(MCRoundNumberSample) + /*ToDoCoder: temp comment: len(nextroundseed) + len(VrfProof) + */ len(hashSample) + len(timeSample) + len(hashSample) + len(Version) + //x10
		5*len(cnt) + len(feeSample) //x9
	// ---
	log.Lvl4("Block Size Minus Transactions is: ", BlockSizeMinusTransactions)

	return BlockSizeMinusTransactions
}

// TransactionMeasurement computes the size of 5 types of transactions we currently have in the system:
// Por, ServAgrPropose, Pay, StoragePay, ServAgrCommit
func TransactionMeasurement(SectorNumber, SimulationSeed int) (PorTxSize uint32, ServAgrProposeTxSize uint32, PayTxSize uint32, StoragePayTxSize uint32, ServAgrCommitTxSize uint32, finalCommitSize uint32) {
	// -- Hash Sample ----
	sha := sha256.New()
	if _, err := sha.Write([]byte("a sample seed")); err != nil {
		log.Error("Couldn't hash header:", err)
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}
	hash := sha.Sum(nil)
	var hashSample [32]byte
	copy(hashSample[:], hash[:])
	// ---------------- payment transaction sample ----------------
	r := rand.New(rand.NewSource(int64(SimulationSeed)))
	var Version, SequenceNumber, index, LockingScriptSize, fileSizeSample, UnlockingScriptSize [4]byte
	var Amount [8]byte
	var startRoundSample, MCRoundNumberSample, pricePerRoundSample [3]byte
	var duration [2]byte
	var cmtSample [71]byte // see https://medium.com/coinmonks/on-bitcoin-transaction-sizes-97e31bc9d816
	UnlockinScriptSample := []byte("48304502203a776322ebf8eb8b58cc6ced4f2574f4c73aa664edce0b0022690f2f6f47c521022100b82353305988cb0ebd443089a173ceec93fe4dbfe98d74419ecc84a6a698e31d012103c5c1bc61f60ce3d6223a63cedbece03b12ef9f0068f2f3c4a7e7f06c523c3664")
	LockingScriptSample := []byte("76a914977ae6e32349b99b72196cb62b5ef37329ed81b488ac063d1000000000001976a914f76bc4190f3d8e2315e5c11c59cfc8be9df747e388ac")
	var cnt [1]byte
	// --- time
	t := []byte(time.Now().String())
	var timeSample [4]byte
	copy(timeSample[:], t[:])
	// ---
	x1 := &outpoint{
		hash:  hashSample,
		index: index,
	}
	x2 := &TxPayIn{
		outpoint:            x1,
		UnlockingScriptSize: UnlockingScriptSize,
		UnlockinScript:      UnlockinScriptSample,
		SequenceNumber:      SequenceNumber,
	}
	x3 := &TxPayOut{
		Amount:            Amount,
		LockingScript:     LockingScriptSample,
		LockingScriptSize: LockingScriptSize,
	}
	xin := []*TxPayIn{x2}
	xout := []*TxPayOut{x3}
	x4 := &MC_TxPay{
		LockTime:    timeSample,
		Version:     Version,
		TxInCnt:     cnt,
		TxOutCnt:    cnt,
		TxIns:       xin,
		TxOuts:      xout,
		OnMainChain: true,
	}

	PayTxSize = uint32(len(hashSample) + len(index) + // outpoint
		len(UnlockingScriptSize) + len(UnlockinScriptSample) + len(SequenceNumber) + //TxPayIn
		len(Amount) + len(LockingScriptSample) + len(LockingScriptSize) + //TxPayOut
		len(timeSample) + len(Version) + len(cnt) + len(cnt)) //TxPay
	log.Lvl4("size of a pay transaction is: ", PayTxSize, "bytes")
	// ---------------- por transaction sample  ----------------

	sk, _ := por.RandomizedKeyGeneration()
	Tau, pf := por.RandomizedFileStoring(sk, por.GenerateFile(SectorNumber), SectorNumber)

	// ---------------- ServAgrPropose transaction sample ----------------
	x5 := &SC_ServAgr{
		duration:      duration,
		fileSize:      fileSizeSample,
		startRound:    startRoundSample,
		pricePerRound: pricePerRoundSample,
		Tau:           Tau,
		//MCRoundNumber:   MCRoundNumberSample,
	}

	x7 := &SC_TxServAgrPropose{
		tx:               x4,
		ServAgrID:        x5,
		clientCommitment: cmtSample,
		OnMainChain:      false,
	}

	log.Lvl5("x5 is:", x5, " and x7 is: ", x7)

	ServAgrProposeTxSize = PayTxSize + //tx
		uint32(len(duration)+len(fileSizeSample)+len(startRoundSample)+len(pricePerRoundSample)+len(Tau)+ //ServAgr tx
			len(cmtSample)) //clientCommitment

	FinalCommitSize := ServAgrProposeTxSize + uint32(len(cmtSample))
	log.Lvl4("size of a ServAgr Propose transaction (including ServAgr creation tx) is: ", ServAgrProposeTxSize,
		"bytes \n with ",
		len(duration)+len(fileSizeSample)+len(startRoundSample)+len(pricePerRoundSample)+len(Tau), " bytes for ServAgr, \n and ",
		PayTxSize, " bytes for payment")

	// ---------------- por transaction sample ----------------
	var randombyte = make([]byte, 8)
	rand := random.New()
	var muArraySample []kyber.Scalar
	var ServAgrIdSample = r.Uint64()

	for i := range muArraySample {
		muArraySample[i] = por.Suite.Scalar().Pick(blake2xb.New(randombyte))
	}
	sigmaSample := por.Suite.G1().Point().Mul(por.Suite.G1().Scalar().Pick(rand), nil)

	x8 := &por.Por{
		Mu:    muArraySample,
		Sigma: sigmaSample,
	}

	porSize := SectorNumber*por.Suite.G1().ScalarLen() + por.Suite.G2().PointLen() + size.Of(x8)

	//sk, _ := por.RandomizedKeyGeneration()
	//_, pf := por.RandomizedFileStoring(sk, por.GenerateFile(SectorNumber), SectorNumber)
	p := por.CreatePoR(pf, SectorNumber, SimulationSeed)

	x6 := &SC_TxPoR{
		ServAgrID:     ServAgrIdSample,
		por:           &p,
		MCRoundNumber: MCRoundNumberSample,
		OnMainChain:   false,
	}

	log.Lvl5("tx por is: ", x6)

	PorTxSize = uint32(porSize /*size of pur por*/ +
		8 /*len(ServAgrIdSample)*/ + len(MCRoundNumberSample)) //TxPoR

	log.Lvl4("size of a por transaction is: ", PorTxSize, " bytes \n with ",
		SectorNumber*por.Suite.G1().ScalarLen()+por.Suite.G2().PointLen(), " bytes for pure por")
	// ---------------- TxStoragePay transaction sample ----------------
	x9 := &MC_TxStoragePay{
		ServAgrID:   ServAgrIdSample,
		tx:          x4,
		OnMainChain: true,
	}

	log.Lvl5("tx StoragePay is: ", x9)

	StoragePayTxSize = 8 /*len(ServAgrIdSample)*/ + PayTxSize
	log.Lvl4("size of a StoragePay transaction is: ", StoragePayTxSize)
	// ---------------- TxServAgrCommit transaction sample ----------------
	x10 := SC_TxServAgrCommit{
		serverCommitment: cmtSample,
		ServAgrID:        ServAgrIdSample,
		OnMainChain:      false,
	}

	log.Lvl5("tx ServAgrCommit is: ", x10)

	ServAgrCommitTxSize = uint32(len(cmtSample) + 8) /*len(ServAgrIdSample)*/
	log.Lvl4("size of a ServAgrCommit transaction is: ", ServAgrCommitTxSize)

	return PorTxSize, ServAgrProposeTxSize, PayTxSize, StoragePayTxSize, ServAgrCommitTxSize, FinalCommitSize
}

/* -------------------------------------------------------------------------------------------
    ------------- measuring side chain's sync transaction, summary and meta blocks ------
-------------------------------------------------------------------------------------------- */

// MetaBlockMeasurement compute the size of meta data and every thing other than the transactions inside the meta block
func SCBlockMeasurement() (SummaryBlockSizeMinusTransactions int, MetaBlockSizeMinusTransactions int) {
	// ----- block header sample -----
	// -- Hash Sample ----
	sha := sha256.New()
	if _, err := sha.Write([]byte("a sample seed")); err != nil {
		log.Error("Couldn't hash header:", err)
		log.LLvl1("Panic Raised:\n\n")
		panic(err)
	}
	hash := sha.Sum(nil)
	var hashSample [32]uint8
	copy(hashSample[:], hash[:])
	// --
	var Version [4]byte
	var cnt [2]byte
	var feeSample, BlockSizeSample [3]byte
	var SCRoundNumberSample [3]byte
	var samplePublicKey [33]byte
	//var samplePublicKey kyber.Point
	// --- VRF
	// t := []byte("first round's seed")
	// VrfPubkey, VrfPrivkey := vrf.VrfKeygen()
	// proof, _ := VrfPrivkey.ProveBytes(t)
	// _, vrfOutput := VrfPubkey.VerifyBytes(proof, t)
	// var nextroundseed [64]byte = vrfOutput
	// var VrfProof [80]byte = proof
	// --- time
	ti := []byte(time.Now().String())
	var timeSample [4]byte
	copy(timeSample[:], ti[:])
	// ---
	x10 := &SCBlockHeader{
		SCRoundNumber: SCRoundNumberSample,
		//RoundSeed:         nextroundseed,
		//LeadershipProof:   VrfProof,
		PreviousBlockHash: hashSample,
		Timestamp:         timeSample,
		MerkleRootHash:    hashSample,
		Version:           Version,
		LeaderPublicKey:   samplePublicKey,
		//BlsSignature:      sampleBlsSig, // this will be added back in the protocol
	}
	// ---------------- meta block sample ----------------
	var TxPorArraySample []*SC_TxPoR

	x9 := &SCMetaBlockTransactionList{
		TxPoRs:   TxPorArraySample,
		TxPoRCnt: cnt,
		Fees:     feeSample,
	}
	// real! TransactionListSize = size.Of(x9) + sum of size of included transactions

	x11 := &SCMetaBlock{
		BlockSize:                  BlockSizeSample,
		SCBlockHeader:              x10,
		SCMetaBlockTransactionList: x9,
	}

	log.Lvl5(x11)

	MetaBlockSizeMinusTransactions = len(BlockSizeSample) + //x11: SCMetaBlock
		len(SCRoundNumberSample) + /* len(nextroundseed) + len(VrfProof) +*/ len(hashSample) + len(timeSample) +
		len(hashSample) + len(Version) + len(samplePublicKey) + //x10: SCBlockHeader
		len(cnt) + len(feeSample) //x9: SCMetaBlockTransactionList
	// ---
	log.Lvl4("Meta Block Size Minus Transactions is: ", MetaBlockSizeMinusTransactions)

	//------------------------------------- Summary block -----------------------------
	// ---------------- summary block sample ----------------
	var TxSummaryArraySample []*TxSummary

	x12 := &SCSummaryBlockTransactionList{
		//---
		TxSummary:    TxSummaryArraySample,
		TxSummaryCnt: cnt,
		Fees:         feeSample,
	}
	x13 := &SCSummaryBlock{
		BlockSize:                     BlockSizeSample,
		SCBlockHeader:                 x10,
		SCSummaryBlockTransactionList: x12,
	}
	log.Lvl5(x13)
	SummaryBlockSizeMinusTransactions = len(BlockSizeSample) + //x13: SCSummaryBlock
		len(SCRoundNumberSample) + /*len(nextroundseed) + len(VrfProof) +*/ len(hashSample) + len(timeSample) + len(hashSample) +
		len(Version) + len(samplePublicKey) + //x10: SCBlockHeader
		len(cnt) + len(feeSample) //x12: SCSummaryBlockTransactionList
	log.Lvl4("Summary Block Size Minus Transactions is: ", SummaryBlockSizeMinusTransactions)

	return SummaryBlockSizeMinusTransactions, MetaBlockSizeMinusTransactions
}

// SyncTransactionMeasurement computes the size of sync transaction
func SyncTransactionMeasurement() (SyncTxSize int) {
	return
}
func SCSummaryTxMeasurement(SummTxNum int) (SummTxsSizeInSummBlock int) {
	r := rand.New(rand.NewSource(int64(0)))
	var a []uint64
	for i := 0; i < SummTxNum; i++ {
		a = append(a, r.Uint64())
	}
	return 2 * len(a)
}

func SCSummaryActivationTxMeasurement(ContractsToActivate int) int {
	/// A Contract Summary should look like as follows:
	/// [] ID, ProviderId, CustomerId,  NumberOfRounds, CostPerRound

	return ContractsToActivate * 5 * int(unsafe.Sizeof(uint64(0)))
}

func SCSummaryDisputeTxMeasurement(disputes int) int {
	// [] ID, FaultyId, Outcome, Duration
	return disputes * 4 * int(unsafe.Sizeof(uint64(0)))
}
