Decode Config File
--------------------

 The configuration-file has the following syntax:
 ```
 Name1 = value1
 Name2 = value2
 [empty line]
 n1, n2, n3, n4
 v11, v12, v13, v14
 v21, v22, v23, v24
```
 The Name1...Namen are global configuration-options.
 n1..nn are configuration-options for one run
 Both the global and the run-configuration are copied to both
 the platform and the app-configuration.

## PreScript file ##

About preScript file which runs before simulation on all servers:
In localHost.go file (it should be in another file when the simulation is distributed), in Deploy function, at one point, it says:
// Check for PreScript and copy it to the deploy-dir

In the simulation config file, there is a line that specify preScript file:
```
PreScript = "testPreScript.toml"
```

Now I have added the following address (to the code) to look for it in there: "/Users//Documents/GitHub/chainBoostScale/ChainBoost/simulation/platform/"
The code copy this file to where it call deploy-dir which is: "/Users//Documents/GitHub/chainBoostScale/ChainBoost/simulation/manage/simulation/build"

Later, again in localhost.go file, in the Start() function, it run the preScript file:
 
The code:
```
exec.command("sh", -c , ... ) 
```
will run the following shell-script.

//To create a file in each server: 

```
touch mainbchainbc.csv
```


This file will be created in the deploy-dir

To write/read/update this file:
------------------------------
In the initialization phase, the data comes from the simulation “config file”
In build.go, after deploy.start() (which is where the prescript is decoded and runned and the centralbc file is generated and its directory is deploy-dir which is “/simulation/manage/simulation/build/”) we have:
```
err := deployP.Start()
// Coder: initializing central blockchain -------------------------
   blockchain.InitializeCentralBC(rc.Get("RoundDuration"),
           rc.Get("PercentageTxPay"),
           rc.Get("DistributionMeanFileSize"), rc.Get("DistributionVarianceFileSize"),
           rc.Get("DistributionMeanContractDuration"), rc.Get("DistributionVarianceContractDuration"),
           rc.Get("Nodes"),
           rc.Get("BlockSize"))
// --------------------------------------------
```
to add other system-wide configurations params to existing Onet’s system-wide configurations params, I added few lines of codes in following files:
1- localhost.go
2- build.go
3- platform.go

- [ ] For future reference: The modification I made in the Onet’s module, I start them with the “Coder” keyword. So they are easy to find!

3- In platform.go I modified the  "Config struct" and added our params.
```
type Config struct {
   // string denoting the group used for simulations
   // XXX find ways to remove that "one suite" assumption
   Suite       string
   Debug       int
   // : adding some other system-wide configurations
   RoundDuration      string
   PercentageTxPay    string
   BlockSize string
}
```

This struct is used for all platforms (local host, mininet, ResearchLabab). This means we should not have to make any change when changing platform from local host to mininet (multiple servers)

2- In build.go I read these params from our protocol’s config file (.toml) and send them to platform.Configure function, which is again a platform independent change:


```
deployP.Configure(&platform.Config{
Debug:       log.DebugVisible(),
Suite:       runconfigs[0].Get("Suite"),
// : adding some other system-wide configurations
RoundDuration:      runconfigs[0].Get("RoundDuration"),
PercentageTxPay:    runconfigs[0].Get("PercentageTxPay"),
BlockSize:          runconfigs[0].Get("BlockSize"),
       })
```


1- In localhost.go, first I added those params to “Localhost struct” 
```
type Localhost struct {
...
   // : adding some other system-wide configurations
   RoundDuration      string
   PercentageTxPay    string
   BlockSize string
}
```


and then modified the
```
 “func (d *Localhost) Configure(pc *Config)” function, so when it is called in build.go, it initializes its params with the added input parameters:

// Configure various internal variables
func (d *Localhost) Configure(pc *Config) {
   d.Lock()
   defer d.Unlock()
   pwd, _ := os.Getwd()
   d.runDir = pwd + "/build"
   os.RemoveAll(d.runDir)
   log.ErrFatal(os.Mkdir(d.runDir, 0770))
   d.Suite = pc.Suite
   // ------------------------------
   // : adding some other system-wide configurations
   d.RoundDuration = pc.RoundDuration
   d.PercentageTxPay = pc.PercentageTxPay
   d.BlockSize = pc.BlockSize
   // ------------------------------
   d.localDir = pwd
   d.debug = pc.Debug
   d.running = false
   if d.Simulation == "" {
       log.Fatal("No simulation defined in simulation")
   }
   //log.LLvl3(fmt.Sprintf("Localhost dirs: RunDir %s", d.runDir))
   //log.LLvl3("Localhost configured ...")
}
```
So, what we have at the end is:
-------------------------------------------
1- we have added the params in config file

2- in StartBiuld() function, the modified platform.config structure get initialized by reading the params passed from the config file

3- in (*localhost).Configure function, the values are passed to the modified localhost structure

4- in blockchain.InitializeCentralBC, which is called in build.go, RunTest function (before starting the protocol yet!), “the config params that are needed for initializing the centralbc file” are passed to the blockchain.InitializeCentralBC function from (modified?) *platform.RunConfig structure

5- the function blockchain.InitializeCentralBC uses these params to initialize the centralbc file

6- in localhost.go, in function (*localhost).Start, “the config params that are needed inside the protocol” are passed to function Simulate() (function simulate has modified input params)

7- in function simulate(), after creating an instance of our BasedDFSProtocol, the passed params are sent to the protocol structure (our protocol has these params in its structure and get initialized here) 
