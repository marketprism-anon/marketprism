package testconsensus

import (
	"flag"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/chainBoostScale/ChainBoost/MainAndSideChain/BLSCoSi"
	"github.com/chainBoostScale/ChainBoost/onet"
	"github.com/chainBoostScale/ChainBoost/onet/log"
	"github.com/stretchr/testify/require"
	"go.dedis.ch/kyber/v3"
	"go.dedis.ch/kyber/v3/pairing"
)

var defaultTimeout = 20 * time.Second
var testSuite = pairing.NewSuiteBn256()

type Counter struct {
	veriCount   int
	refuseIndex int
	sync.Mutex
}

type Counters struct {
	counters []*Counter
	sync.Mutex
}

func (co *Counters) add(c *Counter) {
	co.Lock()
	co.counters = append(co.counters, c)
	co.Unlock()
}

func (co *Counters) size() int {
	co.Lock()
	defer co.Unlock()
	return len(co.counters)
}

func (co *Counters) get(i int) *Counter {
	co.Lock()
	defer co.Unlock()
	return co.counters[i]
}

var counters = &Counters{}

// verify function that returns true if the length of the data is 1.
func verify(msg, data []byte) bool {
	c, err := strconv.Atoi(string(msg))
	if err != nil {
		log.Error("Failed to cast msg", msg)
		return false
	}

	if len(data) == 0 {
		log.Error("Data is empty.")
		return false
	}

	counter := counters.get(c)
	counter.Lock()
	counter.veriCount++
	log.Lvl4("Verification called", counter.veriCount, "times")
	counter.Unlock()
	if len(msg) == 0 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

// verifyRefuse will refuse the refuseIndex'th calls
func verifyRefuse(msg, data []byte) bool {
	c, err := strconv.Atoi(string(msg))
	if err != nil {
		log.Error("Failed to cast", msg)
		return false
	}

	counter := counters.get(c)
	counter.Lock()
	defer counter.Unlock()
	defer func() { counter.veriCount++ }()
	if counter.veriCount == counter.refuseIndex {
		log.Lvl2("Refusing for count==", counter.refuseIndex)
		return false
	}
	log.Lvl3("Verification called", counter.veriCount, "times")
	if len(msg) == 0 {
		log.Error("Didn't receive correct data")
		return false
	}
	return true
}

// ack is a dummy
func ack(a, b []byte) bool {
	return true
}

func TestMain(m *testing.M) {
	flag.Parse()
	log.MainTest(m)
}

func TestBftCoSiRefuse(t *testing.T) {
	t.Skip("doesn't work with new onet testing...")
	const protoName = "TestBftCoSiRefuse"
	if _, err := onet.GlobalProtocolRegister(protoName, BLSCoSi.NewBdnProtocol); err != nil {
		panic(err)
	}
	// the refuseIndex has both leaf and sub leader failure
	configs := []struct{ n, f, r int }{
		{4, 0, 3},
		{4, 0, 1},
		{9, 0, 9},
		{9, 0, 1},
	}
	for _, c := range configs {
		runProtocol(t, c.n, c.f, c.r, protoName, 0)
	}
}

func TestBftCoSiFault(t *testing.T) {
	const protoName = "TestBftCoSiFault"

	if _, err := onet.GlobalProtocolRegister(protoName, BLSCoSi.NewBdnProtocol); err != nil {
		panic(err)
	}
	configs := []struct{ n, f, r int }{
		{3000, 1000, 0},
		{9, 2, 0},
		{10, 3, 0},
	}
	for _, c := range configs {
		runProtocol(t, c.n, c.f, c.r, protoName, 0)
	}
}

func runProtocol(t *testing.T, nbrHosts int, nbrFault int, refuseIndex int, protoName string, scheme int) {
	log.Lvlf1("Starting with %d hosts with %d faulty ones and refusing at %d. Protocol name is %s",
		nbrHosts, nbrFault, refuseIndex, protoName)
	local := onet.NewLocalTest(testSuite)
	defer local.CloseAll()

	servers, roster, tree := local.GenTree(nbrHosts, false)
	require.NotNil(t, roster)

	pi, err := local.CreateProtocol(protoName, tree, onet.NilServiceID)
	require.NoError(t, err)

	publics := roster.Publics()
	bftCosiProto := pi.(*BLSCoSi.BlsCosi)
	bftCosiProto.CreateProtocol = local.CreateProtocol
	bftCosiProto.FinalSignature = make(chan []byte, 1)
	bftCosiProto.SetNbrSubTree(1)
	counter := &Counter{refuseIndex: refuseIndex}
	counters.add(counter)
	proposal := []byte(strconv.Itoa(counters.size() - 1))
	bftCosiProto.Msg = proposal
	bftCosiProto.Data = []byte("hello world")
	bftCosiProto.Timeout = defaultTimeout
	bftCosiProto.Threshold = nbrHosts - nbrFault
	log.Lvl3("Added counter", counters.size()-1, refuseIndex)

	//require.True(t, int(math.Pow(float64(bftCosiProto.nSubtrees),
	//	2.0)) <= nbrHosts)

	// kill the leafs first
	nbrFault = min(nbrFault, len(servers))
	for i := len(servers) - 1; i > len(servers)-nbrFault-1; i-- {
		log.Lvl3("Pausing server:", servers[i].ServerIdentity.Public, servers[i].Address())
		servers[i].Pause()
	}

	err = bftCosiProto.Start()
	require.NoError(t, err)

	// verify signature
	err = getAndVerifySignature(bftCosiProto.FinalSignature, publics, proposal, scheme)
	require.NoError(t, err)

	// check the counters
	counter.Lock()
	defer counter.Unlock()

	// We use <= because the verification function may be called more than
	// once on the same node if a sub-leader in ftcosi fails and the tree is
	// re-generated.
	require.True(t, nbrHosts-nbrFault <= counter.veriCount)
}

func getAndVerifySignature(sigChan chan []byte, publics []kyber.Point, proposal []byte, scheme int) error {
	var sig []byte
	timeout := defaultTimeout + time.Second
	select {
	case sig = <-sigChan:
	case <-time.After(timeout):
		return fmt.Errorf("didn't get commitment after a timeout of %v", timeout)
	}

	// verify signature
	if sig == nil {
		return fmt.Errorf("signature is nil")
	}

	err := func() error {
		switch scheme {
		case 1:
			return BLSCoSi.BdnSignature(sig).Verify(testSuite, proposal, publics)
		default:
			return BLSCoSi.BlsSignature(sig).Verify(testSuite, proposal, publics)
		}
	}()
	if err != nil {
		return fmt.Errorf("didn't get a valid signature: %s", err)
	}
	log.Lvl2("Signature correctly verified!")
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
