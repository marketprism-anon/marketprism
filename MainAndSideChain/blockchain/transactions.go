// note that functions: ScalarLen(), PointLen() and size.Of() return the size in bytes
// ScalarLength, for example is a suit function, which in turn calls a group function with the same name,
// and thenreturns "mod.NewInt64(0, Order).MarshalSize()"
// length of uint64: 8 byte

/* Note that all nodes (same in mc. and sc.) can verify each epoch’s leader and committee from the mc.

mc leader: verify the authenticity of: the leader and the involved committee of a proposed sync tx.
leader and committee: verify the correctness of: POR tx.*/

package blockchain

import (
    "github.com/chainBoostScale/ChainBoost/por"
    "github.com/chainBoostScale/ChainBoost/MainAndSideChain/BLSCoSi"
)

/*  payment transactions are close to byzcoin's and from the structure
provided in: https://developer.bitcoin.org/reference/transactions.html */
type outpoint struct {
	hash  [32]byte //TXID
	index [4]byte
}
type TxPayIn struct { // Each input spends an outpoint from a previous transaction
	outpoint            *outpoint // 36 bytes
	UnlockingScriptSize [4]byte   // see https://developer.bitcoin.org/reference/transactions.html#compactsize-unsigned-integers
	UnlockinScript      []byte    // signature script // see https://medium.com/coinmonks/on-bitcoin-transaction-sizes-97e31bc9d816 73?
	SequenceNumber      [4]byte
}
type TxPayOut struct {
	Amount            [8]byte
	LockingScript     []byte // pubkey script // see https://medium.com/coinmonks/on-bitcoin-transaction-sizes-97e31bc9d816 35?
	LockingScriptSize [4]byte
}
type MC_TxPay struct {
	LockTime    [4]byte
	Version     [4]byte
	TxInCnt     [1]byte // compactSize uint see https://developer.bitcoin.org/reference/transactions.html#compactsize-unsigned-integers
	TxOutCnt    [1]byte
	TxIns       []*TxPayIn
	TxOuts      []*TxPayOut
	OnMainChain bool
}

/* ---------------- market matching transactions ---------------- */

//server agreement
type SC_ServAgr struct {
	duration      [2]byte
	fileSize      [4]byte
	startRound    [3]byte
	pricePerRound [3]byte
	Tau           []byte
	OnMainChain   bool
	//MCRoundNumber   [3]byte //ToDoCoder: why MCRoundNumber is commented here?
	/* later: Instead of the "file tag" (Tau []byte),
	the round number is sent and stored on chain and the verifiers
	can reproduce the file tag (as well as random query)
	from that round's seed as a source of randomness */
}

/* TxServAgrPropose: A client create a ServAgr and add approprite (duration*price) escrow and sign it and issue a ServAgr propose transaction
which has the ServAgr and payment (escrow) info in it */
type SC_TxServAgrPropose struct {
	tx               *MC_TxPay
	ServAgrID        *SC_ServAgr
	clientCommitment [71]byte
	OnMainChain      bool
}

type SC_TxServAgrCommit struct {
	serverCommitment [71]byte
	ServAgrID        uint64
	OnMainChain      bool
}

/* ---------------- transactions that will be issued (with ChainBoost: each side chain's round / Pure MainChain: each main chain's round) until a ServAgr is active ---------------- */

/* por txs are designed in away that the verifier (any miner) has sufficient information to verify it */
type SC_TxPoR struct {
	ServAgrID     uint64
	por           *por.Por
	MCRoundNumber [3]byte // to determine the random query used for it
	OnMainChain   bool
}

/* ---------------- transactions that will be issued after a ServAgr is expired(?) ---------------- */

type MC_TxStoragePay struct {
	ServAgrID   uint64
	tx          *MC_TxPay
	OnMainChain bool
}

/* ---------------- transactions that will be issued for each round ---------------- */
/* the proof is generated as an output of fucntion call "ProveBytes" in vrf package*/
/* ---------------- block structure and its metadata ----------------
(from algorand) : "Blocks consist of a list of transactions,  along with metadata needed by MainChain miners.
Specifically, the metadata consists of
	- the round number,
	- the proposer’s VRF-based seed,
	- a hash of the previous block in the ledger,and
	- a timestamp indicating when the block was proposed
The list of transactions in a block logically translates to a set of weights for each user’s public key
(based on the balance of currency for that key), along with the total weight of all outstanding currency."
*/
type TransactionList struct {
	//---
	TxPays   []*MC_TxPay
	TxPayCnt [2]byte
	//---
	TxPoRs   []*SC_TxPoR
	TxPoRCnt [2]byte
	//---
	TxServAgrProposes   []*SC_TxServAgrPropose
	TxServAgrProposeCnt [2]byte
	//---
	TxServAgrCommits   []*SC_TxServAgrCommit
	TxServAgrCommitCnt [2]byte
	//---
	TxStoragePay    []*MC_TxStoragePay
	TxStoragePayCnt [2]byte
	//--- tx from side chain
	TxSCSync    []*TxSCSync
	TxSCSyncCnt [2]byte
	//---
	Fees [3]byte
}

type BlockHeader struct {
	MCRoundNumber [3]byte
	// next round's seed for VRF based leader election which is the output of this round's leader's proof verification: VerifyBytes
	// _, (next round's seed)RoundSeed := (current round's leader)VrfPubkey.VerifyBytes((current round's leader)proof, (current round's seed)t)
	RoundSeed         [64]byte
	LeadershipProof   [80]byte
	PreviousBlockHash [32]byte
	Timestamp         [4]byte
	//--
	MerkleRootHash  [32]byte
	Version         [4]byte
	LeaderPublicKey [33]byte // see https://medium.com/coinmonks/on-bitcoin-transaction-sizes-97e31bc9d816
}

type Block struct {
	BlockSize       [3]byte
	BlockHeader     *BlockHeader
	TransactionList *TransactionList
}

/* -------------------------------------------------------------------- */
//  ----------------  Side Chain -------------------------------------
/* -------------------------------------------------------------------- */

/* -------------------------------------------------------------------------------------------
---------------- transaction and block types used just in sidechain ----------------
-------------------------------------------------------------------------------------------- */

type SCMetaBlockTransactionList struct {
	//---
	TxPoRs   []*SC_TxPoR
	TxPoRCnt [2]byte
	//---
	Fees [3]byte
}
type SCBlockHeader struct {
	SCRoundNumber [3]byte
	// next round's seed for VRF based leader election which is the output of this round's leader's proof verification: VerifyBytes
	// _, (next round's seed)RoundSeed := (current round's leader)VrfPubkey.VerifyBytes((current round's leader)proof, (current round's seed)t)
	RoundSeed         [64]byte
	LeadershipProof   [80]byte
	PreviousBlockHash [32]byte
	Timestamp         [4]byte
	//--
	MerkleRootHash  [32]byte
	Version         [4]byte
	LeaderPublicKey [33]byte // see https://medium.com/coinmonks/on-bitcoin-transaction-sizes-97e31bc9d816
	// LeaderPublicKey kyber.Point
	// the combined signature of committee members for each meta/summary blocks should be
	// included in their header `SCBlockHeader` which enables future validation
	BlsSignature BLSCoSi.BlsSignature
}
type SCMetaBlock struct {
	BlockSize                  [3]byte
	SCBlockHeader              *SCBlockHeader
	SCMetaBlockTransactionList *SCMetaBlockTransactionList
}
type TxSummary struct {
	//---
	ServAgrID       []uint64
	ConfirmedPoRCnt []uint64
	//---
}
type SCSummaryBlockTransactionList struct {
	//---
	TxSummary    []*TxSummary
	TxSummaryCnt [2]byte
	//---
	Fees [3]byte
}
type SCSummaryBlock struct {
	BlockSize                     [3]byte
	SCBlockHeader                 *SCBlockHeader
	SCSummaryBlockTransactionList *SCSummaryBlockTransactionList
}

// side chain's Sync transaction is the result of summerizing the summary block of each epoch in side chain
type TxSCSync struct {
	//---
	// this information "should be kept in side chain" in `SCSummaryBlock` and
	// ofcourse be sent to mainchain via `TxSCSync` to make it's effect on mainchain
	ServAgrID       []uint64
	ConfirmedPoRCnt []uint64
	//---
}
