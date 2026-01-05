// Package por
//-----------------------------------------------------------------------
//  --------------- Compact PoR ----------------------------------------
//-----------------------------------------------------------------------
/* https://link.springer.com/article/10.1007%2Fs00145-012-9129-2
-----------------------------------------------------------------------
The information that a miner will need to verify a por:
	1- To authenticate File Tag:
		a. File owner's pk1
	2- To verify proof
		a. File owner's pk2
		b. The por
			i. Sigma		kyber.Point
			ii. Mu          [s]kyber.Scalar
	3- The file tag (Tau)
	4- Access to "the query!":
The round seed should generate identical query (with the one that storage server got its challenged)
*/

/*
At the 128-bit security level, a nearly optimal choice for
a pairing-friendly curve is a Barreto-Naehrig (BN) curve over
a prime field of size roughly 256 bits with embedding degree k= 12.
bits of security: 128 bits / 124 bits.

the bn256 implemented in kyber is from paper "ew software speed
records for cryptographic pairings" which gives us:
- BN curve over a prime field Fp of size 257 bits.
- The prime p is given by the BN polynomial parametrization p = 36u4+36u3+24u2+6u+1,
where u = v3 and v = 1966080.
The curve equation is E : y2 = x3 +17.
*/

//ToDoCoder: Random Query and File tag should be generated and parsed from the current round's seed as a source of randomness
package por

import (
	"bytes"
	"encoding/binary"
	"math/rand"
	"strconv"

	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/pairing/bn256"
	"go.dedis.ch/kyber/v3/sign/bls"
	"go.dedis.ch/kyber/v3/sign/schnorr"
	"go.dedis.ch/kyber/v3/util/key"
	"go.dedis.ch/kyber/v3/util/random"
	"go.dedis.ch/kyber/v3/xof/blake2xb"
)

//const S = 1 // number of sectors in eac block (sys. par.)
// Each sector is one element of Zp, and there are s sectors per block.
// If the processed file is b bits long,then there are n = [b/s lg p] blocks.

// number of blocks
const n = 1000

//size of query set (i<n)
const l = 5

var Suite = pairing.NewSuiteBn256()

//
var TauSize int

//-----------------------------------------------------------------------
// sigma_i is an element of G
// the length of sigma array is equal to n value
// (number of blocks in each file which is a function of file size and sector number)
// this is the storage overhead!
type processedFile struct {
	m_ij  initialFile
	sigma [n]kyber.Point
}

//m_ij is an elemnt of Z_P
type initialFile struct {
	m [n][]kyber.Scalar
}
type randomQuery struct {
	i   [l]int
	v_i [l]kyber.Scalar
}

// the length of Mu array is equal to s value (sector  number set in config params)
// this is the communication overhead!
type Por struct {
	Mu    []kyber.Scalar
	Sigma kyber.Point
}

type PrivateKey struct {
	alpha kyber.Scalar
	ssk   kyber.Scalar //ToDoCoder:  these keys should be united if its possible!
}
type PublicKey struct {
	v   kyber.Point
	spk kyber.Point
}
type hashablePoint interface{ Hash([]byte) kyber.Point }

// utility functions
func RandomizedKeyGeneration() (PrivateKey, PublicKey) {
	//randomizedKeyGeneration: pubK=(alpha,ssk),prK=(v,spk)
	clientKeyPair := key.NewKeyPair(onet.Suite) // onet suit is "Ed25519"
	ssk := clientKeyPair.Private
	spk := clientKeyPair.Public
	//BLS keyPair
	//Package bn256: implements the Optimal Ate pairing over a
	//256-bit Barreto-Naehrig curve as described in
	//http://cryptojedi.org/papers/dclxvi-20100714.pdf.
	//claimed 128-bit security level.
	//Package bn256 from kyber library is used in blscosi module for bls scheme
	private, public := bls.NewKeyPair(Suite, random.New())
	alpha := private
	v := public
	// make it two functions that give private and public keys separately and the private one should not be accessible publicly.
	return PrivateKey{
			alpha,
			ssk,
		}, PublicKey{
			spk: spk,
			v:   v,
		}
}
func GenerateFile(SectorNumber int) initialFile {
	// first apply the erasure code to obtain M′; then split M′
	// into n blocks (for some n), each s sectors long:
	// {mij} 1≤i≤n 1≤j≤s
	var m_ij [n][]kyber.Scalar
	for i := 0; i < n; i++ {
		for j := 0; j < SectorNumber; j++ {
			m_ij[i] = append(m_ij[i], Suite.Scalar().Pick(Suite.RandomStream()))
		}
	}
	return initialFile{m: m_ij}
}
func randomizedVerifyingQuery(SimulationSeed int) *randomQuery {

	rand.Seed(int64(SimulationSeed))
	var randombyte = make([]byte, 8)
	binary.LittleEndian.PutUint64(randombyte, uint64(SimulationSeed))
	var rng = blake2xb.New(randombyte)
	//-----------------------
	var b [l]int
	var v [l]kyber.Scalar
	for i := 0; i < l; i++ {
		b[i] = rand.Intn(n)
		v[i] = Suite.Scalar().Pick(rng)
	}
	return &randomQuery{i: b, v_i: v}
}

// RandomizedFileStoring
/*
	This function is called by the """file owner""" to create
			// file tag -
			// file authentication values -
			// and key-pair
	// The file tag and the public key pair should be stored on the bc (to be verified along with por proof later)
	// and the authentication values should be sent and stored on prover's server , he will need it to create por
*/
func RandomizedFileStoring(sk PrivateKey, initialfile initialFile, SectorNumber int) ([]byte, processedFile) {
	m_ij := initialfile.m
	//u1,..,us random from G
	var u []kyber.Scalar
	var U []kyber.Point
	var st1, st2 bytes.Buffer
	for j := 0; j < SectorNumber; j++ {
		rand := random.New()
		u = append(u, Suite.G1().Scalar().Pick(rand))
		U = append(U, Suite.G1().Point().Mul(u[j], nil))
		t, _ := u[j].MarshalBinary()
		st1.Write(t)
	}

	//Tau0 := "name"||n||u1||...||us
	//Tau=Tau0||Ssig(ssk)(Tau0) "File Tag"

	//a random file name from some sufficiently large domain (e.g.,Zp)
	aRandomFileName := random.Int(bn256.Order, random.New())
	st2.Write(aRandomFileName.Bytes())
	st2.Write([]byte(strconv.Itoa(n)))
	st2.ReadFrom(&st1)
	Tau0 := st2
	//sg, _ := schnorr.Sign(onet.Suite, sk.ssk, []byte(Tau0))
	//Tau := Tau0 + string(sg)
	sg, _ := schnorr.Sign(onet.Suite, sk.ssk, Tau0.Bytes())
	Tau := append(Tau0.Bytes(), sg...)
	TauSize = len(Tau)

	// we need a BLS hash here. I brought this from kyber.bls.sign
	// https://github.com/dedis/kyber/blob/b627bb323bc7380f4c09d803208a18b7624e1ec1/sign/bls/bls.go
	// ----  isn't there another way?---------------------------------------
	hashable, ok := Suite.G1().Point().(hashablePoint)
	if !ok {
		log.LLvl1("point needs to implement hashablePoint")
	}
	// --------------------------------------------------------------------
	//create "AuthValue" (Sigma_i) for block i
	//Sigma_i = Hash(name||i).P(j=1,..,s)u_j^m_ij
	var b [n]kyber.Point
	for i := 0; i < n; i++ {
		h := hashable.Hash(append(aRandomFileName.Bytes(), byte(i)))
		p := Suite.G1().Point().Null()
		for j := 0; j < SectorNumber; j++ {
			p = Suite.G1().Point().Add(p, Suite.G1().Point().Mul(m_ij[i][j], U[j]))
		}
		b[i] = Suite.G1().Point().Mul(sk.alpha, p.Add(p, h))
	}
	return Tau, processedFile{ // why this func is returning initial file again?!
		m_ij:  initialFile{m: m_ij},
		sigma: b,
	}
}

// CreatePoR this function will be called by the server who wants to create a PoR
// in the paper this function takes 3 inputs: public key , file tag , and processedFile -
// I don't see why the first two parameters are needed!
func CreatePoR(processedfile processedFile, SectorNumber, SimulationSeed int) Por {
	// "the query can be generated from a short seed using a random oracle,
	// and this short seed can be transmitted instead of the longer query."
	// note: this function is called by the verifier in the paper but a prover who have access
	// to the random seed (from blockchain) can call this (and get the query) herself.
	rq := randomizedVerifyingQuery(SimulationSeed)
	m_ij := processedfile.m_ij.m
	sigma := processedfile.sigma
	var Mu []kyber.Scalar
	for j := 0; j < SectorNumber; j++ {
		tv := Suite.Scalar().Zero()
		for i := 0; i < l; i++ {
			//Mu_j= S(Q)(v_i.m_ij)
			tv = Suite.Scalar().Add(Suite.Scalar().Mul(rq.v_i[i], m_ij[rq.i[i]][j]), tv)
		}
		Mu = append(Mu, tv)
	}
	p := Suite.G1().Point().Null()
	for i := 0; i < l; i++ {
		//sigma=P(Q)(sigma_i^v_i)
		t := rq.i[i]
		p = Suite.G1().Point().Add(p, Suite.G1().Point().Mul(rq.v_i[i], sigma[t]))
	}
	return Por{
		Mu:    Mu,
		Sigma: p,
	}
}

// VerifyPoR servers will verify por tx.s when they receive it
// in the paper this function takes 3 inputs: public key , private key, and file tag
// I don't see why the private key is needed!
func VerifyPoR(pk PublicKey, Tau []byte, p Por, SectorNumber, SimulationSeed int) (bool, error) {
	rq := randomizedVerifyingQuery(SimulationSeed)
	//check the file tag (Tau) integrity
	error := schnorr.Verify(onet.Suite, pk.spk, Tau[:32+len(strconv.Itoa(n))+SectorNumber*32], Tau[32+len(strconv.Itoa(n))+SectorNumber*32:])
	if error != nil {
		log.LLvl1("issue in verifying the file tag signature:", error)
	}
	//extract the random selected points
	rightTermPoint := Suite.G1().Point().Null() // pairing check: right term of right hand side
	for j := 0; j < SectorNumber; j++ {
		//These numbers (length of components) can be extracted from calling
		//some utility function in kyber. in case we wanted to customize stuff!
		thisPoint := Tau[32+len(strconv.Itoa(n))+j*32 : 32+len(strconv.Itoa(n))+(j+1)*32]
		u := Suite.Scalar().SetBytes(thisPoint)
		U := Suite.G1().Point().Mul(u, nil)
		rightTermPoint = rightTermPoint.Add(rightTermPoint, U.Mul(p.Mu[j], U))
	}

	hashable, ok := Suite.G1().Point().(hashablePoint)
	if !ok {
		log.LLvl1("point needs to implement hashablePoint")
	}
	// --------------------------------------------------------------------
	leftTermPoint := Suite.G1().Point().Null() // pairing check: left term of right hand side
	for i := 0; i < l; i++ {
		h := hashable.Hash(append(Tau[:32], byte(rq.i[i])))
		//h := hashable.Hash(append(Tau[:1],byte(rq.i[i])))
		leftTermPoint = Suite.G1().Point().Add(leftTermPoint, Suite.G1().Point().Mul(rq.v_i[i], h))
	}
	//checking: e(sigma, g) =? e(PHash(name||i)^v_i.P(j=1,..,s)(u_j^mu_j,v)
	right := Suite.Pair(Suite.G1().Point().Add(leftTermPoint, rightTermPoint), pk.v)
	left := Suite.Pair(p.Sigma, Suite.G2().Point().Base())
	if !left.Equal(right) {
		log.LLvl1("err")
	}
	var refuse = false
	return refuse, error
}

// -------------------------------------------------------------//
