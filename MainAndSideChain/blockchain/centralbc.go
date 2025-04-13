package blockchain

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/rand"
	"strconv"
	"time"

	log "github.com/chainBoostScale/ChainBoost/onet/log"
)

type ExperimentConfig struct {
	FileSizeDistributionMean            float64
	FileSizeDistributionVariance        float64
	ServAgrDurationDistributionMean     float64
	ServAgrDurationDistributionVariance float64
	NegotiationDurationVariance         float64
	NegotiationDurationMean             float64
	SimulationSeed                      int
	Nodes                               int
	NumberOfActiveContractsPerServer    int
	SideChains                          []string
	FaultyContractsRate                 float64
}

// these values are written in the market matching sheet but since they are fixed
// we don't need to read them frequently from the xlsx file, instead we keep them in global variablles
// and use them when needed
//var FileSizeRow []string
//var ServAgrDurationRow []string

// generateNormalValues  generates values that follow a normal distribution with specified variance and mean
//
// FROM: https://go.dev/src/math/rand/normal.go
// NormFloat64 returns a normally distributed float64 in
// the range -math.MaxFloat64 through +math.MaxFloat64 inclusive,
// with standard normal distribution (mean = 0, stddev = 1).
// To produce a different normal distribution, callers can
// adjust the output using:
//
//	sample = NormFloat64() * desiredStdDev + desiredMean

/// stddev = sqrt(vairance)

func generateNormalValues(variance float64, mean float64, nodes int, SimulationSeed int) []interface{} {
	var list []interface{}

	rand.Seed(int64(SimulationSeed))

	for i := 1; i <= nodes; i++ {
		value := mean + variance*rand.NormFloat64()
		if value < 1 {
			value = 1
		}
		list = append(list, value)
	}
	return list
}

func flip(bias float64) bool {
	return rand.Float64() < bias
}

// InitializeMainChain function is called in simulation level
func InitializeMainChainBC(exconf ExperimentConfig) {
	// --------------------- generating normal distributed number based on config params ---------------------
	// --------------------- distribution of market matching information ---------------------

	exconf.Nodes = exconf.Nodes * exconf.NumberOfActiveContractsPerServer
	FileSizeRow := generateNormalValues(exconf.FileSizeDistributionVariance, exconf.FileSizeDistributionMean, exconf.Nodes, exconf.SimulationSeed)

	ServAgrDurationRow := generateNormalValues(exconf.ServAgrDurationDistributionVariance, exconf.ServAgrDurationDistributionMean, exconf.Nodes, exconf.SimulationSeed)

	rand.Seed(int64(exconf.SimulationSeed))
	NegotiationDurationRow := generateNormalValues(exconf.NegotiationDurationVariance, exconf.NegotiationDurationMean, exconf.Nodes, int(rand.Uint64()))

	//--------------------- fill the mainchainbc file with generated numbers  ---------------------
	var err error
	err = InitalizeMainChainDbTables()
	if err != nil {
		panic(err)
	}

	for i, _ := range FileSizeRow {
		bc := flip(0.7)
		err = InitialInsertValuesIntoMarketMatchingTable(int(FileSizeRow[i].(float64)), int(ServAgrDurationRow[i].(float64)), 0, i+1, bc, bc, int(NegotiationDurationRow[i].(float64)))
		if err != nil {
			panic(err)
		}
	}
	err = UpdateActiveFaultyContracts(exconf.FaultyContractsRate)

	SimulationSeedAsStr := strconv.Itoa(exconf.SimulationSeed)
	err = InsertIntoMainChainRoundTable(0, SimulationSeedAsStr, 0, "", 0, 0, 0, 0, 0, time.Now(), 0, 0, 0, false, false, false)
	if err != nil {
		panic(err)
	}

	data := fmt.Sprintf("%v", rand.Intn(exconf.SimulationSeed))

	sha := sha256.New()
	if _, err := sha.Write([]byte(data)); err != nil {
		log.Error("Couldn't hash header:", err)
	}
	hash := sha.Sum(nil)
	err = InsertIntoMainChainRoundTable(1, hex.EncodeToString(hash), 0, "", 0, 0, 0, 0, 0, time.Now(), 0, 0, 0, false, false, false)

	logMsg := fmt.Sprintf(
		`Config params used in initial initialization of main chain:
			 File Size Distribution Mean: " %f
			 File Size Distribution Variance: %f
			 ServAgr Duration Distribution Mean: %f
			 ServAgr Duration Distribution Variance: %f
		`, exconf.FileSizeDistributionMean, exconf.FileSizeDistributionVariance, exconf.ServAgrDurationDistributionMean,
		exconf.ServAgrDurationDistributionVariance)

	log.LLvl1(logMsg)
}

// InitializeSideChainBC function is called in simulation level (build.go)
func InitializeSideChainBC(names []string) {
	for _, name := range names {
		err := InitalizeSideChainDbTables(name)
		if err != nil {
			panic(err)
		}
	}
}
