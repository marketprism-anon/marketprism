//All Algorand users execute crypto-graphic sortition to determine if they are selected to propose a block in a given round
//Algorand’s blocks consist of a list of transactions,  along with metadata needed by BA⋆.
//Specifically, the metadata consists of
//the round number,
//the proposer’s VRF-based seed,
//a hash of the previous block in the ledger,
//and a time stamp indicating when the block was proposed.
//The list of transactions in a block logically translates to a set of weights for each user’s public key
//(based on the balance of currency for that key), along with the total weight of all outstanding currency.

package vrf

//import "github.com/chainBoostScale/ChainBoost/onet/log"

//Testvrf is called in Start in ChainBoost to test the VRF module
// func Testvrf() {
// 	t := []byte("first test")
// 	VrfPubkey, VrfPrivkey := VrfKeygen()
// 	proof, ok := VrfPrivkey.ProveBytes(t)
// 	if !ok {
// 		log.Lvl2("error while generating proof")
// 	}
// 	r, _ := VrfPubkey.VerifyBytes(proof, t)
// 	if r == true {
// 		log.Lvl2("proof is approved")
// 	} else {
// 		log.Lvl2("proof is rejected")
// 	}
// }

// NextRoundSeed Algorand: "each round's seed is computed by VRF with the seed of the previous round and this is done by the leader of round r-1
// This seed (and the corresponding VRF proof π) is included in every proposed block,
// so that once Algorand reaches agreement on the block for round r−1, everyone knows seed r at the start of round r.
// the selection seed is refreshed once every R rounds: at round r Algorand calls the sortition functions with seed r−1−(r mod R)
// This seed (and the corresponding VRF proof π) is included in every proposed block,
//so that once Algorand reaches agreement on the block for round r−1, everyone knows seed r at the start of round r."
func NextRoundSeed() {
	// ⟨seed r,π⟩←VRF sku(seed r−1||r)
}

//Algorand: "The value of seed0, which bootstraps seed selection, can be chosen at random at the start of Algorand
//by the initial participants (after their public keys are declared) using distributed random number generation"
// ToDoCoder: distributed random number generation
func initialSeed() []byte {
	return []byte("hi")
}
