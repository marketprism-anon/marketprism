package network

import (
	"testing"

	"github.com/chainBoostScale/ChainBoost/onet/log"
	_ "go.dedis.ch/kyber/v3/group/edwards25519"
	"go.dedis.ch/kyber/v3/suites"
)

var tSuite = suites.MustFind("Ed25519")

func TestMain(m *testing.M) {
	log.MainTest(m)
}
