package main

// import (
// 	"context"
// 	"encoding/json"
// 	"io"
// 	"log"
// 	"math"
// 	"math/big"
// 	"net/http"
// 	"strings"
// 	"sync"
// 	"time"

// 	"github.com/ethereum/go-ethereum/accounts/abi"
// 	"github.com/ethereum/go-ethereum/accounts/abi/bind"
// 	"github.com/ethereum/go-ethereum/common"
// )

// // ========== МОДЕЛЬ ПРАВИЛА (відповідає полям форми) ==========
// type AlertRule struct {
// 	ID      string `json:"id"`
// 	Creator string `json:"creator,omitempty"`
// 	Address string `json:"address"` // 0x...

// 	Anomalies *struct {
// 		Enabled         bool   `json:"enabled"`
// 		MinGasPriceGwei string `json:"minGasPriceGwei"` // рядок (може бути "")
// 	} `json:"anomalies,omitempty"`

// 	NFT *struct {
// 		Mint bool `json:"mint"`
// 		Buy  struct {
// 			Enabled   bool   `json:"enabled"`
// 			MinAmount string `json:"minAmount"` // число рядком
// 			Currency  string `json:"currency"`  // "ETH" | "USDT"
// 		} `json:"buy"`
// 		Sell struct {
// 			Enabled   bool   `json:"enabled"`
// 			MinAmount string `json:"minAmount"`
// 			Currency  string `json:"currency"` // "ETH" | "USDT"
// 		} `json:"sell"`
// 	} `json:"nft,omitempty"`

// 	Swap *SwapRule `json:"swap,omitempty"`

// 	ETHTransfer *struct {
// 		Enabled bool   `json:"enabled"`
// 		MinEth  string `json:"minEth"`
// 	} `json:"ethTransfer,omitempty"`

// 	CreatedAt int64 `json:"createdAt"`
// }
// type TokenInfo struct {
// 	Address  string  `json:"address"`
// 	Symbol   string  `json:"symbol"`
// 	PriceUSD float64 `json:"price_usd"`
// }

// // ========== ГЛОБАЛЬНЕ СХОВИЩЕ ПРАВИЛ ==========
// var (
// 	alertRulesMu sync.RWMutex
// 	alertRules   []AlertRule
// )

// // CurrentAlertRules: знімок правил для аналітики
// func CurrentAlertRules() []AlertRule {
// 	alertRulesMu.RLock()
// 	defer alertRulesMu.RUnlock()
// 	out := make([]AlertRule, len(alertRules))
// 	copy(out, alertRules)
// 	return out
// }

// // ========== ХЕЛПЕРИ ==========
// func normHexAddr(s string) string {
// 	s = strings.ToLower(strings.TrimSpace(s))
// 	if s == "" {
// 		return ""
// 	}
// 	if !strings.HasPrefix(s, "0x") {
// 		s = "0x" + s
// 	}
// 	if len(s) != 42 {
// 		return ""
// 	}
// 	return s
// }

// func uniqLowerHex(addrs []string) []string {
// 	seen := map[string]struct{}{}
// 	out := make([]string, 0, len(addrs))
// 	for _, a := range addrs {
// 		low := normHexAddr(a)
// 		if low == "" {
// 			continue
// 		}
// 		if _, ok := seen[low]; ok {
// 			continue
// 		}
// 		seen[low] = struct{}{}
// 		out = append(out, low)
// 	}
// 	return out
// }

// func genID() string { return "rule_" + time.Now().Format("20060102T150405.000000000") }

// // ========== HTTP-HANDLERS ==========

// // POST /alert-rules
// // Приймає або плоский JSON правила, або {"payload": {...}}.
// func CreateAlertRuleHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusNoContent)
// 		return
// 	}

// 	// зчитати сире тіло
// 	body, _ := io.ReadAll(r.Body)
// 	_ = r.Body.Close()

// 	// якщо { payload: {...} } — розгортаємо
// 	var wrap struct {
// 		Payload json.RawMessage `json:"payload"`
// 	}
// 	if json.Unmarshal(body, &wrap) == nil && len(wrap.Payload) > 0 {
// 		body = wrap.Payload
// 	}

// 	// декодуємо у тимчасову структуру (така ж як AlertRule, але без ID/CreatedAt)
// 	var in struct {
// 		Creator     string          `json:"creator,omitempty"`
// 		Address     string          `json:"address"`
// 		Anomalies   json.RawMessage `json:"anomalies,omitempty"`
// 		NFT         json.RawMessage `json:"nft,omitempty"`
// 		Swap        json.RawMessage `json:"swap,omitempty"`
// 		ETHTransfer json.RawMessage `json:"ethTransfer,omitempty"`
// 	}

// 	if err := json.Unmarshal(body, &in); err != nil {
// 		http.Error(w, "bad json", http.StatusBadRequest)
// 		return
// 	}

// 	addr := normHexAddr(in.Address)
// 	if addr == "" {
// 		http.Error(w, "invalid address", http.StatusBadRequest)
// 		return
// 	}

// 	// парсимо вкладені блоки, якщо вони є
// 	var anomalies *struct {
// 		Enabled         bool   `json:"enabled"`
// 		MinGasPriceGwei string `json:"minGasPriceGwei"`
// 	}
// 	if len(in.Anomalies) > 0 {
// 		var a struct {
// 			Enabled         bool   `json:"enabled"`
// 			MinGasPriceGwei string `json:"minGasPriceGwei"`
// 		}
// 		if err := json.Unmarshal(in.Anomalies, &a); err == nil {
// 			anomalies = &a
// 		}
// 	}

// 	var nft *struct {
// 		Mint bool `json:"mint"`
// 		Buy  struct {
// 			Enabled   bool   `json:"enabled"`
// 			MinAmount string `json:"minAmount"`
// 			Currency  string `json:"currency"`
// 		} `json:"buy"`
// 		Sell struct {
// 			Enabled   bool   `json:"enabled"`
// 			MinAmount string `json:"minAmount"`
// 			Currency  string `json:"currency"`
// 		} `json:"sell"`
// 	}
// 	if len(in.NFT) > 0 {
// 		var n struct {
// 			Mint bool `json:"mint"`
// 			Buy  struct {
// 				Enabled   bool   `json:"enabled"`
// 				MinAmount string `json:"minAmount"`
// 				Currency  string `json:"currency"`
// 			} `json:"buy"`
// 			Sell struct {
// 				Enabled   bool   `json:"enabled"`
// 				MinAmount string `json:"minAmount"`
// 				Currency  string `json:"currency"`
// 			} `json:"sell"`
// 		}
// 		if err := json.Unmarshal(in.NFT, &n); err == nil {
// 			// нормалізація валюти
// 			n.Buy.Currency = strings.ToUpper(strings.TrimSpace(n.Buy.Currency))
// 			n.Sell.Currency = strings.ToUpper(strings.TrimSpace(n.Sell.Currency))
// 			nft = &n
// 		}
// 	}

// 	var swap *SwapRule
// 	if len(in.Swap) > 0 {
// 		var s SwapRule
// 		if err := json.Unmarshal(in.Swap, &s); err == nil {
// 			s.Currency = strings.ToUpper(strings.TrimSpace(s.Currency))
// 			s.Tokens = uniqLowerHex(s.Tokens)
// 			swap = &s
// 		}
// 	}

// 	var ethTransfer *struct {
// 		Enabled bool   `json:"enabled"`
// 		MinEth  string `json:"minEth"`
// 	}
// 	if len(in.ETHTransfer) > 0 {
// 		var e struct {
// 			Enabled bool   `json:"enabled"`
// 			MinEth  string `json:"minEth"`
// 		}
// 		if err := json.Unmarshal(in.ETHTransfer, &e); err == nil {
// 			ethTransfer = &e
// 		}
// 	}
// 	log.Println("Body:", swap)
// 	rule := AlertRule{
// 		ID:          genID(),
// 		Creator:     strings.TrimSpace(in.Creator),
// 		Address:     addr,
// 		Anomalies:   anomalies,
// 		NFT:         nft,
// 		Swap:        swap,
// 		ETHTransfer: ethTransfer,
// 		CreatedAt:   time.Now().Unix(),
// 	}
// 	////log.Println("🔴 Адреса:", rule) перевірка
// 	alertRulesMu.Lock()
// 	alertRules = append(alertRules, rule)
// 	alertRulesMu.Unlock()

// 	w.Header().Set("Content-Type", "application/json")
// 	_ = json.NewEncoder(w).Encode(map[string]any{"ok": true, "created": rule})

// }

// // GET /alert-rules
// func ListAlertRuleHandler(w http.ResponseWriter, r *http.Request) {
// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusNoContent)
// 		return
// 	}

// 	// Отримуємо Telegram ID з query параметра
// 	telegramID := r.URL.Query().Get("telegram_id")

// 	alertRulesMu.RLock()
// 	defer alertRulesMu.RUnlock()

// 	var filtered []AlertRule

// 	// Якщо ID передано — фільтруємо
// 	if telegramID != "" {
// 		for _, rule := range alertRules {
// 			if rule.Creator == telegramID {
// 				filtered = append(filtered, rule)
// 			}
// 		}
// 	} else {
// 		// Якщо ID не передано — нічого не повертаємо (порожній список)
// 		filtered = []AlertRule{}
// 	}

// 	// Відповідь JSON
// 	w.Header().Set("Content-Type", "application/json")
// 	if err := json.NewEncoder(w).Encode(filtered); err != nil {
// 		http.Error(w, err.Error(), http.StatusInternalServerError)
// 	}
// }

// type ContractsRequest struct {
// 	Contracts []string `json:"contracts"`
// }

// func ListTokenPrice(w http.ResponseWriter, r *http.Request) {
// 	if r.Method == http.MethodOptions {
// 		w.WriteHeader(http.StatusNoContent)
// 		return
// 	}

// 	var contracts []string

// 	if r.Method == http.MethodPost {

// 		// 1) Приймаємо формат: { "0": "0x...", "1": "0x..." }
// 		var m map[string]string
// 		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
// 			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
// 			return
// 		}

// 		// 2) Перетворюємо в масив контрактів
// 		for _, v := range m {
// 			contracts = append(contracts, v)
// 		}

// 	} else {
// 		contractsParam := r.URL.Query().Get("contracts")
// 		if contractsParam == "" {
// 			http.Error(w, "No contracts provided", http.StatusBadRequest)
// 			return
// 		}
// 		contracts = strings.Split(contractsParam, ",")
// 	}
// 	client := ClientOpen()
// 	var tokens []TokenInfo
// 	for _, addr := range contracts {
// 		tokenAddress := common.HexToAddress(addr)
// 		decimals, err := GetTokenDecimals(context.Background(), client, tokenAddress)
// 		if err != nil || decimals == 0 {
// 			decimals = 18
// 		}

// 		price, err := GetTokenPriceUS(addr)
// 		if err != nil {
// 			continue
// 		}

// 		// price, err := GetTokenPriceUSD(client, addr)
// 		// if err != nil {
// 		// 	continue
// 		// }

// 		symbol := "UNKNOWN"
// 		_ = bind.NewBoundContract(
// 			common.HexToAddress(addr),
// 			parseABI(`[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"type":"function"}]`),
// 			client, client, client,
// 		).Call(nil, &[]any{&symbol}, "symbol")

// 		tokens = append(tokens, TokenInfo{
// 			Address:  addr,
// 			Symbol:   symbol,
// 			PriceUSD: price,
// 		})
// 	}
// 	log.Println("tokens:", tokens)
// 	w.Header().Set("Content-Type", "application/json")
// 	json.NewEncoder(w).Encode(tokens)
// }

// func parseABI(abiJSON string) abi.ABI {
// 	parsed, err := abi.JSON(strings.NewReader(abiJSON))
// 	if err != nil {
// 		panic(err) // або обробляй помилку як тобі потрібно
// 	}
// 	return parsed
// }
// func ToBigFloat(raw *big.Int, decimals uint8) *big.Float {
// 	if raw == nil {
// 		return big.NewFloat(0)
// 	}
// 	value := new(big.Float).SetInt(raw)
// 	denom := new(big.Float).SetFloat64(math.Pow10(int(decimals)))
// 	return new(big.Float).Quo(value, denom)
// }
