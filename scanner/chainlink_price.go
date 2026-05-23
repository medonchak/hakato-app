package main

import (
	"context"
	"errors"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

// -----------------------------------------------------------------------------
// MINIMAL ABI for Chainlink AggregatorV3Interface
// -----------------------------------------------------------------------------
const chainlinkAggregatorABIStable = `[
  {
    "inputs": [],
    "name": "decimals",
    "outputs": [{"internalType": "uint8","name": "","type": "uint8"}],
    "stateMutability": "view",
    "type": "function"
  },
  {
    "inputs": [],
    "name": "latestRoundData",
    "outputs": [
      {"internalType": "uint80","name": "roundId","type": "uint80"},
      {"internalType": "int256","name": "answer","type": "int256"},
      {"internalType": "uint256","name": "startedAt","type": "uint256"},
      {"internalType": "uint256","name": "updatedAt","type": "uint256"},
      {"internalType": "uint80","name": "answeredInRound","type": "uint80"}
    ],
    "stateMutability": "view",
    "type": "function"
  }
]`

func ReadChainlinkPrice(
	ctx context.Context,
	client *ethclient.Client,
	feed common.Address,
) (float64, error) {

	parsedABI, err := abi.JSON(strings.NewReader(chainlinkAggregatorABIStable))
	if err != nil {
		return 0, err
	}

	contract := bind.NewBoundContract(feed, parsedABI, client, client, client)

	opts := &bind.CallOpts{Context: ctx}

	// -------------------------------------------------------------------------
	// decimals()
	// -------------------------------------------------------------------------
	var decOut []any
	if err := contract.Call(opts, &decOut, "decimals"); err != nil {
		return 0, err
	}
	decimals := *abi.ConvertType(decOut[0], new(uint8)).(*uint8)

	// -------------------------------------------------------------------------
	// latestRoundData()
	// -------------------------------------------------------------------------
	var dataOut []any
	if err := contract.Call(opts, &dataOut, "latestRoundData"); err != nil {
		return 0, err
	}

	answer := *abi.ConvertType(dataOut[1], new(*big.Int)).(**big.Int)
	if answer.Sign() <= 0 {
		return 0, errors.New("invalid oracle answer")
	}

	// -------------------------------------------------------------------------
	// answer / 10^decimals
	// -------------------------------------------------------------------------
	price := new(big.Float).SetInt(answer)
	scale := new(big.Float).SetFloat64(math.Pow10(int(decimals)))
	price.Quo(price, scale)

	f, _ := price.Float64()
	return f, nil
}

// -----------------------------------------------------------------------------
// OPTIONAL example
// -----------------------------------------------------------------------------
// func example() error {
// 	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
// 	defer cancel()

// 	client, err := ethclient.Dial("https://eth.llamarpc.com")
// 	if err != nil {
// 		return err
// 	}
// 	defer client.Close()

// 	price, err := ReadChainlinkPrice(
// 		ctx,
// 		client,
// 		common.HexToAddress("0x3E7d1eAB13ad0104d2750B8863b489D65364e32D"),
// 	)
// 	if err != nil {
// 		return err
// 	}

// 	_ = price
// 	return nil
// }
