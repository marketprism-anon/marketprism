// Package protocol implements the BLS protocol using a main protocol and multiple
// subprotocols, one for each substree.
//
// Deprecated: use the bdnproto instead to be robust against the rogue public-key
// attack described here: https://crypto.stanford.edu/~dabo/pubs/papers/BLSmultisig.html
package BLSCoSi

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"github.com/chainBoostScale/ChainBoost/onet/network"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/bls"
	"golang.org/x/xerrors"
)

// Register the protocols
func init() {
	_, err := onet.GlobalProtocolRegister(DefaultProtocolName,
		NewDefaultProtocol)
	log.ErrFatal(err)
	_, err = onet.GlobalProtocolRegister(DefaultSubProtocolName,
		NewDefaultSubProtocol)
	log.ErrFatal(err)
}

// todo:  changed default time out from 10 sec
const DefaultTimeout = 10000 * time.Second
const DefaultSubleaderFailures = 2

// VerificationFn is called on every node. Where msg is the message that is
// co-signed and the data is additional data for verification.
type VerificationFn func(msg, data []byte) bool

// VerifyFn is called to verify a single signature
type VerifyFn func(suite pairing.Suite, pub kyber.Point, msg []byte, sig []byte) error

// SignFn is called to sign the message
type SignFn func(suite pairing.Suite, secret kyber.Scalar, msg []byte) ([]byte, error)

// AggregateFn is called to aggregate multiple signatures and to produce a
// mask of the peer's participation
type AggregateFn func(suite pairing.Suite, mask *sign.Mask, sigs [][]byte) ([]byte, error)

// BlsCosi holds the parameters of the protocol.
// It also defines a channel that will receive the final signature.
// This protocol should only exist on the root node.
type BlsCosi struct {
	*onet.TreeNodeInstance
	Msg            []byte
	Data           []byte
	CreateProtocol CreateProtocolFunction
	Verify         VerifyFn
	Sign           SignFn
	Aggregate      AggregateFn
	// Timeout is not a global timeout for the protocol, but a timeout used
	// for waiting for responses for sub protocols.
	Timeout           time.Duration
	SubleaderFailures int
	Threshold         int
	FinalSignature    chan []byte // final signature that is sent back to client

	StoppedOnce      sync.Once
	SubProtocolsLock sync.Mutex
	SubProtocols     []*SubBlsCosi
	SubProtocolName  string
	VerificationFn   VerificationFn
	Suite            *pairing.SuiteBn256
	SubTrees         BlsProtocolTree
	//  added
	BlockType string // "metablock", "summaryblock"
}

// CreateProtocolFunction is a function type which creates a new protocol
// used in BlsCosi protocol for creating sub leader protocols.

// : changed this from
// type CreateProtocolFunction func(name string, t *onet.Tree, sid onet.ServiceID) (onet.ProtocolInstance, error)
// to: (what's the difference?!)
type CreateProtocolFunction func(name string, t *onet.Tree, sid onet.ServiceID) (onet.ProtocolInstance, error)

// NewDefaultProtocol is the default protocol function used for registration
// with an always-true verification.
// Called by GlobalRegisterDefaultProtocols
func NewDefaultProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewBlsCosi(n, vf, DefaultSubProtocolName, pairing.NewSuiteBn256())
}

// DefaultFaultyThreshold computes the maximum number of faulty nodes
func DefaultFaultyThreshold(n int) int {
	return (n - 1) / 3
}

// DefaultThreshold computes the minimal threshold authorized using
// the formula 3f+1
func DefaultThreshold(n int) int {
	return n - DefaultFaultyThreshold(n)
}

// added
func DefaultBlockType() string {
	return "Meta Block"
}

// NewBlsCosi method is used to define the blscosi protocol.
func NewBlsCosi(n *onet.TreeNodeInstance, vf VerificationFn, subProtocolName string, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	nNodes := len(n.Roster().List)
	c := &BlsCosi{
		TreeNodeInstance:  n,
		FinalSignature:    make(chan []byte, 1),
		Timeout:           DefaultTimeout,
		SubleaderFailures: DefaultSubleaderFailures,
		Threshold:         DefaultThreshold(nNodes),
		Sign:              bls.Sign,
		Verify:            bls.Verify,
		Aggregate:         Aggregate,
		VerificationFn:    vf,
		SubProtocolName:   subProtocolName,
		Suite:             suite,
		//  added
		BlockType: DefaultBlockType(),
	}

	return c, nil
}

// SetNbrSubTree generates N new subtrees that will be used
// for the protocol
func (p *BlsCosi) SetNbrSubTree(nbr int) error {
	if nbr <= 0 {
		return xerrors.New("need at least 1 subtree")
	}
	if nbr > len(p.Roster().List) {
		return xerrors.New("cannot have more subtrees than nodes")
	}
	if p.Threshold == 1 || nbr <= 0 {
		p.SubTrees = []*onet.Tree{}
		return nil
	}

	var err error
	p.SubTrees, err = NewBlsProtocolTree(p.Tree(), nbr)
	if err != nil {
		return xerrors.Errorf("error in tree generation: %v", err)
	}

	return nil
}

// Shutdown stops the protocol
func (p *BlsCosi) Shutdown() error {
	p.StoppedOnce.Do(func() {
		p.SubProtocolsLock.Lock()
		for _, subCosi := range p.SubProtocols {
			// we're stopping the root thus it will stop the children
			// by itself using a broadcasted message
			subCosi.Shutdown()
		}
		p.SubProtocolsLock.Unlock()
		close(p.FinalSignature)
	})

	log.LLvl1("BLS CoSi ends")
	return nil
}

// Dispatch is not used for the main protocol
func (p *BlsCosi) Dispatch() error {
	// This protocol relies only on the start call to spin up sub-protocols
	return nil
}

// Start is done only by root and starts the protocol.
// It also verifies that the protocol has been correctly parameterized.
func (p *BlsCosi) Start() error {
	//if !p.IsRoot() {
	if !p.IsCommitteeRoot() {
		p.Done()
		return xerrors.New("node must be the root")
	}

	if p.SubTrees == nil {
		// the default number of subtree is the square root to
		// distribute the nodes evenly
		if err := p.SetNbrSubTree(int(math.Sqrt(float64(len(p.Roster().
			List))))); err != nil {
			p.Done()
			return xerrors.Errorf("couldn't set subtrees: %v", err)
		}
	}

	err := p.checkIntegrity()
	if err != nil {
		p.Done()
		return xerrors.Errorf("integrity check failed: %v", err)
	}

	log.Lvl1("Starting BLS CoSi on %v", p.ServerIdentity())

	go p.runSubProtocols()

	return nil
}

func (p *BlsCosi) runSubProtocols() {
	// : commented. we don't want the nodes to be done after one round of protocol!
	//defer p.Done()

	// Verification of the data is done before contacting the children
	if ok := p.VerificationFn(p.Msg, p.Data); !ok {
		// root should not fail the verification otherwise it would not have started the protocol
		log.Errorf("verification failed on root node")
		return
	}

	// start all subprotocols
	p.SubProtocolsLock.Lock()
	p.SubProtocols = make([]*SubBlsCosi, len(p.SubTrees))
	for i, tree := range p.SubTrees {
		log.Lvlf3("Invoking start sub protocol on %v", tree.Root.ServerIdentity)
		var err error
		p.SubProtocols[i], err = p.startSubProtocol(tree)
		if err != nil {
			p.SubProtocolsLock.Unlock()
			log.Error(err)
			return
		}
	}
	p.SubProtocolsLock.Unlock()
	log.LLvl1(p.ServerIdentity().Address, "all (: sub bls) protocols started")

	// Wait and collect all the signature responses
	responses, err := p.collectSignatures()
	if err != nil {
		log.Error(err)
		return
	}

	log.LLvl1(p.ServerIdentity().Address, "collected all signature responses")

	// generate root signature
	sig, err := p.generateSignature(responses)
	if err != nil {
		log.Error(err)
		return
	}

	p.FinalSignature <- sig
}

// checkIntegrity checks if the protocol has been instantiated with
// correct parameters
func (p *BlsCosi) checkIntegrity() error {
	if p.Msg == nil {
		return fmt.Errorf("no proposal msg specified")
	}
	if p.CreateProtocol == nil {
		return fmt.Errorf("no create protocol function specified")
	}
	if p.VerificationFn == nil {
		return fmt.Errorf("verification function cannot be nil")
	}
	if p.SubProtocolName == "" {
		return fmt.Errorf("sub-protocol name cannot be empty")
	}
	if p.Timeout < 500*time.Microsecond {
		return fmt.Errorf("unrealistic timeout")
	}
	if p.Threshold > p.Tree().Size() {
		return fmt.Errorf("threshold (%d) bigger than number of nodes (%d)", p.Threshold, p.Tree().Size())
	}
	if p.Threshold < 1 {
		return fmt.Errorf("threshold of %d smaller than one node", p.Threshold)
	}

	return nil
}

// checkFailureThreshold returns true when the number of failures
// is above the threshold
func (p *BlsCosi) checkFailureThreshold(numFailure int) bool {
	return numFailure > len(p.Roster().List)-p.Threshold
}

// startSubProtocol creates, parametrize and starts a subprotocol on a given tree
// and returns the started protocol.
func (p *BlsCosi) startSubProtocol(tree *onet.Tree) (*SubBlsCosi, error) {
	pi, err := p.CreateProtocol(p.SubProtocolName, tree, onet.NilServiceID)
	if err != nil {
		return nil, err
	}
	cosiSubProtocol := pi.(*SubBlsCosi)
	cosiSubProtocol.Msg = p.Msg
	cosiSubProtocol.Data = p.Data
	// Fail fast enough if the subleader is failing to try
	// at least three leaves as new subleader
	cosiSubProtocol.Timeout = p.Timeout / time.Duration(p.SubleaderFailures+1)
	// Give one leaf for free but as we don't know how many leaves
	// could fail from the other trees, we need as much as possible
	// responses. The main protocol will deal with early answers.
	cosiSubProtocol.Threshold = tree.Size() - 1

	log.Lvl2("Starting sub protocol with subleader", tree.Root.Children[0].ServerIdentity)
	err = cosiSubProtocol.Start()
	if err != nil {
		return nil, err
	}
	// I want to see the list of nodes!
	log.Lvl3("Tree used in SubBlsCosi is", tree.Roster.List)
	return cosiSubProtocol, err
}

// Collect signatures from each sub-leader, restart whereever sub-leaders fail to respond.
// The collected signatures are already aggregated for a particular group
func (p *BlsCosi) collectSignatures() (ResponseMap, error) {
	p.SubProtocolsLock.Lock()
	numSubProtocols := len(p.SubProtocols)
	responsesChan := make(chan StructResponse, numSubProtocols)
	errChan := make(chan error, numSubProtocols)
	closeChan := make(chan bool)
	// force to stop pending selects in case of timeout or quick answers
	defer func() { close(closeChan) }()

	for i, subProtocol := range p.SubProtocols {
		go func(i int, subProtocol *SubBlsCosi) {
			for {
				// this select doesn't have any timeout because a global is used
				// when aggregating the response. The close channel will act as
				// a timeout if one subprotocol hangs.
				select {
				case <-closeChan:
					// quick answer/failure
					return
				case <-subProtocol.subleaderNotResponding:
					// x1 := p.SubTrees[i].Root
					// x2 := p.SubTrees[i].Root.Children[0]
					// x3 := p.SubTrees[i].Root.Children[0].RosterIndex
					// log.LLvl1(x1, ":", x2, ":", x3)

					subleaderID := p.SubTrees[i].Root.Children[0].RosterIndex
					log.Lvlf2("(subprotocol %v) subleader with id %d failed, restarting subprotocol", i, subleaderID)

					// generate new tree by adding the current subleader to the end of the
					// leafs and taking the first leaf for the new subleader.
					nodes := []int{p.SubTrees[i].Root.RosterIndex}
					for _, child := range p.SubTrees[i].Root.Children[0].Children {
						nodes = append(nodes, child.RosterIndex)
					}

					if len(nodes) < 2 || subleaderID > nodes[1] {
						errChan <- fmt.Errorf("(subprotocol %v) failed with every subleader, ignoring this subtree",
							i)
						return
					}
					nodes = append(nodes, subleaderID)

					var err error
					p.SubTrees[i], err = GenSubtree(p.SubTrees[i].Roster, nodes)
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in tree generation: %v", i, err)
						return
					}

					// restart subprotocol
					// send stop signal to old protocol
					subProtocol.HandleStop(StructStop{subProtocol.TreeNode(), Stop{}})
					subProtocol, err = p.startSubProtocol(p.SubTrees[i])
					if err != nil {
						errChan <- fmt.Errorf("(subprotocol %v) error in restarting of subprotocol: %s", i, err)
						return
					}

					p.SubProtocolsLock.Lock()
					p.SubProtocols[i] = subProtocol
					p.SubProtocolsLock.Unlock()
				case response := <-subProtocol.subResponse:
					responsesChan <- response
					return
				}
			}
		}(i, subProtocol)
	}
	p.SubProtocolsLock.Unlock()

	// handle answers from all parallel threads
	responseMap := make(ResponseMap)
	numSignature := 0
	numFailure := 0
	timeout := time.After(p.Timeout)
	for numSubProtocols > 0 && numSignature < p.Threshold-1 && !p.checkFailureThreshold(numFailure) {
		select {
		case res := <-responsesChan:
			// changed public from:
			//publics := p.Publics()
			// to:
			publics := p.SubTrees[0].Roster.Publics()
			mask, err := sign.NewMask(p.Suite, publics, nil)
			if err != nil {
				return nil, err
			}
			err = mask.SetMask(res.Mask)
			if err != nil {
				return nil, err
			}

			public, index := searchPublicKey(p.TreeNodeInstance, res.ServerIdentity)
			if public != nil {
				if _, ok := responseMap[index]; !ok {
					count := mask.CountEnabled()
					numSignature += count
					numFailure += res.SubtreeCount() + 1 - count

					responseMap[index] = &res.Response
				}
			}
		case err := <-errChan:
			err = fmt.Errorf("error in getting responses: %s", err)
			return nil, err
		case <-timeout:
			// here we use the entire timeout so that the protocol won't take
			// more than Timeout + root computation time
			return nil, fmt.Errorf("not enough replies from nodes at timeout %v "+
				"for Threshold %d, got %d responses for %d requests", p.Timeout,
				p.Threshold, numSignature, len(p.Roster().List)-1)
		}
	}

	if p.checkFailureThreshold(numFailure) {
		return nil, fmt.Errorf("too many refusals (got %d), the threshold of %d cannot be achieved",
			numFailure, p.Threshold)
	}

	return responseMap, nil
}

// Sign the message with this node and aggregates with all child signatures (in structResponses)
// Also aggregates the child bitmasks
func (p *BlsCosi) generateSignature(responses ResponseMap) (BlsSignature, error) {
	// changed public from:
	//publics := p.Publics()
	// to:
	publics := p.SubTrees[0].Roster.Publics()

	//generate personal mask
	personalMask, err := sign.NewMask(p.Suite, publics, p.Public())
	if err != nil {
		return nil, err
	}

	// generate personal signature and append to other sigs
	personalSig, err := p.Sign(p.Suite, p.Private(), p.Msg)
	if err != nil {
		return nil, err
	}

	// even if there is only one, it is aggregated to include potential processing
	// done during the aggregation
	agg, err := p.Aggregate(p.Suite, personalMask, [][]byte{personalSig})
	if err != nil {
		return nil, err
	}

	_, index := searchPublicKey(p.TreeNodeInstance, p.ServerIdentity())
	// fill the map with the Root signature
	responses[index] = &Response{
		Mask:      personalMask.Mask(),
		Signature: agg,
	}

	// Aggregate all signatures
	sig, err := p.makeAggregateResponse(p.Suite, publics, responses)
	if err != nil {
		log.Lvlf2("%v failed to create aggregate signature", p.ServerIdentity())
		return nil, err
	}
	return sig, err
}

// searchPublicKey looks for the corresponding server identity in the roster
// to prevent forged identity to be used
func searchPublicKey(p *onet.TreeNodeInstance, servID *network.ServerIdentity) (kyber.Point, int) {
	for idx, si := range p.Roster().List {
		if si.Equal(servID) {
			return p.NodePublic(si), idx
		}
	}

	return nil, -1
}

// makeAggregateResponse takes all the responses from the children and the subleader to
// aggregate the signature and the mask
func (p *BlsCosi) makeAggregateResponse(suite pairing.Suite, publics []kyber.Point, responses ResponseMap) (BlsSignature, error) {
	finalMask, err := sign.NewMask(suite, publics, nil)
	if err != nil {
		return nil, err
	}
	finalSignature := suite.G1().Point()

	for _, res := range responses {
		if res == nil || len(res.Signature) == 0 {
			continue
		}

		sig, err := res.Signature.Point(suite)
		if err != nil {
			return nil, err
		}
		finalSignature = finalSignature.Add(finalSignature, sig)

		err = finalMask.Merge(res.Mask)
		if err != nil {
			return nil, err
		}
	}

	sig, err := finalSignature.MarshalBinary()
	if err != nil {
		return nil, err
	}

	log.Lvlf3("%v is done aggregating signatures with total of %d signatures", p.ServerIdentity(), finalMask.CountEnabled())

	return append(sig, finalMask.Mask()...), nil
}

// func aggregate(suite pairing.Suite, mask *sign.Mask, sigs [][]byte) ([]byte, error) {
// 	return bls.AggregateSignatures(suite, sigs...)
// }

// --------------------------------   : added   --------------------------------
func (p *BlsCosi) IsCommitteeRoot() bool {
	if p.TreeNode().Name() == p.SubTrees[0].Root.Name() {
		return true
	} else {
		return false
	}
}
