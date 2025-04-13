# Main and Side Chain #

Blockchain is stored in two layers: 
- in a file on hardware 
- and in the storage of a root node in the protocol. 
So when each node sends and receives a message from the root node, it is equal to accessing the blockchain [^bcassump], reading from it or writing in it (in the case of the next block, if having a valid leadership proof).

## Main Chain protocol ##

Main Chain consensus:
--------------------------

1. On each round, all nodes check for their leadership by comparing their current power with **their own vrf output**.

2. if it was bigger than their random output they send a message to bc-layer1 announcing their leadership.

3. bc-layer 1 can verify their leadership argument [^vrf] and the first leader that has announced her leadership will get to be the round’s leader. [^mcleaderassump]

4. Bc-layer1 update the bc-layer2 using the config params, the selected leader’s info Etc.

5. once the round duration is finished, it announces the next round’s start by directly sending a message to all nodes. This is equal to having all nodes observe bc and once the new block is added (after round duration) they start the next round.


## Main Chain Blockcahin ##

- ServAgr stands for `service agreement`
- por stands for `proof of retrievibility`

There are 6 sheets, namely MarketMatching, FirstQueue, SecondQueue, RoundTable, PowerTable, and Overall Evaluation. [^sheets]

- `MarketMatching`: the overall information about the market matching
    - about the servers: IP, 
    - about the ServAgr: ID, duration, File size, and starting round#, isPublished (if a ServAgr get expired, the column published is set to 0 until its poropose and commit transaction get submitted to the blockchain again)
- `PowerTable`: A matrix of each server's added power in each round
- `FirstQueue`: there are 5 types of trransactions in there
    - `propose ServAgr': including the information of teh ServAgr and the client's payment for it
    - `commit ServAgr`: in which the server commits to the ServAgr id already published by the client
    - `por`: for each active (not expired) ServAgr each server issue ane por
    - `storage payment`: after the ServAgr duration pass and a ServAgr expires, this transaction is assued to pay for the service
- `SecondQueue`: the queue of regular payment transactions
- `RoundTable`: the overall information of the blockchain including:
    - each round's seed
    - the added block size
    - IP of the leader in each round
    - number of each transaction type that is submitted in each round
    - `TotalNumTxs`: total number of all submitted transactions in each round
    - the time that each round has started
    - `AveWait-RegPay` and `AveWait-OtherTxs`: the average wait time in each round for regular payment and other types of transactions
    - `RegPaySpaceFull` and `BlockSpaceFull`: 1 indicates the allocated space for regular payment is full /  the block space is full
- `Overall Evaluation`: the overall results up until each round


## Side Chain Blockcahin ##

There are 3 sheets, namely FirstQueue, RoundTable, and Overall Evaluation.[^sheets]



<!--FootNote-->
[^bcassump]: assumption: All nodes have a same view of the blockchain.
[^vrf]: verify vrf is implemented but it is not added here.
[^sc]: if the SimState parameter in the config file be set to 1, the side chain protocol will not be run atall and the side chain blockchain file will be empty (i.e. there will be no side chain blockchain)
[^sheets]: these sheets are updated each round so, after running the simulation, we can track the blockchain's progress while running. try opening the file and closing if you are using microsoft or just refreshing the file if you are opening it in visual studio code IDE.
[^mcleaderassump]: the simplifying assumption here is that in each round multiple nodes may have bigger power than their vrf output hence get selected, different systems have different methods to resolve this multiple branch issue, some are trying to have a single winning node, some define a rule to pick one of them. Here for simpllicity we accept thefirst announcement (the first leader that has announce his leadership) as the round's leader.
<!--FootNote-->
