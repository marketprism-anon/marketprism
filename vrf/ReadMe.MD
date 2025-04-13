# VRF #

The VRF module is brought from [Algorand](https://github.com/algorand/go-algorand)
and more specifically from the implemented VRF located in the following branch:
<https://github.com/algorand/go-algorand/tree/1504f5f39ec2893d2b0ac9b60d1b87f6e7d4e2fc/crypto>

All Algorand users execute cryptographic sortition to determine if they are selected to propose a block in a given round.

---------------------------------
- [ ] In ByzcoinX: RandHound leader is selected by the VRF-based leader election
- [ ] in Algorand: Election of the committee is private and totally random (via the VRF based technique combined with proof of stake)


The VRF module is brought from (https://github.com/algorand/go-algorand/pull/2310) which is a pure Go VRF, aand a pull request to Algorand not yet approved/merged.
