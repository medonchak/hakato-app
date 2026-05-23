package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

var (
	WETH               = common.HexToAddress("0xC02aaA39b223FE8D0A0e5C4F27eAD9083C756Cc2")
	UNISWAP_V3_FACTORY = common.HexToAddress("0x1F98431c8aD98523631AE4a59f267346ea31F984")
	CHAINLINK_ETH_USD  = common.HexToAddress("0x5f4eC3Df9cbd43714FE2740f5E3616155c5b8419")

	UNISWAP_V2_FACTORY   = common.HexToAddress("0x5C69bEe701ef814a2B6a3EDD4B1652CB9cc5aA6f")
	SUSHISWAP_FACTORY    = common.HexToAddress("0xc35DADB65012eC5796536bD9864eD8773aBc74C4")
	UNISWAP_V2_INIT_HASH = "96e8ac4277198ff8b6f785478aa9a39f403cb768dd02cbee326c3e7da348845f"
	SUSHISWAP_INIT_HASH  = "e18a34eb0e04b04f7a0ac29a6e80748dca96319b42c54d679cb821dca90c6303"
)

var (
	factoryV3ABI, _ = abi.JSON(strings.NewReader(`[{"inputs":[{"internalType":"address","name":"tokenA","type":"address"},{"internalType":"address","name":"tokenB","type":"address"},{"internalType":"uint24","name":"fee","type":"uint24"}],"name":"getPool","outputs":[{"internalType":"address","name":"pool","type":"address"}],"stateMutability":"view","type":"function"}]`))

	poolV3ABI, _ = abi.JSON(strings.NewReader(`[{"inputs":[],"name":"slot0","outputs":[{"internalType":"uint160","name":"sqrtPriceX96","type":"uint160"},{"internalType":"int24","name":"tick","type":"int24"},{"internalType":"uint16","name":"observationIndex","type":"uint16"},{"internalType":"uint16","name":"observationCardinality","type":"uint16"},{"internalType":"uint16","name":"observationCardinalityNext","type":"uint16"},{"internalType":"uint8","name":"feeProtocol","type":"uint8"},{"internalType":"bool","name":"unlocked","type":"bool"}],"stateMutability":"view","type":"function"}]`))

	pairV2ABI, _ = abi.JSON(strings.NewReader(`[
		{"inputs":[],"name":"getReserves","outputs":[{"internalType":"uint112","name":"reserve0","type":"uint112"},{"internalType":"uint112","name":"reserve1","type":"uint112"},{"internalType":"uint32","name":"blockTimestampLast","type":"uint32"}],"stateMutability":"view","type":"function"},
		{"inputs":[],"name":"token0","outputs":[{"internalType":"address","name":"","type":"address"}],"stateMutability":"view","type":"function"}
	]`))

	chainlinkABI, _ = abi.JSON(strings.NewReader(`[{"inputs":[],"name":"latestRoundData","outputs":[{"internalType":"uint80","name":"roundId","type":"uint80"},{"internalType":"int256","name":"answer","type":"int256"},{"internalType":"uint256","name":"startedAt","type":"uint256"},{"internalType":"uint256","name":"updatedAt","type":"uint256"},{"internalType":"uint80","name":"answeredInRound","type":"uint80"}],"stateMutability":"view","type":"function"}]`))
)

var ErrNoLiquidity = errors.New("no active liquidity")

func GetTokenPriceUS(tokenAddr string) (float64, error) {
	if tokenAddr == "" {
		return 0, ErrNoLiquidity
	}

	client, err := ethclient.Dial("https://eth.llamarpc.com")
	if err != nil {
		return 0, err
	}
	defer client.Close()

	token := common.HexToAddress(tokenAddr)

	// ---------- 1️⃣ UNISWAP V3 ----------
	if price, err := safePriceFromV3(client, token); err == nil && price > 0 {
		return price, nil
	}

	// ---------- 2️⃣ UNISWAP V2 ----------
	if price, err := safePriceFromV2(client, token); err == nil && price > 0 {
		return price, nil
	}

	// ---------- 3️⃣ НОРМАЛЬНИЙ СТАН ----------
	return 0, ErrNoLiquidity
}

// ======================== Uniswap V3 ========================
func priceFromUniswapV3(client *ethclient.Client, token common.Address) (float64, error) {
	fees := []uint32{500, 3000, 10000, 100} // 0.05%, 0.3%, 1%, 0.01%

	for _, fee := range fees {
		{
			poolAddr, err := getV3PoolAddress(client, token, WETH, fee)
			if err != nil || poolAddr == (common.Address{}) {
				continue
			}

			sqrtPriceX96, err := getSqrtPriceX96(client, poolAddr)
			if err != nil || sqrtPriceX96.Sign() == 0 {
				continue
			}

			priceETH := sqrtPriceToPrice(sqrtPriceX96, token, WETH)
			ethUSD, _ := getETHPriceUSD(client)
			return priceETH * ethUSD, nil
		}
	}
	return 0, fmt.Errorf("uniswap v3 pool not found")
}

func getV3PoolAddress(client *ethclient.Client, tokenA, tokenB common.Address, fee uint32) (common.Address, error) {
	data, _ := factoryV3ABI.Pack("getPool", tokenA, tokenB, fee)
	msg := ethereum.CallMsg{To: &UNISWAP_V3_FACTORY, Data: data}
	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil || len(result) == 0 {
		return common.Address{}, err
	}
	return common.BytesToAddress(result[12:]), nil // padded address
}

func getSqrtPriceX96(client *ethclient.Client, pool common.Address) (*big.Int, error) {
	data, _ := poolV3ABI.Pack("slot0")
	msg := ethereum.CallMsg{To: &pool, Data: data}
	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil || len(result) < 32 {
		return nil, err
	}
	return new(big.Int).SetBytes(result[:32]), nil
}

func sqrtPriceToPrice(sqrt *big.Int, token, weth common.Address) float64 {
	p := new(big.Float).SetInt(sqrt)
	p.Mul(p, p)
	p.Quo(p, new(big.Float).SetFloat64(math.Pow(2, 192)))

	f, _ := p.Float64()

	// Якщо токен іде першим у парі — інвертуємо
	if token.Hex() < weth.Hex() {
		f = 1 / f
	}
	return f
}

// ======================== V2 (Uniswap + Sushi) ========================
func priceFromV2(client *ethclient.Client, token common.Address) (float64, error) {
	type factoryInfo struct {
		addr     common.Address
		initHash string
	}
	factories := []factoryInfo{
		{UNISWAP_V2_FACTORY, UNISWAP_V2_INIT_HASH},
		{SUSHISWAP_FACTORY, SUSHISWAP_INIT_HASH},
	}

	for _, f := range factories {
		pair := computePairAddress(token, WETH, f.addr, f.initHash)

		reserves, err := getReservesV2(client, pair)
		if err != nil || reserves[0].Sign() == 0 || reserves[1].Sign() == 0 {
			continue
		}

		token0, _ := getToken0V2(client, pair)
		var tokenReserve, wethReserve *big.Int
		if token0 == token {
			tokenReserve = reserves[0]
			wethReserve = reserves[1]
		} else {
			tokenReserve = reserves[1]
			wethReserve = reserves[0]
		}

		priceETH := new(big.Float).Quo(new(big.Float).SetInt(wethReserve), new(big.Float).SetInt(tokenReserve))
		ethUSD, _ := getETHPriceUSD(client)
		p, _ := priceETH.Float64()
		return p * ethUSD, nil
	}
	return 0, fmt.Errorf("v2 pool not found")
}

func computePairAddress(tokenA, tokenB, factory common.Address, initCodeHash string) common.Address {
	if tokenA.Hex() > tokenB.Hex() {
		tokenA, tokenB = tokenB, tokenA
	}
	salt := crypto.Keccak256Hash(tokenA.Bytes(), tokenB.Bytes())
	codeHash := common.Hex2Bytes(initCodeHash)
	return crypto.CreateAddress2(factory, salt, codeHash)
}

func getReservesV2(client *ethclient.Client, pair common.Address) ([2]*big.Int, error) {
	data, _ := pairV2ABI.Pack("getReserves")
	msg := ethereum.CallMsg{To: &pair, Data: data}
	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil || len(result) < 64 {
		return [2]*big.Int{}, err
	}
	return [2]*big.Int{
		new(big.Int).SetBytes(result[:32]),
		new(big.Int).SetBytes(result[32:64]),
	}, nil
}

func getToken0V2(client *ethclient.Client, pair common.Address) (common.Address, error) {
	data, _ := pairV2ABI.Pack("token0")
	msg := ethereum.CallMsg{To: &pair, Data: data}
	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil || len(result) == 0 {
		return common.Address{}, err
	}
	return common.BytesToAddress(result), nil
}

// ======================== Chainlink ETH/USD ========================
func getETHPriceUSD(client *ethclient.Client) (float64, error) {
	data, _ := chainlinkABI.Pack("latestRoundData")
	msg := ethereum.CallMsg{To: &CHAINLINK_ETH_USD, Data: data}
	result, err := client.CallContract(context.Background(), msg, nil)
	if err != nil || len(result) < 160 {
		return 2600, nil // fallback
	}
	answer := new(big.Int).SetBytes(result[32:64]) // int256 answer
	if answer.Sign() < 0 {
		answer = new(big.Int).Neg(answer)
	}
	price := new(big.Float).SetInt(answer)
	price.Quo(price, big.NewFloat(1e8))
	f, _ := price.Float64()
	return f, nil
}
func safePriceFromV2(client *ethclient.Client, token common.Address) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			// ❗ panic тут НЕДОПУСТИМИЙ
		}
	}()

	price, err := priceFromV2(client, token)
	if err != nil || price <= 0 {
		return 0, ErrNoLiquidity
	}

	return price, nil
}
func safePriceFromV3(client *ethclient.Client, token common.Address) (float64, error) {
	defer func() {
		if r := recover(); r != nil {
			// ❗ panic тут НЕДОПУСТИМИЙ
		}
	}()

	price, err := priceFromUniswapV3(client, token)
	if err != nil || price <= 0 {
		return 0, ErrNoLiquidity
	}

	return price, nil
}
