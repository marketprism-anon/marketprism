package BLSCoSi

/* : brought from Package bdnproto:
Package bdnproto implements the Boneh-Drijvers-Neven signature scheme
to protect the aggregates against rogue public-key attacks.
This is a modified version of blscosi/protocol which is now deprecated.
package bdnproto */

// ToDoCoder: these material should be put in a different package so that when simulation or
// blockchain want to import ../prototocol, they can import that package (bc can't import ChainBoost)

import (
	"github.com/chainBoostScale/ChainBoost/onet"
	"go.dedis.ch/kyber/v3/pairing"
	"go.dedis.ch/kyber/v3/sign"
	"go.dedis.ch/kyber/v3/sign/bdn"
)

// BdnProtocolName is the name of the main protocol for the BDN signature scheme.
const BdnProtocolName = "bdnCoSiProto"

// BdnSubProtocolName is the name of the subprotocol for the BDN signature scheme.
const BdnSubProtocolName = "bdnSubCosiProto"

// GlobalRegisterBdnProtocols registers both protocol to the global register.
//func GlobalRegisterBdnProtocols() {
func init() {
	onet.GlobalProtocolRegister(BdnProtocolName, NewBdnProtocol)
	onet.GlobalProtocolRegister(BdnSubProtocolName, NewSubBdnProtocol)
}

//NewBdnProtocol is used to register the protocol with an always-true verification.
func NewBdnProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewBdnCosi(n, vf, BdnSubProtocolName, pairing.NewSuiteBn256())
}

// NewBdnCosi makes a protocol instance for the BDN CoSi protocol.
func NewBdnCosi(n *onet.TreeNodeInstance, vf VerificationFn, subProtocolName string, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	c, err := NewBlsCosi(n, vf, subProtocolName, suite)
	if err != nil {
		return nil, err
	}

	mbc := c.(*BlsCosi)
	mbc.Sign = bdn.Sign
	mbc.Verify = bdn.Verify
	mbc.Aggregate = Aggregate

	return mbc, nil
}

// NewSubBdnProtocol is the default sub-protocol function used for registration
// with an always-true verification.
func NewSubBdnProtocol(n *onet.TreeNodeInstance) (onet.ProtocolInstance, error) {
	vf := func(a, b []byte) bool { return true }
	return NewSubBdnCosi(n, vf, pairing.NewSuiteBn256())
}

// NewSubBdnCosi uses the default sub-protocol to make one compatible with
// the robust scheme.
func NewSubBdnCosi(n *onet.TreeNodeInstance, vf VerificationFn, suite *pairing.SuiteBn256) (onet.ProtocolInstance, error) {
	pi, err := NewSubBlsCosi(n, vf, suite)
	if err != nil {
		return nil, err
	}

	subCosi := pi.(*SubBlsCosi)
	subCosi.Sign = bdn.Sign
	subCosi.Verify = bdn.Verify
	subCosi.Aggregate = Aggregate

	return subCosi, nil
}

// aggregate uses the robust aggregate algorithm to aggregate the signatures.
func Aggregate(suite pairing.Suite, mask *sign.Mask, sigs [][]byte) ([]byte, error) {
	sig, err := bdn.AggregateSignatures(suite, sigs, mask)
	if err != nil {
		return nil, err
	}
	return sig.MarshalBinary()
}
