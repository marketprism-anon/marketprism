
# MarketPrism. #

MarketPrism's official implementation in Go..

MarketPrism is a secure performance booster for blockchain-based resource markets.

We used latest version of [Onet](https://github.com/dedis/onet/tree/v3.2.9) (v.3.2.9) at the time for network, simulation, and communication modules 
as well as [Cosi](https://github.com/dedis/cothority) module from Cothority. 
We used [Kyber](https://github.com/dedis/kyber) for advanced cryptographic primitives.


## Getting Started ###
note: running on an OS other than IOS needs a change in c extention config code

- Install Go
- Clone or Downloade the marketprism's source code from Git <https://github.com/marketprism-anon/marketprism>
- Open a terminal in the directory where the folder marketprism is located
- in the case of debugging the following code in ([simul_test.go](https://github.com/marketprism-anon/marketprism/blob/3P1M1D/simulation/manage/simulation/simul_test.go)) indicates the debug logging level, with 0 being the least logging and 5 being the most (every tiny detail is logged in this level)
```
log.SetDebugVisible(1)
```

## Config File ##

Config File "ChainBoost.toml" is located under the following directory:
([ChainBoost.toml](https://github.com/marketprism-anon/marketprism/blob/3P1M1D/simulation/manage/simulation/ChainBoost.toml))


## Project Layout ##

`marketprism` is split into various subpackages.

The following packages provide core functionality to `marketprism`:

--------------------------------------------------------------------------------------------------
1. these modules from Dedis Lab are used with few modifications: [Onet](https://github.com/marketprism-anon/marketprism/tree/3P1M1D/onet) (including `Network`, `Overlay`, and `Log`) and `Simulation` (See: [Onet ReadMe File](https://github.com/dedis/onet/blob/3P1M1D/README.md))
2. This module is used intact: `Kyber` from Dedis Lab (See: [Kyber ReadMe File](https://github.com/dedis/kyber/blob/master/README.md))
3. This module from Algorand is used with some modifications: [VRF](https://github.com/marketprism-anon/marketprism/tree/3P1M1D/vrf) (See: [VRF ReadMe File](https://github.com/marketprism-anon/marketprism/blob/3P1M1D/vrf/ReadMe.MD))
4. Added modules for marketprism:
- [PoR](https://github.com/marketprism-anon/marketprism/tree/3P1M1D/por) (See: [PoR ReadMe File](https://github.com/marketprism-anon/marketprism/blob/3P1M1D/por/README.md))
- [MainandSideChain](https://github.com/marketprism-anon/marketprism/tree/3P1M1D/MainAndSideChain) including (See: [MainandSideChain ReadMe File](https://github.com/marketprism-anon/marketprism/blob/3P1M1D/MainAndSideChain/ReadMe.MD))
  - `Blockchain` package for tx, block structure, measurement, management of tx queues, management of blockchain in two layers 
  - main and side chain's `Consensus protocol` (`BlsCosi` is used for sideChain. part of it is brought from Dedis’s `BlsCosi` (See: [BLSCosSi ReadMe file](https://github.com/dedis/cothority/blob/main/blscosi/README.md) with some modifications applied)
--------------------------------------------------------------------------------------------------

<!--FootNote-->
[^1]: there may be some rounds that there is no leader for them, an empty block will be added to the blockchain in those rounds and the information of the root node (blockchain layer 1) is added (it can be removed) as the round leader but all the other columns are empty. in these rounds transactions will be added normally to the queue but no transaction is removed bcz the block is empty.
[^2]: when in a round, some transactions should wait in a queue (i.e. the allocated space for  that transaction is full) and are submitted in another round, the average wait of that queue in the round that those transactions get to be submitted increases.
<!--FootNote-->


## On Building using Makefiles

this repo contains three Makefiles, one on the root of the repo, one in `simulation/manage/simulation/Makefile` and one in `simulation/platform/ResearchLabab_users/Makefile`

The Makefile in the root of the repo builds all the binaries (simul and users) and puts them in the build folder,
it allows the following commands:

* `make build` : builds the binaries
* `make deploy USER=<user>` : builds the binaries and deploys them to gateway using rsync over SSH
* `make clean`: cleans up all the binaries that were built.

The in-folder Makefiles are helper Makefiles for the one in the root of the repo, and are used to build their respective binaries

## On Running the exeperiment using the orchestrator:

The orchestrator is a production ready way to orchestrate different instances of the simulation executable. in order to run the experiment:
1) edit `ssh.toml` to match your expected files to be uploaded to the VMs and the ones you want to retrieve after the experiments are over.
2) run `./orchestrator ssh.toml`

Note: The logs of every simul instance are written under the folder `<vm-ip-address>/stdout.txt` , it contains stdout and stderr outputs

# Time Out
Timeouts are parsed according to Go's time.Duration: A duration string
is a possibly signed sequence of decimal numbers, each with optional
fraction and a unit suffix, such as "300ms", "-1.5h" or "2h45m". Valid
time units are "ns", "us" (or "µs"), "ms", "s", "m", "h".

This is a list of timeouts that we want to control or may want to keep static but edit according to our network setting:

### Network Level
- In the overlay layer, there is a GlobalProtocolTO of 10 mins (I increased it from 1 min to be sure it is not causing error!), 
- In the Server file, in TryConn a 2 sec listening TO and a 10 sec TO for getting access to IPs
- In the TCP files, a globalTO of 1 minute (increased now!) for connection TO and a dialTO of 1 minute (increased now!) which the later has a function for changing it.
- A 20 seconds TO for SSH

###  A speciall TimeOut
- A joining TO of 20 sec for trying to invite the nodes to join the simulation.

### Prrotocol(s) Level
- BLSCoSi has a TO which is set to 100 secs and is for waiting for response from the subprotocol

In the `marketprism.toml` config file:
- A `RunWait` parameter which shows how many seconds to wait for a run (one line of .toml-file) to finish
- A `timeout` shows how many seconds to wait for the while experiment to finish (default: RunWait \* #Runs)
