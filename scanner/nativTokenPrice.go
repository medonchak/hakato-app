package main

import (
	"context"
	"errors"
	"math/big"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const chainlinkAggregatorABI = `[
  {
    "inputs": [],
    "name": "latestRoundData",
    "outputs": [
      {"name":"roundId","type":"uint80"},
      {"name":"answer","type":"int256"},
      {"name":"startedAt","type":"uint256"},
      {"name":"updatedAt","type":"uint256"},
      {"name":"answeredInRound","type":"uint80"}
    ],
    "stateMutability":"view",
    "type":"function"
  }
]`

var nativeOracle = map[int64]string{
	1:  "0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419", // ETH / USD
	56: "0x0567F2323251f0Aab15c8dFb1967E4e8A7D42aeE", // BNB / USD
}

func GetNativePriceUSD(chainID int64, client *ethclient.Client) (float64, error) {
	oracleAddr, ok := nativeOracle[chainID]
	if !ok {
		return 0, errors.New("unsupported chain_id")
	}

	parsedABI, err := abi.JSON(strings.NewReader(chainlinkAggregatorABI))
	if err != nil {
		return 0, err
	}

	contract := bind.NewBoundContract(
		common.HexToAddress(oracleAddr),
		parsedABI,
		client,
		client,
		client,
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 🔴 ВАЖЛИВО: приймаємо як slice interface{}
	var result []interface{}

	if err := contract.Call(
		&bind.CallOpts{Context: ctx},
		&result,
		"latestRoundData",
	); err != nil {
		return 0, err
	}

	// result[1] = answer (int256)
	answer, ok := result[1].(*big.Int)
	if !ok || answer.Sign() <= 0 {
		return 0, errors.New("invalid oracle price")
	}

	// Chainlink price має 8 decimals
	price := new(big.Float).Quo(
		new(big.Float).SetInt(answer),
		big.NewFloat(1e8),
	)

	val, _ := price.Float64()
	return val, nil
}

/*
ПРИКЛАД ВИКЛИКУ:

priceETH, _ := GetNativePriceUSD(
    1,
    "https://eth.llamarpc.com",
)

priceBNB, _ := GetNativePriceUSD(
    56,
    "https://bsc-dataseed.binance.org",
)
*/
