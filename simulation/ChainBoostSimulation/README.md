

## The Distributed Simulation On The Cluster ##


### The Distributed Simulation Architecture

After building the simulaion on your local machine, it will build two executable files `simul` and `users`, and create two xlsx files (to keep blockchain information) `mainchainbc` and `sidechainbc` which all will be under a created folder in the simulation module named `deploy`. 

Two other config files named `chainBoost.toml` and `ResearchLabab.toml` are transfered to this folder. These files are the ones that the simulation configuration is specified in them. So to set the simulation configuration these two config files should be used. 

After running the local phase of the distributed simulation, all these files (under the `deploy` folder in the local machine) are transferred to the gateway, to your home directory under a folder named `remote`. 

(For now) the executable `users` file is the one that run the simulation from the gateway on the cluster. After running this file the simulation starts and the `mainchainbc` and `sidechainbc` files get updated at the end of each main chain and side chain round JUST by one specific node.

## Commands to Run the simulation:

### Building and preparing the distributed simulation toolkit:

first set your username in

- ResearchLab.toml 

plus from the followinng two places that should be removed once we add dynamic config: 

- https://github.com/chainBoostScale/ChainBoost/blob/986893b53416876cd64a56fd3928a17fc41a6dc5/simulation/platform/ResearchLabab_users/users.go#L161

- https://github.com/chainBoostScale/chainBoost/blob/6f60cef8009c3c3690665774529da70754c2c037/simulation/platform/ResearchLabab.go#L191


Then via the following command build the simulation:
`$ go test -platform=ResearchLabab timeout 300000s -run ^TestSimulation$`

** after building the stimulation, you will see a `simulation started` message in your local machine,  thats where it should return (doesn't now) so you can terminate your local run for now. **

## Running the distributed simulation:

After doing the above steps (for now for each simulation configuration we shuld build and transfer it), then run it from your own home directory `on the gateway` by calling the executable `users` file in the created `remote` directory there. After running this file the simulation starts and the `mainchainbc` and `sidechainbc` files get updated at the end of each main chain and side chain round JUST by one specific node.

### getting simulation result in the distributed simulation

** this process should become automatic **

1- (from gateway):

    `$ rsync root@192.168.3.220:~/remote/mainchainbc.xlsx ~/simulationResult`
    `$ rsync root@192.168.3.220:~/remote/sidechainbc.xlsx ~/simulationResult`

2- open sftp

    `$ sftp user@research-lab-url`
    `$ lcd /Users/Coder/Desktop/ChainBoostDocuments/simulationResult` set it to where your `simulationResult` folder is located.
    `$ get -R simulationResult`


### develop the Local simulation and run it:

- Open a terminal in the directory where the folder ChainBoost is located
- run the following command: 

```
/usr/local/go/bin/go test -timeout 50000s -run ^TestSimulation$ github.com/chainBoostScale/ChainBoost/simulation/manage/simulation
```
OR

open cmd in simulation folder and run the following command:

`$ go test -platform=localhost -timeout 300000s -run ^TestSimulation$`

`$ go test -debug=5 -timeout 30000s -run ^TestSimulation$ github.com/chainBoostScale/ChainBoost/simulation/manage/simulation`

- this will call the TestSimulation function in the file: ([simul_test.go](https://github.com/chainBstSc/ChainBoost/blob/master/simulation/manage/simulation/simul_test.go))
- the stored blockchain in Excel file "mainchainbc.xlsx" and "sidechainbc.xlsx" can be found under the `build` directory that is going to be created after simulation run[^1]
- in the case of debugging the following code in ([simul_test.go](https://github.com/chainBstSc/ChainBoost/blob/master/simulation/manage/simulation/simul_test.go)) indicates the debug logging level, with 0 being the least logging and 5 being the most (every tiny detail is logged in this level)

```
log.SetDebugVisible(1)
```
