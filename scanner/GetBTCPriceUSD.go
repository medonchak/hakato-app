package main

import (
	"context"
	"errors"
	"log"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const chainlinkAggregatorABIBTC = `[
	{
		"inputs": [],
		"name": "latestRoundData",
		"outputs": [
			{"internalType":"uint80","name":"roundId","type":"uint80"},
			{"internalType":"int256","name":"answer","type":"int256"},
			{"internalType":"uint256","name":"startedAt","type":"uint256"},
			{"internalType":"uint256","name":"updatedAt","type":"uint256"},
			{"internalType":"uint80","name":"answeredInRound","type":"uint80"}
		],
		"stateMutability":"view",
		"type":"function"
	},
	{
		"inputs": [],
		"name": "decimals",
		"outputs": [
			{"internalType":"uint8","name":"","type":"uint8"}
		],
		"stateMutability":"view",
		"type":"function"
	}
]`

var btcUsdFeeds = map[int]common.Address{
	1: common.HexToAddress("0xF4030086522a5bEEa4988F8cA5B36dbC97BeE88c"), // ETH BTC/USD
}

func GetBTCPriceUSD(client *ethclient.Client) (float64, error) {
	chainID := 1 // BTC oracle завжди на Ethereum
	log.Printf("[price][BTC] start")

	feed, ok := btcUsdFeeds[chainID]
	if !ok {
		log.Printf("[price][BTC][ERR] feed not found for chain %d", chainID)
		return 0, errors.New("BTC/USD feed not found")
	}
	log.Printf("[price][BTC] feed=%s", feed.Hex())

	parsedABI, err := abi.JSON(strings.NewReader(chainlinkAggregatorABIBTC))
	if err != nil {
		log.Printf("[price][BTC][ERR] abi parse: %v", err)
		return 0, err
	}

	contract := bind.NewBoundContract(feed, parsedABI, client, client, client)
	ctxCall, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	opts := &bind.CallOpts{Context: ctxCall}

	// ---------- decimals() ----------
	var out []any
	if err := contract.Call(opts, &out, "decimals"); err != nil {
		log.Printf("[price][BTC][ERR] decimals call: %v", err)
		return 0, err
	}
	if len(out) == 0 {
		log.Printf("[price][BTC][ERR] decimals empty output")
		return 0, errors.New("decimals empty")
	}

	decimals, ok := out[0].(uint8)
	if !ok {
		log.Printf("[price][BTC][ERR] decimals type: %T", out[0])
		return 0, errors.New("invalid decimals type")
	}
	log.Printf("[price][BTC] decimals=%d", decimals)

	// ---------- latestRoundData() ----------
	out = nil
	if err := contract.Call(opts, &out, "latestRoundData"); err != nil {
		log.Printf("[price][BTC][ERR] roundData call: %v", err)
		return 0, err
	}
	if len(out) < 2 {
		log.Printf("[price][BTC][ERR] roundData output len=%d", len(out))
		return 0, errors.New("invalid roundData output")
	}

	answer, ok := out[1].(*big.Int)
	if !ok {
		log.Printf("[price][BTC][ERR] answer type: %T", out[1])
		return 0, errors.New("invalid answer type")
	}
	if answer.Sign() <= 0 {
		log.Printf("[price][BTC][ERR] answer <= 0: %v", answer)
		return 0, errors.New("invalid BTC oracle answer")
	}
	log.Printf("[price][BTC] raw answer=%s", answer.String())

	price := new(big.Float).SetInt(answer)
	div := new(big.Float).SetFloat64(math.Pow10(int(decimals)))
	price.Quo(price, div)

	f, _ := price.Float64()
	log.Printf("[price][BTC] final price=%.2f USD", f)

	return f, nil
}
