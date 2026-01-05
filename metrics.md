# Metrics we collect when running the experiment and how we collect them:

## Mainchain:


* For Each Transaction Type:
   * case: Regular Payment Transactions: 
        From Queue#2: pop transactions until (pay\_transaction\_share\_of\_block is full) or queue#2 is empty.
        
      * increment a counter for each transaction popped.

      * total\_pay\_tx\_size += pay\_trx\_size

    * case: Propose Transactions, Commit Transactions, Sync Transactions, Storage Payment, PoR Transactions (only for the non-sidechain case):

        * from Queue#1: pop transactions until (other\_transaction\_share\_of\_block\_is\_full) or queue#1 is empty.

        * for each transaction type, increment its corresponding counter

        * total\_other\_trx\_size += other\_trx\_size



* total\_trx\_in\_blocks= sum of transaction counters.

* avg_confirmation_time = sum\_of\_transaction\_confirmation\_times / total\_trx\_in\_blocks


* Effective Blocksize: mc\_block\_header_size + total\_pay\_tx\_size + total\_other\_trx\_size


## Sidechain: 

* case: PoR Transactions:

 * from Queue: pop transactions until (block is full) or queue is empty.

    * increment por\_tx\_counter for each transaction

      * confirmation\_time = 
            (current\_sidechain\_round - sidechain\_round\_it\_got\_into\_queue) 
            + (number\_of\_sc\_rounds\_in\_one\_epoch) x (current_epoch - epoch\_it\_got\_in\_queue) //current\_sidechain\_round resets every epoch.
        
        _example_:
        
 ![simulation/chainBoostFiles/Untitled Diagram.jpg]	(simulation/chainBoostFiles/Untitled Diagram.jpg)
 
 	Let’s suppose a transaction was in the queue on epoch 0, sidechain round 5. Then got confirmed on epoch 1, side chain round 2. And we know that an epoch is 6 sidechain rounds.
Then the confirmation time is 2-5 + 6(1-0), which is 3 rounds.






        
   *  total\_trx\_size += tx\_size



* average\_conf\_time = sum(confirmation\_time) / por\_tx\_counter

* Effective Blocksize: sc\_block\_header\_size + total\_trx\_size 
