# How to add an extra sidechain to the codebase:

## Chainboost.toml modification:

in Chainboost.toml, add the name of your new new sidechain to `Sidechains`

## SideChainProtocol(ChainBoost).go modifications:

Create a new copy of the following structures:

~~~golang
type RtLSideChainNewRound struct {
	SCRoundNumber            int
	CommitteeNodesTreeNodeID []onet.TreeNodeID
	blocksize                int
}
type RtLSideChainNewRoundChan struct {
	*onet.TreeNode
	RtLSideChainNewRound
}
type LtRSideChainNewRound struct { //fix this
	NewRound      bool
	SCRoundNumber int
	SCSig         BLSCoSi.BlsSignature
}
type LtRSideChainNewRoundChan struct {
	*onet.TreeNode
	LtRSideChainNewRound
}
~~~

## in ChainboostSimulation.go

Add a couple new variants of `RtLSideChainNewRoundChan` and `LtRSideChainNewRoundChan` to the Chainboost struct.

Add processing code to the dispatch function.

Add a new instance of `CommitteeNodesTreeNodeID` and `NextSideChainLeader` to Chainboost struct


## in runsimul.go

Register your new `RtLSideChainNewRoundChan` and `LtRSideChainNewRoundChan` variants with the protocol.

## SideChainProtocol(ChainBoost).go

Add Chain Specific handling codes (check if statements for if `name == "por"`)
Add Code to update sidechain committees (TBD - needs implementing priority for miners.)

## in ChainboostSimulation.go:

Update the CoSi handling code with sidechain specific SendTo.







