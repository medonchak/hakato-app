package main

import (
	"context"
	"database/sql"
	"encoding/hex"
	"errors"
	"math"
	"math/big"
	"strconv"
	"strings"
	"sync"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
)

// -------------------- CACHES --------------------

var (
	tokenDecimalsCacheMu sync.RWMutex
	tokenDecimalsCache   = map[string]uint8{}

	tokenSymbolCacheMu sync.RWMutex
	tokenSymbolCache   = map[string]string{}
)

// -------------------- TYPES --------------------

// SwapRule — у тебе вже є; тут лише нагадування як використовується.
// type SwapRule struct {
// 	Enabled   bool
// 	Tokens    []string // token contract addresses
// 	MinAmount string   // min USD (string float), e.g. "1000"
// }

type TransferDirection string

const (
	DirOut   TransferDirection = "OUT"   // токени пішли від sender tx
	DirIn    TransferDirection = "IN"    // токени прийшли на sender tx
	DirMixed TransferDirection = "MIXED" // і туди і назад (часто при складних роутерах)
	DirNone  TransferDirection = "NONE"
)

type TokenMoveInfo struct {
	Token         common.Address      `json:"token"`
	Symbol        string              `json:"symbol"`
	Decimals      uint8               `json:"decimals"`
	RawAmount     *big.Int            `json:"rawAmount"`
	HumanAmount   string              `json:"humanAmount"` // string, щоб не губити точність
	PriceUSD      string              `json:"priceUsd"`    // string
	ValueUSD      string              `json:"valueUsd"`    // string
	Direction     TransferDirection   `json:"direction"`
	TransfersSeen int                 `json:"transfersSeen"`
	FromAgg       map[string]*big.Int `json:"fromAgg"` // from -> sum raw
	ToAgg         map[string]*big.Int `json:"toAgg"`   // to -> sum raw
}

type TxExtraInfo struct {
	TxHash            common.Hash     `json:"txHash"`
	From              common.Address  `json:"from"`
	To                *common.Address `json:"to,omitempty"`
	Nonce             uint64          `json:"nonce"`
	ValueWei          *big.Int        `json:"valueWei"`
	GasLimit          uint64          `json:"gasLimit"`
	GasUsed           uint64          `json:"gasUsed"`
	EffectiveGasPrice *big.Int        `json:"effectiveGasPrice"`
	GasCostWei        *big.Int        `json:"gasCostWei"`
	Status            uint64          `json:"status"` // 1 success, 0 fail
	BlockNumber       *big.Int        `json:"blockNumber,omitempty"`
}

type TxCheckStatus int

const (
	TxDone TxCheckStatus = iota
	TxPending
)

type CheckTransfersResult struct {
	Tx     TxExtraInfo
	Moves  []TokenMoveInfo
	Status TxCheckStatus
}

// -------------------- MAIN FUNCTION --------------------

// CheckTokenTransfersInTxMax — “допустимий максимум” без trace.
// Повертає максимум корисної інфи з receipt + tx + logs.
func CheckTokenTransfersInTxMax(
	ctx context.Context,
	client *ethclient.Client,
	txHash common.Hash,
	swap *SwapRule,
) (*CheckTransfersResult, error) {

	// 0) швидкі перевірки правил
	if swap == nil || !swap.Enabled || len(swap.Tokens) == 0 {
		return &CheckTransfersResult{}, nil
	}

	normalizeHex := func(s string) string {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			return ""
		}
		if !strings.HasPrefix(s, "0x") {
			s = "0x" + s
		}
		return s
	}

	// 1) завантажуємо tx (для From/To/Nonce/Gas/Value) + chainID (для signer)
	tx, isPending, err := client.TransactionByHash(ctx, txHash)
	if err != nil {
		return nil, err
	}
	if isPending {
		return nil, errors.New("tx is pending (no receipt yet)")
	}

	chainID, err := client.NetworkID(ctx)
	if err != nil {
		return nil, err
	}

	msgFrom, err := txSender(chainID, tx)
	if err != nil {
		return nil, err
	}

	// 2) receipt (gasUsed/status/logs/EffectiveGasPrice)
	receipt, err := client.TransactionReceipt(ctx, txHash)
	if err != nil {
		return nil, err
	}

	// 3) сформуємо targets: тільки ті токени, які у rule
	targets := make(map[string]struct{}, len(swap.Tokens))
	for _, t := range swap.Tokens {
		addr := normalizeHex(t)
		if addr != "" && addr != "0x" {
			targets[addr] = struct{}{}
		}
	}

	// 4) userMinAmount (USD)
	userMinUSD := 0.0
	if s := strings.TrimSpace(swap.MinAmount); s != "" {
		userMinUSD, _ = strconv.ParseFloat(s, 64)
	}

	// 5) парсимо logs Transfer
	transferTopic := common.HexToHash("0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef")

	// movedRaw[token] = сумарний raw
	movedRaw := map[string]*big.Int{}

	// directionFlags[token] -> {in/out} відносно sender tx
	type dirFlags struct{ in, out bool }
	dirByToken := map[string]*dirFlags{}

	// агрегуємо from/to по transfer-ах
	fromAgg := map[string]map[string]*big.Int{} // token -> from -> sum
	toAgg := map[string]map[string]*big.Int{}   // token -> to -> sum
	transfersSeen := map[string]int{}           // token -> count

	for _, lg := range receipt.Logs {
		// ERC20 Transfer: topics[0]=Transfer, topics[1]=from, topics[2]=to, data=value
		if len(lg.Topics) < 3 || lg.Topics[0] != transferTopic {
			continue
		}

		tokenAddrLower := normalizeHex(lg.Address.Hex())
		if _, ok := targets[tokenAddrLower]; !ok {
			continue
		}

		// value = uint256 in Data (32 bytes). Якщо data пусте — скіпаємо.
		if len(lg.Data) == 0 {
			continue
		}
		val := new(big.Int).SetBytes(lg.Data)

		// сумуємо raw
		if movedRaw[tokenAddrLower] == nil {
			movedRaw[tokenAddrLower] = new(big.Int)
		}
		movedRaw[tokenAddrLower].Add(movedRaw[tokenAddrLower], val)

		// from/to з topics (останні 20 байт)
		fromAddr := topicToAddress(lg.Topics[1]).Hex()
		toAddr := topicToAddress(lg.Topics[2]).Hex()

		// агрегація
		if fromAgg[tokenAddrLower] == nil {
			fromAgg[tokenAddrLower] = map[string]*big.Int{}
		}
		if fromAgg[tokenAddrLower][fromAddr] == nil {
			fromAgg[tokenAddrLower][fromAddr] = new(big.Int)
		}
		fromAgg[tokenAddrLower][fromAddr].Add(fromAgg[tokenAddrLower][fromAddr], val)

		if toAgg[tokenAddrLower] == nil {
			toAgg[tokenAddrLower] = map[string]*big.Int{}
		}
		if toAgg[tokenAddrLower][toAddr] == nil {
			toAgg[tokenAddrLower][toAddr] = new(big.Int)
		}
		toAgg[tokenAddrLower][toAddr].Add(toAgg[tokenAddrLower][toAddr], val)

		transfersSeen[tokenAddrLower]++

		// визначаємо напрямок відносно sender tx
		if dirByToken[tokenAddrLower] == nil {
			dirByToken[tokenAddrLower] = &dirFlags{}
		}
		if strings.EqualFold(fromAddr, msgFrom.Hex()) {
			dirByToken[tokenAddrLower].out = true
		}
		if strings.EqualFold(toAddr, msgFrom.Hex()) {
			dirByToken[tokenAddrLower].in = true
		}
	}

	// 6) зберемо TxExtraInfo
	var toPtr *common.Address
	if tx.To() != nil {
		tmp := *tx.To()
		toPtr = &tmp
	}

	// effectiveGasPrice (EIP-1559)
	eff := receipt.EffectiveGasPrice
	if eff == nil {
		eff = tx.GasPrice()
		if eff == nil {
			eff = big.NewInt(0)
		}
	}

	gasCostWei := new(big.Int).Mul(new(big.Int).SetUint64(receipt.GasUsed), eff)

	res := &CheckTransfersResult{
		Tx: TxExtraInfo{
			TxHash:            txHash,
			From:              msgFrom,
			To:                toPtr,
			Nonce:             tx.Nonce(),
			ValueWei:          tx.Value(),
			GasLimit:          tx.Gas(),
			GasUsed:           receipt.GasUsed,
			EffectiveGasPrice: eff,
			GasCostWei:        gasCostWei,
			Status:            receipt.Status,
			BlockNumber:       receipt.BlockNumber,
		},
		Moves:  []TokenMoveInfo{},
		Status: TxDone, // ✅ дефолт
	}

	// якщо немає переміщень — одразу
	if len(movedRaw) == 0 {
		return res, nil
	}

	// 7) stable tokens: ціна = 1 (щоб не чекати БД)
	stableUSD := map[string]float64{
		"0xdac17f958d2ee523a2206206994597c13d831ec7": 1.0, // USDT
		"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": 1.0, // USDC
		"0x6b175474e89094c44da98b954eedeac495271d0f": 1.0, // DAI
	}

	for tokenLower, totalRaw := range movedRaw {
		tokenAddr := common.HexToAddress(tokenLower)

		decimals, derr := GetTokenDecimalsSafe(ctx, client, tokenAddr)
		if derr != nil {
			continue
		}

		// symbol (може бути пустий, але це не причина pending)
		symbol, _ := GetTokenSymbolSafe(ctx, client, tokenAddr)
		humanAmountStr := rawToHumanString(totalRaw, decimals)

		// USD price: або stable=1, або БД через EnsureTokenPrice
		priceUSD := 0.0
		if v, ok := stableUSD[tokenLower]; ok {
			priceUSD = v
		} else {
			tokenPriceID, e := EnsureTokenPrice(int(chainID.Int64()), tokenLower, ctx)
			if e != nil {
				// не ламаємо всю tx — просто не можемо оцінити USD
				res.Status = TxPending
				continue
			}

			var p sql.NullFloat64
			_ = DB.QueryRow(`SELECT price_usd FROM token_prices WHERE id = ?`, tokenPriceID).Scan(&p)

			if !p.Valid || p.Float64 <= 0 {
				// ✅ ключове: swap є, але ціни ще нема → pending
				res.Status = TxPending
				continue
			}

			priceUSD = p.Float64
		}

		// valueUSD = humanAmount * priceUSD
		humanAmountFloat := humanToFloat(totalRaw, decimals)
		valueUSD := humanAmountFloat * priceUSD

		// мінімалка в USD
		if userMinUSD > 0 && valueUSD < userMinUSD {
			continue
		}

		// direction
		dir := DirNone
		if f := dirByToken[tokenLower]; f != nil {
			if f.in && !f.out {
				dir = DirIn
			} else if f.out && !f.in {
				dir = DirOut
			} else if f.in && f.out {
				dir = DirOut
			}
		}

		move := TokenMoveInfo{
			Token:         tokenAddr,
			Symbol:        symbol,
			Decimals:      decimals,
			RawAmount:     totalRaw,
			HumanAmount:   humanAmountStr,
			PriceUSD:      floatToMoneyString(priceUSD),
			ValueUSD:      floatToMoneyString(valueUSD),
			Direction:     dir,
			TransfersSeen: transfersSeen[tokenLower],
			FromAgg:       fromAgg[tokenLower],
			ToAgg:         toAgg[tokenLower],
		}

		res.Moves = append(res.Moves, move)
	}

	return res, nil
}

// -------------------- HELPERS --------------------

// txSender дістає sender з tx (EIP-1559/legacy)
func txSender(chainID *big.Int, tx *types.Transaction) (common.Address, error) {
	signer := types.LatestSignerForChainID(chainID)
	from, err := types.Sender(signer, tx)
	if err != nil {
		return common.Address{}, err
	}
	return from, nil
}

func topicToAddress(t common.Hash) common.Address {
	// address у topics закодований як 32 bytes, потрібні останні 20
	return common.BytesToAddress(t.Bytes()[12:])
}

func rawToHumanString(raw *big.Int, decimals uint8) string {
	if raw == nil {
		return "0"
	}
	// string без float-помилок
	// raw / 10^decimals
	den := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil)
	intPart := new(big.Int).Div(raw, den)
	fracPart := new(big.Int).Mod(raw, den)

	if decimals == 0 {
		return intPart.String()
	}

	fracStr := fracPart.String()
	// доповнюємо нулями зліва до decimals
	if len(fracStr) < int(decimals) {
		fracStr = strings.Repeat("0", int(decimals)-len(fracStr)) + fracStr
	}
	// прибираємо зайві нулі справа
	fracStr = strings.TrimRight(fracStr, "0")
	if fracStr == "" {
		return intPart.String()
	}
	return intPart.String() + "." + fracStr
}

func humanToFloat(raw *big.Int, decimals uint8) float64 {
	if raw == nil {
		return 0
	}
	// це float64, може округлятися, але для порівняння/логів ок.
	f := new(big.Float).SetInt(raw)
	den := new(big.Float).SetFloat64(math.Pow10(int(decimals)))
	f.Quo(f, den)
	v, _ := f.Float64()
	return v
}

func floatToMoneyString(v float64) string {
	// проста фіксація під лог (не для фінансового обліку)
	// 6 знаків після коми — норм для USD в логах
	return strconv.FormatFloat(v, 'f', 6, 64)
}

// -------------------- DECIMALS / SYMBOL --------------------

// GetTokenDecimalsSafe — читає decimals і кешує.
// Якщо не вийшло — повертає error (не робимо брехню з 18).
func GetTokenDecimalsSafe(ctx context.Context, client *ethclient.Client, token common.Address) (uint8, error) {
	addr := strings.ToLower(token.Hex())

	tokenDecimalsCacheMu.RLock()
	if d, ok := tokenDecimalsCache[addr]; ok {
		tokenDecimalsCacheMu.RUnlock()
		return d, nil
	}
	tokenDecimalsCacheMu.RUnlock()

	erc20Abi, err := abi.JSON(strings.NewReader(
		`[{"constant":true,"inputs":[],"name":"decimals","outputs":[{"name":"","type":"uint8"}],"type":"function"}]`,
	))
	if err != nil {
		return 0, err
	}

	data, err := erc20Abi.Pack("decimals")
	if err != nil {
		return 0, err
	}

	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err != nil || len(res) == 0 {
		return 0, errors.New("decimals() call failed")
	}

	vals, err := erc20Abi.Unpack("decimals", res)
	if err != nil || len(vals) == 0 {
		return 0, errors.New("decimals() unpack failed")
	}

	// go-ethereum може повернути uint8 або *big.Int в деяких кейсах, страхуємось:
	var dec uint8
	switch v := vals[0].(type) {
	case uint8:
		dec = v
	case *big.Int:
		dec = uint8(v.Uint64())
	default:
		return 0, errors.New("decimals() unexpected type")
	}

	tokenDecimalsCacheMu.Lock()
	tokenDecimalsCache[addr] = dec
	tokenDecimalsCacheMu.Unlock()

	return dec, nil
}

// GetTokenSymbolSafe — читає symbol() (string або bytes32 у деяких токенів), кешує.
func GetTokenSymbolSafe(ctx context.Context, client *ethclient.Client, token common.Address) (string, error) {
	addr := strings.ToLower(token.Hex())

	// ===== CACHE =====
	tokenSymbolCacheMu.RLock()
	if s, ok := tokenSymbolCache[addr]; ok {
		tokenSymbolCacheMu.RUnlock()
		return s, nil
	}
	tokenSymbolCacheMu.RUnlock()

	// ===== ABI =====
	abiStr, err := abi.JSON(strings.NewReader(
		`[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"type":"function"}]`,
	))
	if err != nil {
		return "", err
	}

	data, err := abiStr.Pack("symbol")
	if err != nil {
		return "", err
	}

	// ===== CALL =====
	res, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data}, nil)
	if err == nil && len(res) > 0 {

		vals, err := abiStr.Unpack("symbol", res)
		if err == nil && len(vals) > 0 {
			if s, ok := vals[0].(string); ok && s != "" {
				tokenSymbolCacheMu.Lock()
				tokenSymbolCache[addr] = s
				tokenSymbolCacheMu.Unlock()
				return s, nil
			}
		}
	}

	// ===== FALLBACK: bytes32 =====
	abiB32, err := abi.JSON(strings.NewReader(
		`[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"bytes32"}],"type":"function"}]`,
	))
	if err != nil {
		return "", err
	}

	data2, err := abiB32.Pack("symbol")
	if err != nil {
		return "", err
	}

	res2, err := client.CallContract(ctx, ethereum.CallMsg{To: &token, Data: data2}, nil)
	if err != nil || len(res2) == 0 {
		return "", errors.New("symbol() call failed")
	}

	vals2, err := abiB32.Unpack("symbol", res2)
	if err != nil || len(vals2) == 0 {
		return "", errors.New("symbol() unpack failed")
	}

	b32, ok := vals2[0].([32]byte)
	if !ok {
		return "", errors.New("symbol() bad type")
	}

	s := strings.TrimRight(string(b32[:]), "\x00")
	if s == "" {
		// fallback на адресу
		s = "0x" + strings.ToUpper(hex.EncodeToString(token.Bytes()[:2]))
	}

	tokenSymbolCacheMu.Lock()
	tokenSymbolCache[addr] = s
	tokenSymbolCacheMu.Unlock()

	return s, nil
}

// -------------------- PRICE (UNISWAP V2) --------------------

// GetTokenPriceUSD_UniswapV2 повертає ціну 1 токена у USD (через USDC), як float64.
// amountIn = 10^decimals (реально 1 токен).
func GetTokenPriceUSD_UniswapV2(ctx context.Context, client *ethclient.Client, token common.Address, decimals uint8) (float64, error) {

	// Uniswap V2 Router (Ethereum mainnet)
	routerAddr := common.HexToAddress("0x7a250d5630b4cf539739df2c5dacb4c659f2488d")
	weth := common.HexToAddress("0xC02aaA39b223FE8D0A0E5C4F27eAD9083C756Cc2")
	usdc := common.HexToAddress("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606eB48")

	abiJSON := `[{"constant":true,"inputs":[{"name":"amountIn","type":"uint256"},{"name":"path","type":"address[]"}],"name":"getAmountsOut","outputs":[{"name":"","type":"uint256[]"}],"type":"function"}]`
	routerAbi, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return 0, err
	}

	contract := bind.NewBoundContract(routerAddr, routerAbi, client, client, client)

	amountIn := new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(decimals)), nil) // 1 token

	path := []common.Address{token, weth, usdc}

	var out []any
	// НІ: contract.Call(nil, ...) — в деяких версіях може без контексту.
	// ТАК: використовуємо bind.CallOpts з контекстом.
	callOpts := &bind.CallOpts{Context: ctx}

	if err := contract.Call(callOpts, &out, "getAmountsOut", amountIn, path); err != nil {
		return 0, err
	}

	outs, ok := out[0].([]*big.Int)
	if !ok || len(outs) == 0 {
		return 0, errors.New("bad getAmountsOut output")
	}

	usdcOut := outs[len(outs)-1] // USDC raw (6 decimals)

	// priceUSD = usdcOut / 1e6
	price := new(big.Float).SetInt(usdcOut)
	price.Quo(price, new(big.Float).SetFloat64(1e6))

	v, _ := price.Float64()
	if v <= 0 {
		return 0, errors.New("price <= 0")
	}

	return v, nil
}

// -------------------- OPTIONAL: SIMPLE TIMEOUT WRAPPER --------------------

// helper якщо ти хочеш завжди мати таймаут усередині (але ти вже робиш ctx з таймаутом зовні)
// func withTimeout(parent context.Context, d time.Duration) (context.Context, context.CancelFunc) {
// 	return context.WithTimeout(parent, d)
// }

// -------------------- OPTIONAL: SMALL UTILS --------------------

// якщо треба bytes32 -> hex debug
// func bytesToHex(b []byte) string {
// 	return hex.EncodeToString(b)
// }
