package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"math/big"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

func GetTransactionStatsAndMatchesJSON_Rules(
	txs []TxJSON,
	rules []AlertRule,
	resultAlerts []DBAlertFilter,
	client *ethclient.Client,
) (TxStats, []MatchedTxInfo) {
	transferTopic := common.HexToHash(
		"0xddf252ad1be2c89b69c2b068fc378daa952ba7f163c4a11628f55a4df523b3ef",
	)
	_ = rules // параметр не використовується свідомо

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

	// ETH string ("0.01") -> wei
	ethToWei := func(s string) (*big.Int, bool) {
		return ethStringToWei(strings.TrimSpace(s))
	}

	// decimal / hex ("123", "0x1a") -> big.Int
	parseInt := func(s string) (*big.Int, bool) {
		s = strings.TrimSpace(s)
		if s == "" {
			return nil, false
		}
		v, ok := new(big.Int).SetString(s, 0)
		return v, ok
	}

	stats := TxStats{
		Total:       len(txs),
		TotalGas:    big.NewInt(0),
		MaxTotalGas: big.NewInt(0),
	}

	matches := make([]MatchedTxInfo, 0, 64)

	// ruleID|txHash|tag|token
	emitted := make(map[string]struct{}, 256)

TX_LOOP:
	for _, tx := range txs {

		from := normalizeHex(tx.From)
		to := normalizeHex(tx.To)

		// ==============================
		// 🔒 ГЛОБАЛЬНИЙ ФІЛЬТР ПО ГАМАНЦЮ
		// ==============================
		hasAddressRule := false
		for _, r := range resultAlerts {
			addr := normalizeHex(r.Filter.Address)
			if addr == "" || addr == "0x" {
				continue
			}
			hasAddressRule = true
			if from == addr || to == addr {
				goto TX_RELEVANT
			}
		}
		if hasAddressRule {
			continue TX_LOOP
		}

	TX_RELEVANT:

		// ========= статистика =========
		if strings.TrimSpace(tx.To) == "" {
			stats.ContractCreations++
		} else {
			stats.EOAToEOA++ // фактично "non-creation"
		}

		gpWei, gpOk := parseInt(tx.GasPrice)
		if gpOk {
			cost := new(big.Int).Mul(gpWei, new(big.Int).SetUint64(tx.Gas))
			stats.TotalGas.Add(stats.TotalGas, cost)
			if cost.Cmp(stats.MaxTotalGas) > 0 {
				stats.MaxTotalGas.Set(cost)
			}
		}

		valWei, ok := ethToWei(tx.Value)
		if !ok {
			valWei = big.NewInt(0)
		}

		var gpGwei *big.Int
		if gpOk {
			gpGwei = new(big.Int).Quo(gpWei, big.NewInt(1_000_000_000))
		}

		// ========= правила =========
		for _, r := range resultAlerts {

			if r.Filter.ID == "" {
				continue
			}

			addr := normalizeHex(r.Filter.Address)

			emitKey := func(tag, token string) string {
				if token != "" {
					return fmt.Sprintf("%d|%s|%s|%s", r.RuleID, tx.Hash, tag, token)
				}
				return fmt.Sprintf("%d|%s|%s", r.RuleID, tx.Hash, tag)
			}

			// ----- ANOMALIES -----
			if r.Filter.Anomalies != nil && r.Filter.Anomalies.Enabled {
				if minStr := strings.TrimSpace(r.Filter.Anomalies.MinGasPriceGwei); minStr != "" && gpGwei != nil {
					if min, ok := new(big.Int).SetString(minStr, 10); ok && gpGwei.Cmp(min) >= 0 {
						k := emitKey("anomaly_gas", "")
						if _, seen := emitted[k]; !seen {
							matches = append(matches, MatchedTxInfo{
								TxHash:   tx.Hash,
								From:     from,
								To:       to,
								ValueEth: tx.Value,
								GasUsed:  0,
								GasPrice: tx.GasPrice,
								Nonce:    tx.Nonce,
								RuleID:   r.RuleID,
								UserID:   r.UserID,
								RuleTag:  "anomaly_gas",
							})
							emitted[k] = struct{}{}
						}
					}
				}
			}

			// ----- ETH TRANSFER -----
			// ----- VALUE SWAP (native / stable, BUY / SELL) -----
			// ❗ заміна ETH TRANSFER, CheckTokenTransfersInTxMax НЕ використовується

			// ----- VALUE SWAP (native / stable, BUY / SELL) -----
			// ✔ заміна ETH TRANSFER
			// ✔ використовує ТІЛЬКИ наявні у тебе функції / підходи
			// ✔ без CheckTokenTransfersInTxMax
			// ✔ без вигаданих helper-ів

			// ----- SWAP FINANCE (native / stable value movements) -----
			// ----- SWAP FINANCE (native / stable value movements) -----
			// ----- SWAP FINANCE (native / stable value movements) -----
			if r.Filter.SwapFinance != nil && r.Filter.SwapFinance.Enabled {

				log.Printf("[SWAP_FINANCE][START] tx=%s rule=%d addr=%s",
					tx.Hash, r.RuleID, addr,
				)

				ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
				defer cancel()

				chainIDBig, err := client.NetworkID(ctx)
				if err != nil {
					log.Printf("[SWAP_FINANCE][ERR] network_id err=%v", err)
					continue
				}
				chainID := int(chainIDBig.Int64())

				nativeSymbol := "UNKNOWN"
				switch chainID {
				case 1:
					nativeSymbol = "ETH"
				case 56:
					nativeSymbol = "BNB"
				case 137:
					nativeSymbol = "MATIC"
				}

				minUSD := 0.0
				if s := strings.TrimSpace(r.Filter.SwapFinance.MinUSD); s != "" {
					minUSD, _ = strconv.ParseFloat(s, 64)
				}

				log.Printf("[SWAP_FINANCE][CFG] tx=%s minUSD=%.2f native=%s",
					tx.Hash, minUSD, nativeSymbol,
				)

				addrLower := strings.ToLower(addr)
				fromLower := strings.ToLower(from)
				// toLower := strings.ToLower(to)

				var ins []SwapMove
				var outs []SwapMove

				// =========================
				// 0️⃣ receipt (потрібен і для ERC20 і для route)
				// =========================
				receipt, err := client.TransactionReceipt(ctx, common.HexToHash(tx.Hash))
				if err != nil {
					log.Printf("[SWAP_FINANCE][ERR] receipt err=%v", err)
					continue
				}

				// =========================
				// 1️⃣ NATIVE (ТІЛЬКИ реальний OUT через tx.Value)
				// =========================
				nativePriceUSD, err := GetNativePriceUSD(int64(chainID), client)
				if err != nil {
					log.Printf("[SWAP_FINANCE][NATIVE][ERR] price err=%v", err)
				} else if valWei.Sign() > 0 {

					v := new(big.Float).SetInt(valWei)
					v.Quo(v, big.NewFloat(1e18))
					amount, _ := v.Float64()
					usd := amount * nativePriceUSD

					log.Printf("[SWAP_FINANCE][NATIVE] valueWei=%s amount=%.9f price=%.4f usd=%.4f from=%s to=%s",
						valWei.String(), amount, nativePriceUSD, usd, from, to,
					)

					// IMPORTANT:
					// tx.Value = native, який віддав sender (from). Це OUT для addr тільки якщо from == addr.
					if fromLower == addrLower {
						outs = append(outs, SwapMove{
							Token:       nativeSymbol,
							TokenSymbol: nativeSymbol,
							Amount:      amount,
							USD:         usd,
							IsNative:    true,
							In:          false,
						})
						log.Printf("[SWAP_FINANCE][NATIVE][OUT] %.9f %s (%.4f USD)", amount, nativeSymbol, usd)
					} else {
						log.Printf("[SWAP_FINANCE][NATIVE][SKIP] from!=addr (native OUT не наш)")
					}
				}

				// =========================
				// 2️⃣ ERC20 (receipt logs)
				// =========================
				// stable list залежить від мережі
				stableUSD := map[string]struct{}{}
				switch chainID {
				case 1: // ETH
					stableUSD = map[string]struct{}{
						"0xdac17f958d2ee523a2206206994597c13d831ec7": {}, // USDT
						"0xa0b86991c6218b36c1d19d4a2e9eb0ce3606eb48": {}, // USDC
						"0x6b175474e89094c44da98b954eedeac495271d0f": {}, // DAI
					}
				case 56: // BSC
					stableUSD = map[string]struct{}{
						"0x55d398326f99059ff775485246999027b3197955": {}, // USDT
						"0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d": {}, // USDC
						"0xe9e7cea3dedca5984780bafc599bd69add087d56": {}, // BUSD
					}
				case 137: // Polygon
					stableUSD = map[string]struct{}{
						"0xc2132d05d31c914a87c6611c10748aeb04b58e8f": {}, // USDT
						"0x2791bca1f2de4661ed88a30c99a7a9449aa84174": {}, // USDC
						"0x8f3cf7ad23cd3cadbd9735aff958023239c6a063": {}, // DAI
					}
				}

				erc20Seen := 0
				for _, lg := range receipt.Logs {
					if len(lg.Topics) < 3 || lg.Topics[0] != transferTopic {
						continue
					}

					tokenAddr := strings.ToLower(lg.Address.Hex())
					val := new(big.Int).SetBytes(lg.Data)

					fromT := strings.ToLower(topicToAddress(lg.Topics[1]).Hex())
					toT := strings.ToLower(topicToAddress(lg.Topics[2]).Hex())

					// цікавлять лише реальні взаємодії адреси
					if fromT != addrLower && toT != addrLower {
						continue
					}

					erc20Seen++

					tokenPriceID, err := EnsureTokenPrice(chainID, tokenAddr, ctx)
					if err != nil {
						log.Printf("[SWAP_FINANCE][ERC20][SKIP] ensure price err token=%s err=%v", tokenAddr, err)
						continue
					}

					var price sql.NullFloat64
					_ = DB.QueryRow(
						`SELECT price_usd FROM token_prices WHERE id = ?`,
						tokenPriceID,
					).Scan(&price)

					if !price.Valid || price.Float64 <= 0 {
						log.Printf("[SWAP_FINANCE][ERC20][SKIP] no price token=%s price_id=%d", tokenAddr, tokenPriceID)
						continue
					}

					decimals, err := GetTokenDecimalsSafe(ctx, client, common.HexToAddress(tokenAddr))
					if err != nil {
						log.Printf("[SWAP_FINANCE][ERC20][SKIP] decimals err token=%s err=%v", tokenAddr, err)
						continue
					}

					amount := humanToFloat(val, decimals)
					usd := amount * price.Float64
					_, isStable := stableUSD[tokenAddr]

					symbol := GetTokenSymbolWithFallback(ctx, chainID, client, tokenAddr)

					if fromT == addrLower {
						outs = append(outs, SwapMove{
							Token:       tokenAddr,
							TokenSymbol: symbol,
							Amount:      amount,
							USD:         usd,
							IsStable:    isStable,
							In:          false,
						})
						log.Printf("[SWAP_FINANCE][ERC20][OUT] token=%s sym=%s amt=%.9f usd=%.4f stable=%v to=%s",
							tokenAddr, symbol, amount, usd, isStable, toT,
						)
					}

					if toT == addrLower {
						ins = append(ins, SwapMove{
							Token:       tokenAddr,
							TokenSymbol: symbol,
							Amount:      amount,
							USD:         usd,
							IsStable:    isStable,
							In:          true,
						})
						log.Printf("[SWAP_FINANCE][ERC20][IN] token=%s sym=%s amt=%.9f usd=%.4f stable=%v from=%s",
							tokenAddr, symbol, amount, usd, isStable, fromT,
						)
					}
				}

				log.Printf("[SWAP_FINANCE][AGG] tx=%s erc20_seen=%d ins=%d outs=%d from=%s to=%s",
					tx.Hash, erc20Seen, len(ins), len(outs), from, to,
				)

				// =========================
				// 3️⃣ Route (унікальні токени з Transfer логів, у порядку появи)
				// =========================
				routeMap := map[string]struct{}{}
				var route []string
				for _, lg := range receipt.Logs {
					if len(lg.Topics) < 1 || lg.Topics[0] != transferTopic {
						continue
					}
					tok := strings.ToLower(lg.Address.Hex())
					if _, ok := routeMap[tok]; ok {
						continue
					}
					routeMap[tok] = struct{}{}
					route = append(route, tok)
				}
				log.Printf("[SWAP_FINANCE][ROUTE] tx=%s hops=%d", tx.Hash, len(route))

				// =========================
				// 4️⃣ SUM + CUT
				// =========================
				if len(outs) == 0 {
					log.Printf("[SWAP_FINANCE][CUT] no OUT moves (нічого не віддали) tx=%s", tx.Hash)
					continue
				}

				var totalOutUSD, totalInUSD float64
				for _, o := range outs {
					totalOutUSD += o.USD
				}
				for _, i := range ins {
					totalInUSD += i.USD
				}

				log.Printf("[SWAP_FINANCE][SUM] tx=%s outUSD=%.4f inUSD=%.4f minUSD=%.4f",
					tx.Hash, totalOutUSD, totalInUSD, minUSD,
				)

				// пропускаємо тільки якщо ОБИДВА нижче порогу
				if totalOutUSD < minUSD && totalInUSD < minUSD {
					log.Printf("[SWAP_FINANCE][CUT] below minUSD tx=%s", tx.Hash)
					continue
				}

				k := emitKey("swap_finance", tx.Hash)
				if _, seen := emitted[k]; seen {
					log.Printf("[SWAP_FINANCE][SKIP] already emitted tx=%s", tx.Hash)
					continue
				}

				matches = append(matches, MatchedTxInfo{
					TxHash:     tx.Hash,
					From:       from,
					To:         to,
					RuleName:   r.RuleName,
					RuleID:     r.RuleID,
					UserID:     r.UserID,
					RuleTag:    "SwapFinance",
					NativToken: nativeSymbol,

					// залишаю як у тебе: показуємо "вартість події" по OUT (можеш змінити на max(out,in) як захочеш)
					ValueUSD: fmt.Sprintf("%.2f", totalOutUSD),

					SwapIns:   ins,
					SwapOuts:  outs,
					SwapRoute: route,
				})

				emitted[k] = struct{}{}

				log.Printf("[SWAP_FINANCE][MATCH] tx=%s emitted ins=%d outs=%d outUSD=%.4f inUSD=%.4f",
					tx.Hash, len(ins), len(outs), totalOutUSD, totalInUSD,
				)
			}

			// ----- SWAP -----
			if r.Filter.Swap != nil && r.Filter.Swap.Enabled && len(r.Filter.Swap.Tokens) > 0 {

				// swap має сенс тільки якщо tx належить гаманцю
				if addr != "" && addr != "0x" {
					if from != addr && to != addr {
						continue
					}
				}

				ctx, cancel := context.WithTimeout(context.Background(), 6*time.Second)
				res, err := CheckTokenTransfersInTxMax(ctx, client, common.HexToHash(tx.Hash), r.Filter.Swap)

				cancel()
				if err != nil {
					log.Printf("[SWAP][ERR] tx=%s err=%v", tx.Hash, err)
					continue
				}

				if res == nil {
					log.Printf("[SWAP][ERR] tx=%s res=nil", tx.Hash)
					continue
				}

				if len(res.Moves) == 0 {
					continue
				}

				switch res.Status {
				case TxPending:
					log.Printf("[SWAP][PENDING] tx=%s reason=price_not_ready", tx.Hash)
					continue

				case TxDone:
					// OK — обробляємо далі

				default:
					log.Printf("[SWAP][UNKNOWN] tx=%s status=%d", tx.Hash, res.Status)
					continue
				}

				if len(res.Moves) == 0 {
					log.Printf("[SWAP][DONE_NO_MATCH] tx=%s", tx.Hash)
					continue
				}

				for _, m := range res.Moves {

					k := emitKey("swap", m.Token.Hex())
					if _, seen := emitted[k]; seen {
						continue
					}

					toAddr := ""
					if res.Tx.To != nil {
						toAddr = res.Tx.To.Hex()
					}

					valueEth := "0"
					if res.Tx.ValueWei != nil {
						valueEth = res.Tx.ValueWei.String()
					}

					gasPrice := "0"
					if res.Tx.EffectiveGasPrice != nil {
						gasPrice = res.Tx.EffectiveGasPrice.String()
					}

					gasCost := "0"
					if res.Tx.GasCostWei != nil {
						gasCost = res.Tx.GasCostWei.String()
					}

					matches = append(matches, MatchedTxInfo{
						TxHash:   res.Tx.TxHash.Hex(),
						From:     res.Tx.From.Hex(),
						To:       toAddr,
						ValueEth: valueEth,
						GasUsed:  res.Tx.GasUsed,
						GasPrice: gasPrice,
						Nonce:    res.Tx.Nonce,
						RuleName: r.RuleName,
						RuleID:   r.RuleID,
						UserID:   r.UserID,
						RuleTag:  "swap",

						Token:  m.Token.Hex(),
						Amount: m.RawAmount.String(),

						TokenSymbol:   m.Symbol,
						TokenDecimals: m.Decimals,
						AmountHuman:   m.HumanAmount,
						PriceUSD:      m.PriceUSD,
						ValueUSD:      m.ValueUSD,
						Direction:     string(m.Direction),
						Status:        res.Tx.Status,
						GasCost:       gasCost,
					})
				}

			}
		}
	}

	return stats, matches
}

// func GetTransactionStatsAndMatchesJSON_Rules(txs []TxJSON, rules []AlertRule, resultAlerts []DBAlertFilter, client *ethclient.Client) (TxStats, []MatchedTxInfo) {
// 	normalizeHex := func(s string) string {
// 		s = strings.ToLower(strings.TrimSpace(s))
// 		if s == "" {
// 			return s
// 		}
// 		if !strings.HasPrefix(s, "0x") {
// 			s = "0x" + s
// 		}
// 		return s
// 	}
// 	ethToWei := func(s string) (*big.Int, bool) { return ethStringToWei(s) }

// 	stats := TxStats{
// 		Total:       len(txs),
// 		TotalGas:    big.NewInt(0),
// 		MaxTotalGas: big.NewInt(0),
// 	}

// 	matches := make([]MatchedTxInfo, 0, 64)
// 	emitted := make(map[string]struct{}) // ruleID|txHash

// 	for _, tx := range txs {
// 		// === статистика ===
// 		if strings.TrimSpace(tx.To) == "" {
// 			stats.ContractCreations++
// 		} else {
// 			stats.EOAToEOA++
// 		}
// 		if gpWei, ok := ethToWei(tx.GasPrice); ok {
// 			cost := new(big.Int).Mul(gpWei, new(big.Int).SetUint64(tx.Gas))
// 			stats.TotalGas.Add(stats.TotalGas, cost)
// 			stats.MaxTotalGas.Add(stats.MaxTotalGas, cost)
// 		}

// 		from := normalizeHex(tx.From)
// 		to := normalizeHex(tx.To)

// 		valWei, ok := ethToWei(tx.Value)
// 		if !ok {
// 			valWei = big.NewInt(0)
// 		}

// 		var gpGwei *big.Int
// 		if gpWei, ok := ethToWei(tx.GasPrice); ok {
// 			gpGwei = new(big.Int).Quo(gpWei, big.NewInt(1_000_000_000))
// 		}

// 		// === правила ===
// 		for _, r := range resultAlerts {

// 			if r.Filter.ID == "" {
// 				continue
// 			}
// 			key := func() string { return r.Filter.ID + "|" + tx.Hash }
// 			addr := normalizeHex(r.Filter.Address)

// 			// --- ANOMALIES ---
// 			if r.Filter.Anomalies != nil && r.Filter.Anomalies.Enabled {
// 				if minStr := strings.TrimSpace(r.Filter.Anomalies.MinGasPriceGwei); minStr != "" && gpGwei != nil {
// 					if min, ok := new(big.Int).SetString(minStr, 10); ok && gpGwei.Cmp(min) >= 0 {
// 						k := key()
// 						if _, seen := emitted[k]; !seen {
// 							matches = append(matches, MatchedTxInfo{
// 								TxHash:   tx.Hash,
// 								From:     from,
// 								To:       to,
// 								ValueEth: tx.Value,
// 								GasUsed:  0,
// 								GasPrice: tx.GasPrice,
// 								Nonce:    tx.Nonce,
// 								RuleID:   r.RuleID,
// 								UserID:   r.UserID,
// 								RuleTag:  "anomaly_gas",
// 							})
// 							emitted[k] = struct{}{}
// 						}
// 					}
// 				}
// 				continue
// 			}

// 			// --- ETH TRANSFER ---
// 			if r.Filter.ETHTransfer != nil && r.Filter.ETHTransfer.Enabled {
// 				if from == addr || to == addr {
// 					pass := false
// 					if strings.TrimSpace(r.Filter.ETHTransfer.MinEth) == "" {
// 						pass = valWei.Sign() > 0
// 					} else if minWei, ok := ethToWei(strings.TrimSpace(r.Filter.ETHTransfer.MinEth)); ok {
// 						pass = valWei.Cmp(minWei) >= 0
// 					}
// 					if pass {
// 						k := key()
// 						if _, seen := emitted[k]; !seen {
// 							matches = append(matches, MatchedTxInfo{
// 								TxHash:   tx.Hash,
// 								From:     from,
// 								To:       to,
// 								ValueEth: tx.Value,
// 								GasUsed:  0,
// 								GasPrice: tx.GasPrice,
// 								Nonce:    tx.Nonce,
// 								RuleID:   r.RuleID,
// 								UserID:   r.UserID,
// 								RuleTag:  "eth_transfer",
// 							})
// 							emitted[k] = struct{}{}
// 						}
// 					}
// 				}
// 			}

// 			// --- SWAP ---

// 			if r.Filter.Swap != nil && r.Filter.Swap.Enabled && len(r.Filter.Swap.Tokens) > 0 {
// 				found, err := CheckTokenTransfersInTx(
// 					context.Background(),
// 					client,
// 					common.HexToHash(tx.Hash),
// 					r.Filter.Swap, // <-- прямо передаємо правило swap
// 				)
// 				if err != nil || len(found) == 0 {
// 					continue
// 				}

// 				k := key()
// 				if _, seen := emitted[k]; !seen {
// 					for _, f := range found {
// 						matches = append(matches, MatchedTxInfo{
// 							TxHash:   tx.Hash,
// 							From:     from,
// 							To:       to,
// 							ValueEth: tx.Value,
// 							GasUsed:  0,
// 							GasPrice: tx.GasPrice,
// 							Nonce:    tx.Nonce,
// 							RuleID:   r.RuleID,
// 							UserID:   r.UserID,
// 							RuleTag:  "swap",
// 							Token:    f.Token.Hex(),
// 							Amount:   f.Amount.String(), // RAW
// 						})
// 					}
// 					emitted[k] = struct{}{}
// 				}
// 			}

// 		}
// 	}

// 	return stats, matches
// }
