package main

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"math/big"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/gorilla/mux"
)

// ========== МОДЕЛЬ ПРАВИЛА (твоя) ==========

type AlertRule struct {
	ID          string `json:"id"`
	Creator     string `json:"creator,omitempty"` // тут ти кладеш telegram_id як string
	Address     string `json:"address"`
	AlertChain string `json:"alertChain"`
	Anomalies   *struct {
		Enabled         bool   `json:"enabled"`
		MinGasPriceGwei string `json:"minGasPriceGwei"`
	} `json:"anomalies,omitempty"`

	NFT *struct {
		Mint bool `json:"mint"`
		Buy  struct {
			Enabled   bool   `json:"enabled"`
			MinAmount string `json:"minAmount"`
			Currency  string `json:"currency"`
		} `json:"buy"`
		Sell struct {
			Enabled   bool   `json:"enabled"`
			MinAmount string `json:"minAmount"`
			Currency  string `json:"currency"`
		} `json:"sell"`
	} `json:"nft,omitempty"`

	Swap *SwapRule `json:"swap,omitempty"` // визначена в іншому файлі

	SwapFinance *struct {
		Enabled bool `json:"enabled"`

		// мінімальна сума в USD
		MinUSD string `json:"minUsd"`

		// режим валюти:
		// "NATIVE_ONLY" | "STABLE_ONLY" | "NATIVE_OR_STABLE"
		Allow struct {
			SellNative   bool `json:"sellNative"`
			BuyNative    bool `json:"buyNative"`
			BuyAnyNative bool `json:"buyAnyNative"`
			BuyAnyStable bool `json:"buyAnyStable"`
		} `json:"allow"`
	} `json:"swapFinance,omitempty"`

	CreatedAt int64 `json:"createdAt"`
}

// ========== ДОДАТКОВІ СТРУКТУРИ ДЛЯ ПОРТФЕЛЮ ==========

type TokenInfo struct {
	Address  string  `json:"address"`
	Symbol   string  `json:"symbol"`
	PriceUSD float64 `json:"price_usd"`
}
type TelegramUser struct {
	ID        int64  `json:"id"`
	Username  string `json:"username"`
	FirstName string `json:"first_name"`
}

func genID() string { return "rule_" + time.Now().Format("20060102T150405.000000000") }

func normHexAddr(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return ""
	}
	if !strings.HasPrefix(s, "0x") {
		s = "0x" + s
	}
	if len(s) != 42 {
		return ""
	}
	return s
}

func uniqLowerHex(addrs []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, len(addrs))
	for _, a := range addrs {
		low := normHexAddr(a)
		if low == "" {
			continue
		}
		if _, ok := seen[low]; ok {
			continue
		}
		seen[low] = struct{}{}
		out = append(out, low)
	}
	return out
}

// =====================================================================
// 1) ЮЗЕР ВІДКРИВАЄ MINI APP → СТВОРЮЄМО/ОТРИМУЄМО ЮЗЕРА В БД
// =====================================================================

func InitUserHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER InitUserHandler")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req struct {
		InitData string `json:"initData"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	if req.InitData == "" {
		http.Error(w, "initData required", 400)
		return
	}

	// // 🔐 1. перевіряємо підпис Telegram
	// user, err := VerifyTelegramInitData(req.InitData)
	// if err != nil {
	// 	http.Error(w, "telegram verify failed", 401)
	// 	return
	// }
	values, err := url.ParseQuery(req.InitData)
	if err != nil {
		http.Error(w, "bad initData", 400)
		return
	}

	var user TelegramUser
	if err := json.Unmarshal([]byte(values.Get("user")), &user); err != nil {
		http.Error(w, "bad user data", 400)
		return
	}

	// 👤 2. створюємо або отримуємо користувача
	userID, err := DB_GetOrCreateUser(
		user.ID,
		user.Username,
		user.FirstName,
	)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	// ✅ 3. повертаємо ВЖЕ ПЕРЕВІРЕНОГО користувача
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"user": map[string]any{
			"user_id":     userID,  // ← внутрішній DB ID
			"telegram_id": user.ID, // ← Telegram ID
			"username":    user.Username,
			"first_name":  user.FirstName,
		},
	})
}
func VerifyTelegramInitData(initData string) (*TelegramUser, error) {
	log.Println("HANDLER VerifyTelegramInitData")

	// 1️⃣ Decode initData
	decoded, err := url.QueryUnescape(initData)
	if err != nil {
		return nil, err
	}

	// 2️⃣ Parse query
	values, err := url.ParseQuery(decoded)
	if err != nil {
		return nil, err
	}

	// 3️⃣ Get hash
	hash := values.Get("hash")
	if hash == "" {
		return nil, errors.New("hash missing")
	}

	// 4️⃣ REMOVE hash & signature
	values.Del("hash")
	values.Del("signature") // 🔥 КРИТИЧНО

	// 5️⃣ Build data-check-string
	var pairs []string
	for k, v := range values {
		pairs = append(pairs, k+"="+v[0])
	}
	sort.Strings(pairs)
	dataCheckString := strings.Join(pairs, "\n")

	// 6️⃣ Create secret
	secret := sha256.Sum256([]byte(os.Getenv("TG_BOT_TOKEN")))

	// 7️⃣ Calculate HMAC
	h := hmac.New(sha256.New, secret[:])
	h.Write([]byte(dataCheckString))
	expected := hex.EncodeToString(h.Sum(nil))

	// 8️⃣ Compare
	if expected != hash {
		return nil, errors.New("telegram hash mismatch")
	}

	// 9️⃣ Parse user
	userJSON := values.Get("user")
	if userJSON == "" {
		return nil, errors.New("user missing")
	}

	var user TelegramUser
	if err := json.Unmarshal([]byte(userJSON), &user); err != nil {
		return nil, err
	}

	return &user, nil
}

// =====================================================================
// 2) СТВОРЕННЯ ПОРТФЕЛЯ ДЛЯ ЮЗЕРА
// =====================================================================

func CreatePortfolioHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER CreatePortfolioHandler")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		UserID int64  `json:"userId"` // це ID з таблиці users, НЕ telegram_id
		Name   string `json:"name"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if req.UserID == 0 || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "userId and name required", 400)
		return
	}

	pid, err := DB_CreatePortfolio(req.UserID, req.Name)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"portfolioId": pid,
	})
}

// Список портфелів юзера
func ListPortfoliosHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER ListPortfoliosHandler")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id required", 400)
		return
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "bad user_id", 400)
		return
	}

	ports, err := DB_GetPortfoliosByUser(userID)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(ports)
}

// =====================================================================
// 3) ДОДАТИ ТОКЕН У ПОРТФЕЛЬ + SNAPSHOT + ОНОВЛЕННЯ total_invested / pnl
// =====================================================================

func AddTokenToPortfolioHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER AddTokenToPortfolioHandler")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		PortfolioID int64   `json:"portfolioId"`
		Chain       string  `json:"chain"`
		Contract    string  `json:"contract"`
		Amount      float64 `json:"amount"`
		Invested    float64 `json:"invested"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("AddTokenToPortfolioHandler decode error: %v", err)
		http.Error(w, "bad json", 400)
		return
	}

	if req.PortfolioID == 0 || req.Contract == "" || req.Amount <= 0 || req.Invested <= 0 {
		log.Printf("AddTokenToPortfolioHandler invalid data: %+v", req)
		http.Error(w, "invalid data", 400)
		return
	}
	var chain int

	switch req.Chain {
	case "ETH":
		chain = 1
	case "BSC":
		chain = 56
	default:
		log.Printf("[ALERT][ERR] unknown chain: %s", req.Chain)
		chain = 1
	}
	req.Contract = strings.ToLower(req.Contract)
	log.Printf("AddTokenToPortfolioHandler req: %+v", req)

	// 1️⃣ snapshot ДО зміни (залишаємо як було)
	oldTokens, err := DB_GetTokensByPortfolio(req.PortfolioID)
	if err != nil {
		log.Printf("DB_GetTokensByPortfolio error: %v", err)
	}
	oldPortfolio, err := DB_GetPortfolioByID(req.PortfolioID)
	if err != nil {
		log.Printf("DB_GetPortfolioByID error: %v", err)
	}

	snap := map[string]any{
		"portfolio": oldPortfolio,
		"tokens":    oldTokens,
	}
	if err := DB_SavePortfolioSnapshot(req.PortfolioID, snap); err != nil {
		log.Printf("DB_SavePortfolioSnapshot error: %v", err)
	}
	log.Printf("chain %v", req.Chain)
	// 2️⃣ ENSURE токен у token_prices (ціна/символ підтягнуться асинхронно)
	tokenPriceID, err := EnsureTokenPrice(chain, req.Contract, ctx)
	if err != nil {
		log.Printf("EnsureTokenPrice error: %v", err)
		http.Error(w, "token init error", 500)
		return
	}

	// 3️⃣ перевіряємо чи токен вже є в портфелі (по старій логіці)
	existing, err := DB_GetTokenByPortfolioAndPriceID(req.PortfolioID, tokenPriceID)
	if err != nil {
		log.Printf("DB_GetTokenByPortfolioAndPriceID error: %v", err)
		http.Error(w, "db error", 500)
		return
	}

	buyPrice := req.Invested / req.Amount

	if existing == nil {
		// INSERT у портфель (БЕЗ ціни і symbol)
		tok := Token{
			PortfolioID:  req.PortfolioID,
			Chain:        chain,
			TokenPriceID: tokenPriceID,
			Contract:     req.Contract,
			Symbol:       "",
			Amount:       req.Amount,
			Invested:     req.Invested,
			BuyPriceUSD:  buyPrice,
		}

		if _, err := DB_AddToken(tok); err != nil {
			log.Printf("DB_AddToken error: %v", err)
			http.Error(w, "db error", 500)
			return
		}
	} else {
		// UPDATE (усереднення, ціна НЕ чіпається)
		newAmount := existing.Amount + req.Amount
		newInvested := existing.Invested + req.Invested

		newBuyPrice := 0.0
		if newAmount > 0 {
			newBuyPrice = newInvested / newAmount
		}

		if err := DB_UpdateToken(
			existing.ID,
			newAmount,
			newInvested,
			newBuyPrice,
		); err != nil {
			log.Printf("DB_UpdateToken error: %v", err)
			http.Error(w, "db error", 500)
			return
		}
	}

	// 4️⃣ Перерахунок портфеля (ціни можуть бути ще 0 — це нормально)
	newTokens, err := DB_GetTokensByPortfolio(req.PortfolioID)
	if err != nil {
		log.Printf("DB_GetTokensByPortfolio error: %v", err)
		http.Error(w, "db error", 500)
		return
	}

	var totalInv float64
	var totalPnl float64
	for _, t := range newTokens {
		totalInv += t.Invested
		valueNow := t.CurrentPriceUSD * t.Amount
		totalPnl += valueNow - t.Invested
	}

	if err := DB_UpdatePortfolioTotals(req.PortfolioID, totalInv, totalPnl); err != nil {
		log.Printf("DB_UpdatePortfolioTotals error: %v", err)
		http.Error(w, "db error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":            true,
		"totalInvested": totalInv,
		"totalPnl":      totalPnl,
	})
}

// =====================================================================
// 4) ALERT RULES: СТВОРЕННЯ БАЗОВОГО ПРАВИЛА (назва)
// =====================================================================

func CreateRuleNameHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER CreateRuleNameHandler")

	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		TelegramID int64  `json:"telegram_id"`
		Name       string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if req.TelegramID == 0 || strings.TrimSpace(req.Name) == "" {
		http.Error(w, "invalid data", 400)
		return
	}

	user, err := DB_GetUserByTelegramID(req.TelegramID)
	if err != nil {
		// якщо нема – створюємо
		uid, err2 := DB_GetOrCreateUser(req.TelegramID, "", "")
		if err2 != nil {
			http.Error(w, "db error user", 500)
			return
		}
		user = &User{ID: uid, TelegramID: req.TelegramID}
	}

	ruleID, err := DB_CreateAlertRule(user.ID, req.Name)
	if err != nil {
		log.Println("❌ DB_CreateAlertRule:", err)
		http.Error(w, "db error rule", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ruleId": ruleID,
	})
}

// =====================================================================
// 5) СТВОРЕННЯ ПОВНОГО ФІЛЬТРА (твій CreateAlertRuleHandler, але в БД)
// =====================================================================

func CreateAlertRuleHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER CreateAlertRuleHandler")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	body, _ := io.ReadAll(r.Body)
	r.Body.Close()

	var wrap struct {
		Payload json.RawMessage `json:"payload"`
	}
	if json.Unmarshal(body, &wrap) == nil && len(wrap.Payload) > 0 {
		body = wrap.Payload
	}

	var in struct {
		RuleID      int64           `json:"ruleId"` // ID правила ОБОВ'ЯЗКОВО
		Creator     string          `json:"creator"`
		Address     string          `json:"address"`
		AlertChain string          `json:"alertChain"`
		Anomalies   json.RawMessage `json:"anomalies"`
		NFT         json.RawMessage `json:"nft"`
		Swap        json.RawMessage `json:"swap"`
		SwapFinance json.RawMessage `json:"swapFinance"`
	}

	if err := json.Unmarshal(body, &in); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	// ❗ БЕЗ ruleId — НЕМОЖЛИВО ДОДАТИ ФІЛЬТР
	if in.RuleID == 0 {
		http.Error(w, "ruleId required", 400)
		return
	}

	addr := normHexAddr(in.Address)
	if addr == "" {
		http.Error(w, "invalid address", 400)
		return
	}

	// розбір структур
	var anomalies *struct {
		Enabled         bool   `json:"enabled"`
		MinGasPriceGwei string `json:"minGasPriceGwei"`
	}
	if len(in.Anomalies) > 0 {
		var a struct {
			Enabled         bool   `json:"enabled"`
			MinGasPriceGwei string `json:"minGasPriceGwei"`
		}
		if json.Unmarshal(in.Anomalies, &a) == nil {
			anomalies = &a
		}
	}

	var nft *struct {
		Mint bool `json:"mint"`
		Buy  struct {
			Enabled   bool   `json:"enabled"`
			MinAmount string `json:"minAmount"`
			Currency  string `json:"currency"`
		} `json:"buy"`
		Sell struct {
			Enabled   bool   `json:"enabled"`
			MinAmount string `json:"minAmount"`
			Currency  string `json:"currency"`
		} `json:"sell"`
	}
	if len(in.NFT) > 0 {
		var n struct {
			Mint bool `json:"mint"`
			Buy  struct {
				Enabled   bool   `json:"enabled"`
				MinAmount string `json:"minAmount"`
				Currency  string `json:"currency"`
			} `json:"buy"`
			Sell struct {
				Enabled   bool   `json:"enabled"`
				MinAmount string `json:"minAmount"`
				Currency  string `json:"currency"`
			} `json:"sell"`
		}
		if json.Unmarshal(in.NFT, &n) == nil {
			n.Buy.Currency = strings.ToUpper(n.Buy.Currency)
			n.Sell.Currency = strings.ToUpper(n.Sell.Currency)
			nft = &n
		}
	}
	var chain int

	switch in.AlertChain {
	case "ETH":
		chain = 1
	case "BSC":
		chain = 56
	default:
		log.Printf("[ALERT][ERR] unknown chain: %s", in.AlertChain)
		chain = 1
	}
	var swap *SwapRule
	if len(in.Swap) > 0 {

		var s SwapRule
		if err := json.Unmarshal(in.Swap, &s); err != nil {
			log.Printf("[RULE][ERR] invalid swap json: %v", err)
			http.Error(w, "invalid swap json", 400)
			return
		}

		s.Currency = strings.ToUpper(s.Currency)
		s.Tokens = uniqLowerHex(s.Tokens)

		const (
			waitTimeout = 6 * time.Second
			pollDelay   = 200 * time.Millisecond
		)

		for _, token := range s.Tokens {

			if !common.IsHexAddress(token) {
				log.Printf("[RULE][ERR] invalid token address: %s", token)
				http.Error(w, "invalid token address", 400)
				return
			}

			tokenPriceID, err := EnsureTokenPrice(chain, token, ctx)
			if err != nil {
				log.Printf("[RULE][ERR] token init failed: %s err=%v", token, err)
				http.Error(w, "token init failed", 500)
				return
			}

			deadline := time.Now().Add(waitTimeout)

			for {
				var price sql.NullFloat64
				var symbol sql.NullString

				err := DB.QueryRow(`
				SELECT price_usd, symbol
				FROM token_prices
				WHERE id = ?
			`, tokenPriceID).Scan(&price, &symbol)

				if err != nil {
					log.Printf("[RULE][ERR] token db error: %s err=%v", token, err)
					http.Error(w, "token db error", 500)
					return
				}

				// ✅ успішна ініціалізація
				if price.Valid && price.Float64 > 0 && symbol.Valid && symbol.String != "" {
					break
				}

				// ⏳ таймаут очікування
				if time.Now().After(deadline) {
					log.Printf("[RULE][ERR] token init timeout: %s", token)
					http.Error(w, "token init timeout", 500)
					return
				}

				time.Sleep(pollDelay)
			}
		}

		swap = &s
	}

	var swapFinance *struct {
		Enabled bool `json:"enabled"`

		// мінімальна сума в USD
		MinUSD string `json:"minUsd"`

		// режим валюти:
		// "NATIVE_ONLY" | "STABLE_ONLY" | "NATIVE_OR_STABLE"
		Allow struct {
			SellNative   bool `json:"sellNative"`
			BuyNative    bool `json:"buyNative"`
			BuyAnyNative bool `json:"buyAnyNative"`
			BuyAnyStable bool `json:"buyAnyStable"`
		} `json:"allow"`
	}
	if len(in.SwapFinance) > 0 {
		var e struct {
			Enabled bool `json:"enabled"`

			// мінімальна сума в USD
			MinUSD string `json:"minUsd"`

			// режим валюти:
			// "NATIVE_ONLY" | "STABLE_ONLY" | "NATIVE_OR_STABLE"
			Allow struct {
				SellNative   bool `json:"sellNative"`
				BuyNative    bool `json:"buyNative"`
				BuyAnyNative bool `json:"buyAnyNative"`
				BuyAnyStable bool `json:"buyAnyStable"`
			} `json:"allow"`
		}
		if json.Unmarshal(in.SwapFinance, &e) == nil {
			swapFinance = &e
		}
	}

	// формуємо структуру для збереження
	rule := AlertRule{
		ID:          genID(),
		Creator:     in.Creator,
		Address:     addr,
		AlertChain: in.AlertChain,
		Anomalies:   anomalies,
		NFT:         nft,
		Swap:        swap,
		SwapFinance: swapFinance,
		CreatedAt:   time.Now().Unix(),
	}
	userID, err := strconv.ParseInt(in.Creator, 10, 64)
	if err != nil {
		log.Printf("invalid creator value: %s", in.Creator)
		http.Error(w, "invalid creator value", 400)
		return
	}
	log.Printf("rule %v", rule)
	// ❗ ЗБЕРЕГАЄМО ЯК ФІЛЬТР, НЕ СТВОРЮЄМО НОВЕ ПРАВИЛО
	if err := DB_SaveAlertFilter(in.RuleID, rule, userID); err != nil {
		log.Printf("DB_SaveAlertFilter error: %v", err)
		http.Error(w, "failed to save alert filter", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":     true,
		"filter": rule,
	})
}

// =====================================================================
// 6) ОТРИМАТИ ПРАВИЛА ДЛЯ ЮЗЕРА (твій ListAlertRuleHandler, але з БД)
// =====================================================================

func ListAlertRuleHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER ListAlertRuleHandler")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	telegramIDStr := r.URL.Query().Get("telegram_id")
	if telegramIDStr == "" {
		// як і раніше — якщо немає id, повертаємо порожній список
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]AlertRule{})
		return
	}
	tg, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		http.Error(w, "bad telegram_id", 400)
		return
	}

	user, err := DB_GetUserByTelegramID(tg)
	if err != nil {
		// якщо юзера нема — просто пустий список
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]AlertRule{})
		return
	}

	rules, err := DB_GetAlertFiltersByUserID(user.ID)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	// щоб залишити твою логіку, запишемо Creator = telegram_id
	for i := range rules {
		rules[i].Creator = telegramIDStr
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(rules)
}

// =====================================================================
// 7) АЛЕРТИ ДЛЯ ЮЗЕРА (по telegram_id)
// =====================================================================

func ListUserAlertsHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER ListUserAlertsHandler")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	telegramIDStr := r.URL.Query().Get("telegram_id")
	if telegramIDStr == "" {
		http.Error(w, "telegram_id required", 400)
		return
	}
	tg, err := strconv.ParseInt(telegramIDStr, 10, 64)
	if err != nil {
		http.Error(w, "bad telegram_id", 400)
		return
	}

	user, err := DB_GetUserByTelegramID(tg)
	if err != nil {
		// якщо нема — алертів теж нема
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]any{})
		return
	}

	rows, err := DB_GetAlertsByUserID(user.ID, 50)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	// розпаковуємо details з JSON
	type AlertDTO struct {
		ID        int64       `json:"id"`
		RuleID    int64       `json:"ruleId"`
		TxHash    string      `json:"txHash"`
		Short     string      `json:"shortMessage"`
		Details   interface{} `json:"details"`
		CreatedAt time.Time   `json:"createdAt"`
	}

	resp := make([]AlertDTO, 0, len(rows))
	for _, row := range rows {
		var d any
		_ = json.Unmarshal(row.Details, &d)
		resp = append(resp, AlertDTO{
			ID:        row.ID,
			RuleID:    row.RuleID,
			TxHash:    row.TxHash,
			Short:     row.Short,
			Details:   d,
			CreatedAt: row.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}
func ListAlertRulesHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER ListAlertRulesHandler")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userIDStr := r.URL.Query().Get("user_id")
	if userIDStr == "" {
		http.Error(w, "user_id required", 400)
		return
	}
	userID, err := strconv.ParseInt(userIDStr, 10, 64)
	if err != nil {
		http.Error(w, "bad user_id", 400)
		return
	}

	// витягуємо правила
	rows, err := DB.Query(`
        SELECT id, name
        FROM alert_rules
        WHERE user_id = ?
    `, userID)

	if err != nil {
		http.Error(w, "db error", 500)
		return
	}
	defer rows.Close()

	type RuleCard struct {
		RuleID   int64  `json:"rule_id"`
		Name     string `json:"name"`
		Filters  int    `json:"filters"`
		NewCount int    `json:"new_count"`
	}

	var out []RuleCard

	for rows.Next() {
		var rule RuleCard

		if err := rows.Scan(&rule.RuleID, &rule.Name); err != nil {
			continue
		}

		// рахуємо фільтри правила
		DB.QueryRow(`SELECT COUNT(*) FROM alert_filters WHERE rule_id = ?`,
			rule.RuleID,
		).Scan(&rule.Filters)

		// TODO: рахуємо нові спрацювання (поки 0)
		rule.NewCount = 0

		out = append(out, rule)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(out)
}

// =====================================================================
// 8) ЛОГІКА ЦІНИ ТОКЕНІВ — залишаю як є (ти сам допиляєш при потребі)
// =====================================================================

type ContractsRequest struct {
	Contracts []string `json:"contracts"`
}

func ListTokenPrice(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var contracts []string

	if r.Method == http.MethodPost {
		var m map[string]string
		if err := json.NewDecoder(r.Body).Decode(&m); err != nil {
			http.Error(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
		for _, v := range m {
			contracts = append(contracts, v)
		}
	} else {
		contractsParam := r.URL.Query().Get("contracts")
		if contractsParam == "" {
			http.Error(w, "No contracts provided", http.StatusBadRequest)
			return
		}
		contracts = strings.Split(contractsParam, ",")
	}

	client := ClientOpen()
	var tokens []TokenInfo
	for _, addr := range contracts {
		tokenAddress := common.HexToAddress(addr)
		decimals, err := GetTokenDecimalsSafe(context.Background(), client, tokenAddress)
		if err != nil || decimals == 0 {
			decimals = 18
		}

		price, err := GetTokenPriceUS(addr)
		if err != nil {
			continue
		}

		symbol := "UNKNOWN"
		_ = bind.NewBoundContract(
			common.HexToAddress(addr),
			parseABI(`[{"constant":true,"inputs":[],"name":"symbol","outputs":[{"name":"","type":"string"}],"type":"function"}]`),
			client, client, client,
		).Call(nil, &[]any{&symbol}, "symbol")

		tokens = append(tokens, TokenInfo{
			Address:  addr,
			Symbol:   symbol,
			PriceUSD: price,
		})
	}
	log.Println("tokens:", tokens)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

func parseABI(abiJSON string) abi.ABI {
	parsed, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		log.Fatalf("parseABI: invalid ABI JSON: %v", err)
	}
	return parsed
}

func ToBigFloat(raw *big.Int, decimals uint8) *big.Float {
	if raw == nil {
		return big.NewFloat(0)
	}
	value := new(big.Float).SetInt(raw)
	denom := new(big.Float).SetFloat64(math.Pow10(int(decimals)))
	return new(big.Float).Quo(value, denom)
}
func CurrentAlertRules() ([]AlertRule, []DBAlertFilter) {
	rs, resultAlerts, err := DB_GetAllAlertRules()

	if err != nil {
		log.Println("DB error in CurrentAlertRules:", err)
		return nil, nil
	}
	return rs, resultAlerts
}

// GET /api/alert-rule/filters?rule_id=123
func ListFiltersByRuleHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER ListFiltersByRuleHandler")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ruleIDStr := r.URL.Query().Get("rule_id")

	if ruleIDStr == "" {
		http.Error(w, "rule_id required", 400)
		return
	}

	ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
	if err != nil {
		http.Error(w, "bad rule_id", 400)
		return
	}

	filters, err := DB_GetFiltersByRuleID(ruleID)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(filters)
}

// GET /api/portfolio/tokens?portfolio_id=45
func ListPortfolioTokensHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER ListPortfolioTokensHandler")
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", 405)
		return
	}

	pidStr := r.URL.Query().Get("portfolio_id")
	if pidStr == "" {
		http.Error(w, "portfolio_id required", 400)
		return
	}

	pid, err := strconv.ParseInt(pidStr, 10, 64)
	if err != nil {
		http.Error(w, "bad portfolio_id", 400)
		return
	}

	tokens, err := DB_GetTokensByPortfolio(pid)
	if err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

func HandleStats(w http.ResponseWriter, r *http.Request) {
	chain := r.URL.Query().Get("chain")

	var chainID int
	switch chain {
	case "eth":
		chainID = 1
	case "bsc":
		chainID = 2
	case "base":
		chainID = 3
	case "polygon":
		chainID = 4
	default:
		http.Error(w, "unknown chain", 400)
		return
	}

	var tx1h, tx24h int64
	var gas1h, gas24h string

	DB.QueryRow(`
		SELECT
		  COALESCE(SUM(tx_count),0),
		  COALESCE(SUM(gas_used),0)
		FROM chain_activity_hour
		WHERE chain_id = ?
		  AND hour_ts >= NOW() - INTERVAL 1 HOUR
	`, chainID).Scan(&tx1h, &gas1h)

	DB.QueryRow(`
		SELECT
		  COALESCE(SUM(tx_count),0),
		  COALESCE(SUM(gas_used),0)
		FROM chain_activity_hour
		WHERE chain_id = ?
		  AND hour_ts >= NOW() - INTERVAL 24 HOUR
	`, chainID).Scan(&tx24h, &gas24h)

	_ = json.NewEncoder(w).Encode(map[string]any{
		"chain":   chain,
		"tx_1h":   tx1h,
		"gas_1h":  gas1h,
		"tx_24h":  tx24h,
		"gas_24h": gas24h,
	})
}

func ListAlertsByRuleHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER ListAlertsByRuleHandler")
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1️⃣ rule_id (обовʼязковий)
	ruleIDStr := r.URL.Query().Get("rule_id")
	if ruleIDStr == "" {
		http.Error(w, "rule_id required", http.StatusBadRequest)
		return
	}

	ruleID, err := strconv.ParseInt(ruleIDStr, 10, 64)
	if err != nil || ruleID <= 0 {
		http.Error(w, "bad rule_id", http.StatusBadRequest)
		return
	}

	// 2️⃣ after_id (опціональний)
	var afterID int64
	if s := r.URL.Query().Get("after_id"); s != "" {
		afterID, err = strconv.ParseInt(s, 10, 64)
		if err != nil || afterID < 0 {
			http.Error(w, "bad after_id", http.StatusBadRequest)
			return
		}
	}

	// 3️⃣ DB only (ніякої логіки)
	alerts, err := DB_GetAlertsByRule(ruleID, afterID, 50)
	if err != nil {
		log.Println("❌ DB_GetAlertsByRule:", err)
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	// 4️⃣ JSON
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(alerts)
}
func DeleteAlertFilterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "id required", 400)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}

	if err := DB_DeleteAlertFilter(id); err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
func UpdateAlertFilterHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	var req struct {
		ID     int64       `json:"id"`
		Filter interface{} `json:"filter"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", 400)
		return
	}

	raw, _ := json.Marshal(req.Filter)

	if err := DB_UpdateAlertFilter(req.ID, raw); err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.WriteHeader(http.StatusOK)
}
func DeleteAlertRuleHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "id required", 400)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}

	if err := DB_DeleteAlertRule(id); err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
func DeletePortfolioHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodOptions {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "id required", 400)
		return
	}

	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		http.Error(w, "bad id", 400)
		return
	}

	if err := DB_DeletePortfolio(id); err != nil {
		http.Error(w, "db error", 500)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
func RealizePortfolioOperationHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("HANDLER RealizePortfolioOperationHandler")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	if r.Method == http.MethodOptions {
		w.WriteHeader(204)
		return
	}
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req PortfolioOperationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		log.Printf("❌ decode error: %v", err)
		http.Error(w, "bad json", 400)
		return
	}
	if req.PortfolioID == 0 || req.Type == "" {
		log.Printf("❌ invalid request: %+v", req)
		http.Error(w, "invalid data", 400)
		return
	}

	/* ===========================================================
	   1) SNAPSHOT
	   =========================================================== */
	oldTokens, _ := DB_GetTokensByPortfolio(req.PortfolioID)
	oldPortfolio, _ := DB_GetPortfolioByID(req.PortfolioID)

	_ = DB_SavePortfolioSnapshot(req.PortfolioID, map[string]any{
		"portfolio": oldPortfolio,
		"tokens":    oldTokens,
	})

	/* ===========================================================
	   2) SOURCE TOKEN (UPDATE або DELETE)
	   =========================================================== */

	// беремо поточний amount source-токена
	var oldAmount float64
	if err := DB.QueryRow(`
		SELECT amount
		FROM tokens
		WHERE id = ? AND portfolio_id = ?
	`,
		req.From.TokenID,
		req.PortfolioID,
	).Scan(&oldAmount); err != nil {
		log.Printf("❌ source token not found: %v", err)
		http.Error(w, "source token not found", 404)
		return
	}

	amountDelta := req.From.AmountDelta
	newAmount := oldAmount + amountDelta
	if newAmount < 0 {
		newAmount = 0
	}

	// 🔥 ПОВНА РЕАЛІЗАЦІЯ → DELETE
	if newAmount == 0 || almostZero(newAmount) {

		if err := DB_DeleteToken(req.From.TokenID); err != nil {
			log.Printf("❌ DELETE source token error: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}

	} else {
		log.Printf("realized:%v", req.From.RealizedDelta)
		// 🟢 ЧАСТКОВА РЕАЛІЗАЦІЯ → UPDATE
		if _, err := DB.Exec(`
			UPDATE tokens
			SET amount = ?, realized = realized + ?
			WHERE id = ? AND portfolio_id = ?
		`,
			newAmount,
			req.From.RealizedDelta,
			req.From.TokenID,
			req.PortfolioID,
		); err != nil {
			log.Printf("❌ UPDATE source token error: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}
	}

	/* ===========================================================
	   3) TARGET LOGIC
	   =========================================================== */
	switch req.Type {

	case "REALIZE_CASH":
		// нічого

	case "REALIZE_SWAP":
		if req.To == nil {
			http.Error(w, "to is required", 400)
			return
		}

		if _, err := DB.Exec(`
			UPDATE tokens
			SET amount = amount + ?, invested = invested + ?
			WHERE id = ? AND portfolio_id = ?
		`,
			req.To.AmountDelta,
			req.To.InvestedDelta,
			req.To.TokenID,
			req.PortfolioID,
		); err != nil {
			log.Printf("❌ UPDATE target token error: %v", err)
			http.Error(w, err.Error(), 500)
			return
		}

	case "REALIZE_NEW_TOKEN":
		if req.NewToken == nil || req.NewToken.Contract == "" || req.NewToken.Invested <= 0 {
			http.Error(w, "invalid newToken", 400)
			return
		}

		contract := strings.ToLower(req.NewToken.Contract)
		usd := req.NewToken.Invested

		tokenPriceID, err := EnsureTokenPrice(1, contract, ctx)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		var price float64
		_ = DB.QueryRow(`
			SELECT price_usd
			FROM token_prices
			WHERE id = ?
		`, tokenPriceID).Scan(&price)

		if price <= 0 {
			price, err = GetTokenPriceUS(contract)
			if err != nil || price <= 0 {
				http.Error(w, "cannot determine token price", 500)
				return
			}

			_, _ = DB.Exec(`
				UPDATE token_prices
				SET price_usd = ?, updated_at = NOW()
				WHERE id = ?
			`, price, tokenPriceID)
		}

		amount := usd / price

		existing, err := DB_GetTokenByPortfolioAndPriceID(req.PortfolioID, tokenPriceID)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		if existing == nil {
			tok := Token{
				PortfolioID:  req.PortfolioID,
				TokenPriceID: tokenPriceID,
				Contract:     contract,
				Symbol:       "",
				Amount:       amount,
				Invested:     usd,
				BuyPriceUSD:  price,
			}
			if _, err := DB_AddToken(tok); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		} else {
			if _, err := DB.Exec(`
				UPDATE tokens
				SET amount = amount + ?, invested = invested + ?
				WHERE id = ? AND portfolio_id = ?
			`,
				amount,
				usd,
				existing.ID,
				req.PortfolioID,
			); err != nil {
				http.Error(w, err.Error(), 500)
				return
			}
		}

	default:
		http.Error(w, "unknown operation type", 400)
		return
	}

	/* ===========================================================
	   4) RECALC PORTFOLIO
	   =========================================================== */
	newTokens, err := DB_GetTokensByPortfolio(req.PortfolioID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	var totalInv, totalPnl float64
	for _, t := range newTokens {
		totalInv += t.Invested
		totalPnl += t.CurrentPriceUSD*t.Amount - t.Invested
	}

	if err := DB_UpdatePortfolioTotals(req.PortfolioID, totalInv, totalPnl); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"ok":            true,
		"totalInvested": totalInv,
		"totalPnl":      totalPnl,
	})
}
func GetTokenHandler(w http.ResponseWriter, r *http.Request) {
	chainID, err := strconv.Atoi(r.URL.Query().Get("chain_id"))
	if err != nil || chainID == 0 {
		http.Error(w, "invalid chain_id", http.StatusBadRequest)
		return
	}

	address := r.URL.Query().Get("address")
	if address == "" {
		http.Error(w, "missing address", http.StatusBadRequest)
		return
	}

	token, err := DBGetTokenMetadata(chainID, address)
	if err != nil {
		http.Error(w, "token not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(token)
}
func UpsertTokenHandler(w http.ResponseWriter, r *http.Request) {
	var t TokenData

	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		http.Error(w, "invalid json body", http.StatusBadRequest)
		return
	}

	if t.ChainID == 0 || t.Address == "" {
		http.Error(w, "missing chain_id or address", http.StatusBadRequest)
		return
	}

	if err := DBUpsertTokenMetadata(t); err != nil {
		http.Error(w, "db error", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}

type TopTokenDTO struct {
	ChainID         int64   `json:"chain_id"`
	Token           string  `json:"token"`
	Symbol          string  `json:"symbol"`
	PriceUSD        float64 `json:"price_usd"`
	VolumeUSD24h    float64 `json:"volume_usd_24h"`
	TransferCount   int64   `json:"transfer_count"`
	NetExchangeFlow float64 `json:"net_exchange_flow"`
	WhaleEvents     int64   `json:"whale_events"`
}
type PortfolioTokenDashboardDTO struct {
	Symbol    string  `json:"symbol"`
	PriceUSD  float64 `json:"price_usd"`
	MarketCap float64 `json:"market_cap"`
	Holders   int64   `json:"holders"`
	Top10Pct  float64 `json:"top10_pct"`
	Top50Pct  float64 `json:"top50_pct"`

	Volume24h   float64 `json:"volume_24h"`
	NetFlow     float64 `json:"net_flow"`
	WhaleEvents int64   `json:"whale_events"`

	DailyReportEnabled   bool `json:"daily_report_enabled"`
	AnomalyAlertsEnabled bool `json:"anomaly_alerts_enabled"`
}
type NotificationDTO struct {
	Type  string    `json:"type"`
	Token string    `json:"token"`
	Text  string    `json:"text"`
	Time  time.Time `json:"time"`
}

func HandleGetTopActiveTokens(w http.ResponseWriter, r *http.Request) {
	limit := mustInt(r.URL.Query().Get("limit"), 20)

	rows, err := DB.Query(`
SELECT
	t.chain_id,
	LOWER(t.contract),
	t.symbol,
	IFNULL(m.price_usd,0),
	IFNULL(d.total_volume_usd,0),
	IFNULL(d.transfer_count,0),
	IFNULL(d.net_exchange_flow_usd,0),
	IFNULL(d.whale_events,0)
FROM token_daily_activity d
JOIN tokens_metadata t
  ON t.chain_id=d.chain_id AND LOWER(t.contract)=d.token
LEFT JOIN tokens_metadata m
  ON m.chain_id=d.chain_id AND LOWER(m.contract)=d.token
WHERE d.day_ts = UNIX_TIMESTAMP(CURDATE())
ORDER BY d.total_volume_usd DESC
LIMIT ?
`, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var out []TopTokenDTO
	for rows.Next() {
		var t TopTokenDTO
		if err := rows.Scan(
			&t.ChainID,
			&t.Token,
			&t.Symbol,
			&t.PriceUSD,
			&t.VolumeUSD24h,
			&t.TransferCount,
			&t.NetExchangeFlow,
			&t.WhaleEvents,
		); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		out = append(out, t)
	}

	json.NewEncoder(w).Encode(out)
}

func HandleGetPortfolioTokenDashboard(w http.ResponseWriter, r *http.Request) {
	portfolioID := mustInt64(mux.Vars(r)["portfolio_id"])
	token := strings.ToLower(mux.Vars(r)["token"])
	chainID := mustInt64(r.URL.Query().Get("chain_id"))

	var resp PortfolioTokenDashboardDTO

	_ = DB.QueryRow(`
SELECT symbol, IFNULL(price_usd,0), IFNULL(circulating_market_cap,0),
       IFNULL(holders,0), IFNULL(top10_pct,0), IFNULL(top50_pct,0)
FROM tokens_metadata
WHERE chain_id=? AND LOWER(contract)=?
`, chainID, token).Scan(
		&resp.Symbol,
		&resp.PriceUSD,
		&resp.MarketCap,
		&resp.Holders,
		&resp.Top10Pct,
		&resp.Top50Pct,
	)

	_ = DB.QueryRow(`
SELECT IFNULL(total_volume_usd,0), IFNULL(net_exchange_flow_usd,0), IFNULL(whale_events,0)
FROM token_daily_activity
WHERE chain_id=? AND token=? AND day_ts=UNIX_TIMESTAMP(CURDATE())
`, chainID, token).Scan(
		&resp.Volume24h,
		&resp.NetFlow,
		&resp.WhaleEvents,
	)

	trackingQuery := fmt.Sprintf(`
		SELECT
			COALESCE(ats.daily_report_enabled, 0),
			CASE
				WHEN p.onchain_alerts_enabled = 1
				 AND COALESCE(ats.enabled, 1) = 1
			 AND COALESCE(atn.enabled, 1) = 1
			THEN 1
			ELSE 0
		END
	FROM portfolios p
	JOIN tokens t ON t.portfolio_id = p.id
		JOIN token_prices tp ON tp.id = t.token_price_id
		LEFT JOIN tokens_metadata tm ON tm.chain_id = tp.chain_id AND LOWER(tm.contract) = LOWER(tp.contract)
		LEFT JOIN portfolio_asset_tracking_settings ats
		  ON ats.portfolio_id = p.id
		 AND ats.asset_key = %s
		LEFT JOIN portfolio_asset_tracking_networks atn
		  ON atn.portfolio_id = p.id
		 AND atn.chain_id = tp.chain_id
		 AND atn.token = LOWER(tp.contract)
		WHERE p.id = ? AND tp.chain_id = ? AND LOWER(tp.contract) = ?
		LIMIT 1
		`, portfolioAssetKeyExprSQL("tp", "tm"))
	_ = DB.QueryRow(trackingQuery, portfolioID, chainID, token).Scan(
		&resp.DailyReportEnabled,
		&resp.AnomalyAlertsEnabled,
	)

	json.NewEncoder(w).Encode(resp)
}

func HandleToggleDailyReport(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  int64  `json:"user_id"`
		ChainID int64  `json:"chain_id"`
		Token   string `json:"token"`
		Enabled bool   `json:"enabled"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	_, err := DB.Exec(`
INSERT INTO token_activity_subscriptions
(user_id, chain_id, token, daily_report_enabled)
VALUES (?, ?, ?, ?)
ON DUPLICATE KEY UPDATE daily_report_enabled=VALUES(daily_report_enabled)
`, req.UserID, req.ChainID, strings.ToLower(req.Token), req.Enabled)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.WriteHeader(http.StatusOK)
}
func HandleGetPortfolioNotifications(w http.ResponseWriter, r *http.Request) {
	portfolioID := mustInt64(mux.Vars(r)["portfolio_id"])

	rows, err := DB.Query(`
SELECT notif_type, token, payload_text, created_at
FROM portfolio_notifications
WHERE portfolio_id=?
ORDER BY created_at DESC
LIMIT 50
`, portfolioID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var out []NotificationDTO
	for rows.Next() {
		var n NotificationDTO
		rows.Scan(&n.Type, &n.Token, &n.Text, &n.Time)
		out = append(out, n)
	}

	json.NewEncoder(w).Encode(out)
}

func mustInt64(s string) int64 {
	v, _ := strconv.ParseInt(s, 10, 64)
	return v
}

func mustInt(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}

func HandleTogglePortfolioAnomalyAlerts(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PortfolioID int64 `json:"portfolio_id"`
		Enabled     bool  `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	_, err := DB.Exec(`
			UPDATE portfolios
			SET onchain_alerts_enabled = ?
			WHERE id = ?
			`, req.Enabled, req.PortfolioID)

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if req.Enabled {
		if err := EnsurePortfolioAssetTrackingDefaults(req.PortfolioID); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)
}

func HandleTokenActivity(w http.ResponseWriter, r *http.Request) {
	tokenID := mustInt64(r.URL.Query().Get("token_id"))
	limit := mustInt(r.URL.Query().Get("limit"), 100)

	if tokenID == 0 {
		http.Error(w, "token_id required", 400)
		return
	}

	ref, err := loadTokenRef(tokenID)
	if err != nil {
		http.Error(w, "token not found", 404)
		return
	}

	rows, err := DB.Query(`
SELECT
	block_time,
	tx_hash,
	from_addr,
	to_addr,
	amount_raw,
	amount_usd,
	direction,
	exchange_name
FROM token_transfer_events
WHERE chain_id = ? AND token = ?
ORDER BY block_time DESC
LIMIT ?
`, ref.ChainID, ref.Token, limit)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	// структура та JSON — БЕЗ змін
	// (твій код тут правильний)
}

func HandleTokenHourly(w http.ResponseWriter, r *http.Request) {
	tokenID := mustInt64(r.URL.Query().Get("token_id"))

	if tokenID == 0 {
		http.Error(w, "token_id required", 400)
		return
	}

	ref, err := loadTokenRef(tokenID)
	if err != nil {
		http.Error(w, "token not found", 404)
		return
	}

	rows, err := DB.Query(`
SELECT
	hour_ts,
	transfer_count,
	total_volume_usd,
	exchange_in_usd,
	exchange_out_usd,
	max_transfer_usd
FROM token_hourly_activity
WHERE chain_id = ? AND token = ?
ORDER BY hour_ts DESC
LIMIT 48
`, ref.ChainID, ref.Token)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	// JSON — без змін
}

func HandleTokenAnomalies(w http.ResponseWriter, r *http.Request) {
	tokenID := mustInt64(r.URL.Query().Get("token_id"))

	if tokenID == 0 {
		http.Error(w, "token_id required", 400)
		return
	}

	ref, err := loadTokenRef(tokenID)
	if err != nil {
		http.Error(w, "token not found", 404)
		return
	}

	rows, err := DB.Query(`
SELECT
	block_time,
	tx_hash,
	severity,
	reason,
	amount_usd,
	direction,
	exchange_name
FROM token_anomaly_events
WHERE chain_id = ? AND token = ?
ORDER BY block_time DESC
LIMIT 50
`, ref.ChainID, ref.Token)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	// JSON — без змін
}

func HandleTopActiveTokens(w http.ResponseWriter, r *http.Request) {
	hourTS := time.Now().UTC().Truncate(time.Hour).Unix()

	rows, err := DB.Query(`
SELECT
	t.id AS token_id,
	tha.token,
	SUM(tha.total_volume_usd) AS vol_usd,
	SUM(tha.transfer_count) AS transfers,
	SUM(tha.exchange_transfer_count) AS exchange_txs
FROM token_hourly_activity tha
JOIN token_prices tp ON tp.contract = tha.token AND tp.chain_id = tha.chain_id
JOIN tokens t ON t.token_price_id = tp.id
WHERE tha.hour_ts = ?
GROUP BY t.id, tha.token
ORDER BY vol_usd DESC
LIMIT 50
`, hourTS)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	type Row struct {
		TokenID     int64    `json:"token_id"`
		Token       string   `json:"token"`
		VolumeUSD   *float64 `json:"volume_usd,omitempty"`
		Transfers   int64    `json:"transfers"`
		ExchangeTxs int64    `json:"exchange_txs"`
	}

	var out []Row
	for rows.Next() {
		var r Row
		var vol sql.NullFloat64

		rows.Scan(
			&r.TokenID,
			&r.Token,
			&vol,
			&r.Transfers,
			&r.ExchangeTxs,
		)

		if vol.Valid {
			r.VolumeUSD = &vol.Float64
		}
		out = append(out, r)
	}

	json.NewEncoder(w).Encode(out)
}

type TokenRef struct {
	ChainID int64
	Token   string // lower hex
}

func loadTokenRef(tokenID int64) (*TokenRef, error) {
	var ref TokenRef

	err := DB.QueryRow(`
SELECT
	tp.chain_id,
	LOWER(tp.contract)
FROM tokens t
JOIN token_prices tp ON tp.id = t.token_price_id
WHERE t.id = ?
LIMIT 1
`, tokenID).Scan(&ref.ChainID, &ref.Token)

	if err != nil {
		return nil, err
	}
	return &ref, nil
}
func HandleTopActiveTokensFront(w http.ResponseWriter, r *http.Request) {
	rows, err := DB.Query(`
		SELECT
		  a.chain_id,
		  a.token,
		  IFNULL(tm.symbol, '') AS symbol,
		  a.transfer_count,
		  a.exchange_transfer_count,
		  IFNULL(m.unique_addresses, 0) AS unique_addresses,
		  IFNULL(m.top1_addr_share, 0) AS top1_addr_share,
		  IFNULL(m.exchange_share, 0) AS exchange_share,
		  IFNULL(m.p50_usd, 0) AS p50_usd,
		  IFNULL(m.p99_usd, 0) AS p99_usd
		FROM token_hourly_metrics m
		JOIN token_hourly_activity a
		  ON a.chain_id=m.chain_id AND a.token=m.token AND a.hour_ts=m.hour_ts
		LEFT JOIN tokens_metadata tm
		  ON tm.chain_id=a.chain_id AND tm.contract=a.token
		WHERE m.hour_ts = (
		  SELECT MAX(hour_ts) FROM token_hourly_metrics
		)
		ORDER BY unique_addresses DESC
		LIMIT 120
	`)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	type topRow struct {
		ChainID     int
		Token       string
		Symbol      string
		TxCount     int
		ExchangeTxs int
		UniqueAddr  int
		Top1Share   float64
		ExchangeSh  float64
		P50USD      float64
		P99USD      float64
	}

	var ranked []map[string]interface{}
	for rows.Next() {
		var rr topRow
		if err := rows.Scan(
			&rr.ChainID,
			&rr.Token,
			&rr.Symbol,
			&rr.TxCount,
			&rr.ExchangeTxs,
			&rr.UniqueAddr,
			&rr.Top1Share,
			&rr.ExchangeSh,
			&rr.P50USD,
			&rr.P99USD,
		); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		facts := buildTopTokenFacts(
			rr.UniqueAddr,
			rr.TxCount,
			rr.ExchangeTxs,
			rr.ExchangeSh,
			rr.Top1Share,
			rr.P50USD,
			rr.P99USD,
		)

		ranked = append(ranked, map[string]interface{}{
			"token":          rr.Token,
			"symbol":         rr.Symbol,
			"chain":          ChainName(rr.ChainID),
			"chain_id":       rr.ChainID,
			"activity_score": facts.ActivityScore,
			"health_score":   facts.HealthScore,
			"risk_score":     facts.RiskScore,
			"risk_reasons":   facts.RiskReasons,
		})
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	sort.SliceStable(ranked, func(i, j int) bool {
		ai, _ := ranked[i]["activity_score"].(float64)
		aj, _ := ranked[j]["activity_score"].(float64)
		return ai > aj
	})

	if len(ranked) > 50 {
		ranked = ranked[:50]
	}

	json.NewEncoder(w).Encode(ranked)
}

func HandleMarketTokenActivity(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(strings.ToLower(r.URL.Query().Get("q")))
	period := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("period")))
	sortBy := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("sort_by")))
	order := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("order")))

	chainID := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("chain_id")); raw != "" {
		v, err := strconv.Atoi(raw)
		if err == nil {
			chainID = v
		}
	}

	limit := 300
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			limit = v
		}
	}
	if limit > 1000 {
		limit = 1000
	}

	offset := 0
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		if v, err := strconv.Atoi(raw); err == nil && v > 0 {
			offset = v
		}
	}

	windowHours := int64(1)
	if period == "24h" || period == "day" {
		windowHours = 24
	}
	if period == "" {
		period = "1h"
	}

	if order != "asc" {
		order = "desc"
	}

	sortExprMap := map[string]string{
		"tx":         "tx_count",
		"addresses":  "unique_addresses",
		"symbol":     "symbol",
		"updated_at": "last_hour_ts",
	}
	computedSort := sortBy == "activity" || sortBy == "health" || sortBy == "risk"
	if _, ok := sortExprMap[sortBy]; !ok && !computedSort {
		sortBy = "activity"
		computedSort = true
	}

	nowHour := time.Now().UTC().Truncate(time.Hour).Unix()
	fromHour := nowHour - (windowHours * 3600)
	if period == "1h" {
		fromHour = nowHour - 3600
	}
	toHour := nowHour

	needle := "%" + q + "%"
	sortExpr := "tx_count"
	if !computedSort {
		sortExpr = sortExprMap[sortBy]
	}
	orderExpr := "DESC"
	if order == "asc" {
		orderExpr = "ASC"
	}

	sqlLimit := limit
	sqlOffset := offset
	if computedSort {
		sqlLimit = 5000
		sqlOffset = 0
	}

	query := `
SELECT
  tm.chain_id,
  LOWER(tm.contract) AS token,
  IFNULL(tm.symbol, '') AS symbol,
  '' AS name,
  IFNULL(a.tx_count, 0) AS tx_count,
  IFNULL(a.exchange_txs, 0) AS exchange_txs,
  IFNULL(a.unique_addresses, 0) AS unique_addresses,
  IFNULL(a.top1_share, 0) AS top1_share,
  IFNULL(a.exchange_share, 0) AS exchange_share,
  IFNULL(a.p50_usd, 0) AS p50_usd,
  IFNULL(a.p99_usd, 0) AS p99_usd,
  IFNULL(a.last_hour_ts, 0) AS last_hour_ts
FROM tokens_metadata tm
LEFT JOIN (
  SELECT
    h.chain_id,
    h.token,
    SUM(h.transfer_count) AS tx_count,
    SUM(h.exchange_transfer_count) AS exchange_txs,
    CAST(AVG(IFNULL(m.unique_addresses, 0)) AS SIGNED) AS unique_addresses,
    AVG(IFNULL(m.top1_addr_share, 0)) AS top1_share,
    AVG(IFNULL(m.exchange_share, 0)) AS exchange_share,
    AVG(IFNULL(m.p50_usd, 0)) AS p50_usd,
    AVG(IFNULL(m.p99_usd, 0)) AS p99_usd,
    MAX(h.hour_ts) AS last_hour_ts
  FROM token_hourly_activity h
  LEFT JOIN token_hourly_metrics m
    ON m.chain_id = h.chain_id AND m.token = h.token AND m.hour_ts = h.hour_ts
  WHERE h.hour_ts BETWEEN ? AND ?
  GROUP BY h.chain_id, h.token
) a
  ON a.chain_id = tm.chain_id AND a.token = LOWER(tm.contract)
WHERE
  (? = 0 OR tm.chain_id = ?)
  AND (
    ? = '' OR LOWER(tm.contract) LIKE ? OR LOWER(tm.symbol) LIKE ?
  )
`
	query += " ORDER BY " + sortExpr + " " + orderExpr + ", symbol ASC LIMIT ? OFFSET ?"

	rows, err := DB.Query(
		query,
		fromHour, toHour,
		chainID, chainID,
		q, needle, needle,
		sqlLimit, sqlOffset,
	)
	if err != nil {
		// Fallback for environments where token_hourly_metrics schema is incomplete.
		fallbackQuery := `
SELECT
  tm.chain_id,
  LOWER(tm.contract) AS token,
  IFNULL(tm.symbol, '') AS symbol,
  '' AS name,
  IFNULL(a.tx_count, 0) AS tx_count,
  IFNULL(a.exchange_txs, 0) AS exchange_txs,
  0 AS unique_addresses,
  0 AS top1_share,
  IFNULL(a.exchange_share, 0) AS exchange_share,
  0 AS p50_usd,
  0 AS p99_usd,
  IFNULL(a.last_hour_ts, 0) AS last_hour_ts
FROM tokens_metadata tm
LEFT JOIN (
  SELECT
    h.chain_id,
    h.token,
    SUM(h.transfer_count) AS tx_count,
    SUM(h.exchange_transfer_count) AS exchange_txs,
    AVG(
      IF(
        h.transfer_count > 0,
        CAST(h.exchange_transfer_count AS DOUBLE) / CAST(h.transfer_count AS DOUBLE),
        0
      )
    ) AS exchange_share,
    MAX(h.hour_ts) AS last_hour_ts
  FROM token_hourly_activity h
  WHERE h.hour_ts BETWEEN ? AND ?
  GROUP BY h.chain_id, h.token
) a
  ON a.chain_id = tm.chain_id AND a.token = LOWER(tm.contract)
WHERE
  (? = 0 OR tm.chain_id = ?)
  AND (
    ? = '' OR LOWER(tm.contract) LIKE ? OR LOWER(tm.symbol) LIKE ?
  )
`
		fallbackQuery += " ORDER BY " + sortExpr + " " + orderExpr + ", symbol ASC LIMIT ? OFFSET ?"
		rows, err = DB.Query(
			fallbackQuery,
			fromHour, toHour,
			chainID, chainID,
			q, needle, needle,
			sqlLimit, sqlOffset,
		)
		if err != nil {
			log.Printf("HandleMarketTokenActivity SQL failed (primary + fallback): %v", err)
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode([]map[string]interface{}{})
			return
		}
	}
	defer rows.Close()

	var out []map[string]interface{}
	for rows.Next() {
		var rr struct {
			ChainID       int
			Token         string
			Symbol        string
			Name          string
			TxCount       int
			ExchangeTxs   int
			UniqueAddr    int
			Top1Share     float64
			ExchangeShare float64
			P50USD        float64
			P99USD        float64
			LastHourTS    int64
		}

		if err := rows.Scan(
			&rr.ChainID,
			&rr.Token,
			&rr.Symbol,
			&rr.Name,
			&rr.TxCount,
			&rr.ExchangeTxs,
			&rr.UniqueAddr,
			&rr.Top1Share,
			&rr.ExchangeShare,
			&rr.P50USD,
			&rr.P99USD,
			&rr.LastHourTS,
		); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}

		facts := buildTopTokenFacts(
			rr.UniqueAddr,
			rr.TxCount,
			rr.ExchangeTxs,
			rr.ExchangeShare,
			rr.Top1Share,
			rr.P50USD,
			rr.P99USD,
		)

		out = append(out, map[string]interface{}{
			"token":            rr.Token,
			"symbol":           rr.Symbol,
			"name":             rr.Name,
			"chain":            ChainName(rr.ChainID),
			"chain_id":         rr.ChainID,
			"tx_count":         rr.TxCount,
			"unique_addresses": rr.UniqueAddr,
			"activity_score":   facts.ActivityScore,
			"health_score":     facts.HealthScore,
			"risk_score":       facts.RiskScore,
			"signal_strength":  facts.SignalStrength,
			"last_hour_ts":     rr.LastHourTS,
		})
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if computedSort {
		field := sortBy + "_score"
		sort.SliceStable(out, func(i, j int) bool {
			li, _ := out[i][field].(float64)
			lj, _ := out[j][field].(float64)
			if order == "asc" {
				return li < lj
			}
			return li > lj
		})

		if offset >= len(out) {
			out = []map[string]interface{}{}
		} else {
			end := offset + limit
			if end > len(out) {
				end = len(out)
			}
			out = out[offset:end]
		}
	}

	json.NewEncoder(w).Encode(out)
}

func HandleTokenDashboardFront(w http.ResponseWriter, r *http.Request) {
	chainID, err := strconv.Atoi(r.URL.Query().Get("chainId"))
	if err != nil {
		http.Error(w, "invalid chainId", 400)
		return
	}
	token := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("token")))
	if token == "" {
		http.Error(w, "token required", 400)
		return
	}

	/* ================= METADATA ================= */

	var symbol string
	var priceUSD sql.NullFloat64
	var decimals sql.NullInt64
	_ = DB.QueryRow(`
			SELECT symbol, price_usd, decimals FROM tokens_metadata
			WHERE chain_id=? AND LOWER(contract)=?
		`, chainID, token).Scan(&symbol, &priceUSD, &decimals)

	/* ================= LAST 6 HOURS ================= */

	rows, err := DB.Query(`
			SELECT
				a.transfer_count,
				a.exchange_transfer_count,
				IFNULL(a.exchange_in_usd, 0),
				IFNULL(a.exchange_out_usd, 0),
				m.unique_addresses,
				m.top1_addr_share,
				m.top3_addr_share,
				m.top5_addr_share,
				m.p50_raw,
				m.p95_raw,
				m.p99_raw,
				IFNULL(m.p50_usd, 0),
				IFNULL(m.p95_usd, 0),
				IFNULL(m.p99_usd, 0)
			FROM token_hourly_activity a
			JOIN token_hourly_metrics m
			  ON m.chain_id=a.chain_id
		 AND m.token=a.token
		 AND m.hour_ts=a.hour_ts
		WHERE a.chain_id=? AND a.token=?
		ORDER BY a.hour_ts DESC
		LIMIT 6
	`, chainID, token)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	type hourRow struct {
		tx, exTx, uniq         int
		exIn, exOut            float64
		top1, top3, top5       float64
		p50Raw, p95Raw, p99Raw sql.NullString
		p50, p95, p99          float64
	}

	var hours []hourRow
	for rows.Next() {
		var h hourRow
		if err := rows.Scan(
			&h.tx, &h.exTx,
			&h.exIn, &h.exOut,
			&h.uniq,
			&h.top1, &h.top3, &h.top5,
			&h.p50Raw, &h.p95Raw, &h.p99Raw,
			&h.p50, &h.p95, &h.p99,
		); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		hours = append(hours, h)
	}

	if len(hours) == 0 {
		resp := map[string]interface{}{
			"token":  token,
			"symbol": symbol,
			"chain":  ChainName(chainID),

			"state_label":     "NO DATA",
			"signal_strength": "none",
			"flow_narrative":  "No on-chain transfers were indexed for this token in the selected period (1h/24h).",
			"risk_reasons":    []string{},
			"scores":          map[string]interface{}{"health": 0, "risk": 0, "activity": 0},
			"interpretation":  map[string]interface{}{"transfer_sizes": "Not enough USD-enriched transfers yet to estimate transfer-size behavior.", "exchange_flow": "No exchange flow data for the selected period."},
			"facts":           map[string]interface{}{"active_addresses": 0, "avg_active_6h": 0, "new_addresses": 0, "returning_addresses": 0, "new_ratio": 0, "avg_new_ratio_6h": 0, "tx_count": 0, "avg_tx_6h": 0, "tx_growth_x": 0, "tx_per_address": 0, "exchange_txs": 0, "exchange_ratio": 0, "avg_exchange_ratio_6h": 0, "exchange_in": 0, "exchange_out": 0, "net_exchange": 0, "top1_share": 0, "top3_share": 0, "top5_share": 0, "p50": 0, "p95": 0, "p99": 0, "p50_token_qty": "--", "p95_token_qty": "--", "p99_token_qty": "--", "p50_qty_source": "none", "p95_qty_source": "none", "p99_qty_source": "none", "data_note": "No on-chain transfers were indexed for this token in the selected period (1h/24h)."},
		}
		json.NewEncoder(w).Encode(resp)
		return
	}

	cur := hours[0]

	var sumTx, sumUniq, sumExTx int
	for _, h := range hours {
		sumTx += h.tx
		sumUniq += h.uniq
		sumExTx += h.exTx
	}

	avgTx6h := sumTx / len(hours)
	avgUniq6h := sumUniq / len(hours)

	var txGrowth float64
	if len(hours) > 1 && hours[len(hours)-1].tx > 0 {
		txGrowth = float64(cur.tx) / float64(hours[len(hours)-1].tx)
	} else {
		txGrowth = 0 // Default to 0 if not calculable
	}

	var exRatio float64
	if cur.tx > 0 {
		exRatio = float64(cur.exTx) / float64(cur.tx) * 100
	} else {
		exRatio = 0 // Default to 0 if not calculable
	}

	var txPerAddr float64
	if cur.uniq > 0 {
		txPerAddr = float64(cur.tx) / float64(cur.uniq)
	} else {
		txPerAddr = 0 // Default to 0 if not calculable
	}

	scoreFacts := buildTopTokenFacts(
		cur.uniq,
		cur.tx,
		cur.exTx,
		exRatio/100,
		cur.top1,
		cur.p50,
		cur.p99,
	)
	flowNarrative := buildFlowNarrative(cur.tx, cur.uniq, avgTx6h, avgUniq6h, exRatio, txGrowth)
	transferInterpretation := buildTransferSizeInterpretation(cur.p50, cur.p95, cur.p99, cur.tx)
	exchangeInterpretation := buildExchangeInterpretation(cur.exIn, cur.exOut, exRatio)
	dec := int64(18)
	if decimals.Valid && decimals.Int64 >= 0 {
		dec = decimals.Int64
	}
	bestP50Raw := cur.p50Raw
	bestP95Raw := cur.p95Raw
	bestP99Raw := cur.p99Raw
	for _, h := range hours {
		if !bestP50Raw.Valid && h.p50Raw.Valid {
			bestP50Raw = h.p50Raw
		}
		if !bestP95Raw.Valid && h.p95Raw.Valid {
			bestP95Raw = h.p95Raw
		}
		if !bestP99Raw.Valid && h.p99Raw.Valid {
			bestP99Raw = h.p99Raw
		}
	}
	p50TokenQty, p50QtySource := tokenQtyFromRawOrUSD(bestP50Raw, dec, cur.p50, priceUSD)
	p95TokenQty, p95QtySource := tokenQtyFromRawOrUSD(bestP95Raw, dec, cur.p95, priceUSD)
	p99TokenQty, p99QtySource := tokenQtyFromRawOrUSD(bestP99Raw, dec, cur.p99, priceUSD)
	dataNote := ""
	if cur.tx == 0 && cur.uniq == 0 {
		dataNote = "No on-chain transfers were indexed for this token in the selected period (1h/24h)."
	}

	p50USD := sanitizeUSDMetric(cur.p50)
	p95USD := sanitizeUSDMetric(cur.p95)
	p99USD := sanitizeUSDMetric(cur.p99)

	resp := map[string]interface{}{
		"token":  token,
		"symbol": symbol,
		"chain":  ChainName(chainID),

		"state_label": DetectState(cur.uniq, avgUniq6h, exRatio),
		"scores": map[string]interface{}{
			"health":   scoreFacts.HealthScore,
			"risk":     scoreFacts.RiskScore,
			"activity": scoreFacts.ActivityScore,
		},
		"risk_reasons":    scoreFacts.RiskReasons,
		"flow_narrative":  flowNarrative,
		"signal_strength": scoreFacts.SignalStrength,
		"interpretation": map[string]interface{}{
			"transfer_sizes": transferInterpretation,
			"exchange_flow":  exchangeInterpretation,
		},

		"facts": map[string]interface{}{
			// activity
			"active_addresses": cur.uniq,
			"avg_active_6h":    avgUniq6h,

			// interest (немає raw → чесно 0)
			"new_addresses":       0,
			"returning_addresses": 0,
			"new_ratio":           0,
			"avg_new_ratio_6h":    0,

			// tx
			"tx_count":       cur.tx,
			"avg_tx_6h":      avgTx6h,
			"tx_growth_x":    round(txGrowth, 2),
			"tx_per_address": round(txPerAddr, 2),

			// exchange
			"exchange_txs":   cur.exTx,
			"exchange_ratio": round(exRatio, 2),
			"avg_exchange_ratio_6h": func() float64 {
				if sumTx <= 0 {
					return 0
				}
				return round(float64(sumExTx)/float64(sumTx)*100, 2)
			}(),

			"exchange_in":  cur.exIn,
			"exchange_out": cur.exOut,
			"net_exchange": round(cur.exIn-cur.exOut, 2),

			// distribution
			"top1_share": cur.top1,
			"top3_share": cur.top3,
			"top5_share": cur.top5,

			// sizes
			"p50":            p50USD,
			"p95":            p95USD,
			"p99":            p99USD,
			"p50_token_qty":  p50TokenQty,
			"p95_token_qty":  p95TokenQty,
			"p99_token_qty":  p99TokenQty,
			"p50_qty_source": p50QtySource,
			"p95_qty_source": p95QtySource,
			"p99_qty_source": p99QtySource,
			"price_usd": func() interface{} {
				if priceUSD.Valid {
					return round(priceUSD.Float64, 8)
				}
				return nil
			}(),
			"data_note": dataNote,
		},
	}

	json.NewEncoder(w).Encode(resp)
}

func ChainName(id int) string {
	switch id {
	case 1:
		return "Ethereum"
	case 56:
		return "BSC"
	case 8453:
		return "Base"
	case 137:
		return "Polygon"
	default:
		return "Unknown"
	}
}

func DetectState(active, avg int, exRatio float64) string {
	if active > avg*2 && exRatio > 5 {
		return "РАННІЙ / ПІДТВЕРДЖЕНИЙ ІНТЕРЕС"
	}
	return "АКТИВНІСТЬ"
}
func round(v float64, p int) float64 {
	m := math.Pow(10, float64(p))
	return math.Round(v*m) / m
}

type topTokenFacts struct {
	ActivityScore  float64
	HealthScore    float64
	RiskScore      float64
	SignalStrength string
	RiskReasons    []string
}

func buildTopTokenFacts(
	uniqueAddr int,
	txCount int,
	exchangeTxCount int,
	exchangeShareRatio float64,
	top1Share float64,
	p50USD float64,
	p99USD float64,
) topTokenFacts {
	if txCount == 0 && uniqueAddr == 0 {
		return topTokenFacts{
			ActivityScore:  0,
			HealthScore:    0,
			RiskScore:      0,
			SignalStrength: "none",
			RiskReasons:    []string{"No on-chain activity in selected period"},
		}
	}

	if top1Share > 1 {
		top1Share = top1Share / 100
	}
	if exchangeShareRatio > 1 {
		exchangeShareRatio = exchangeShareRatio / 100
	}

	activity := 28.0*math.Log1p(float64(uniqueAddr)) +
		20.0*math.Log1p(float64(txCount)) +
		12.0*clampValue(1-top1Share, 0, 1)
	activity = round(clampValue(activity, 0, 100), 2)

	volatilitySpread := 0.0
	if p50USD > 0 && p99USD > 0 {
		volatilitySpread = clampValue((p99USD-p50USD)/(p50USD+1), 0, 4)
	}

	riskRaw := 100 * (0.42*clampValue(top1Share, 0, 1) +
		0.28*clampValue(exchangeShareRatio, 0, 1) +
		0.30*(volatilitySpread/4.0))
	riskScore := round(clampValue(riskRaw, 0, 100), 2)

	healthRaw := activity*0.58 + (100-riskScore)*0.42
	healthScore := round(clampValue(healthRaw, 0, 100), 2)

	return topTokenFacts{
		ActivityScore:  activity,
		HealthScore:    healthScore,
		RiskScore:      riskScore,
		SignalStrength: signalBucket(activity),
		RiskReasons: buildRiskReasons(
			top1Share,
			exchangeShareRatio,
			volatilitySpread,
			txCount,
			uniqueAddr,
		),
	}
}

func buildRiskReasons(top1Share, exchangeShareRatio, volatilitySpread float64, txCount, uniqueAddr int) []string {
	var out []string
	if top1Share >= 0.45 {
		out = append(out, "High concentration: top holder dominates flow")
	}
	if exchangeShareRatio >= 0.60 {
		out = append(out, "Exchange-driven hour: majority of flow tied to exchange addresses")
	}
	if volatilitySpread >= 1.6 {
		out = append(out, "Transfer size dispersion is extreme (p99 much larger than median)")
	}
	if txCount > 0 && uniqueAddr > 0 {
		txPerAddr := float64(txCount) / float64(uniqueAddr)
		if txPerAddr >= 3.5 {
			out = append(out, "High churn per wallet indicates speculative behavior")
		}
	}
	if len(out) == 0 {
		out = append(out, "Risk profile is currently moderate")
	}
	if len(out) > 3 {
		out = out[:3]
	}
	return out
}

func signalBucket(activity float64) string {
	switch {
	case activity >= 75:
		return "strong"
	case activity >= 50:
		return "moderate"
	default:
		return "weak"
	}
}

func buildFlowNarrative(curTx, curUniq, avgTx6h, avgUniq6h int, exRatio, txGrowth float64) string {
	growthNote := "flat versus recent baseline"
	if txGrowth >= 1.35 {
		growthNote = "accelerating versus baseline"
	} else if txGrowth <= 0.75 {
		growthNote = "cooling down versus baseline"
	}

	exFlow := "balanced exchange involvement"
	if exRatio >= 55 {
		exFlow = "exchange-dominant flow"
	} else if exRatio <= 20 {
		exFlow = "mostly organic wallet-to-wallet flow"
	}

	uniqDelta := 0
	if avgUniq6h > 0 {
		uniqDelta = int(math.Round((float64(curUniq-avgUniq6h) / float64(avgUniq6h)) * 100))
	}

	return "Current hour is " + growthNote +
		", with " + exFlow +
		"; active wallets " + signedPct(uniqDelta) +
		" vs 6h average and transaction load " + strconv.Itoa(curTx) +
		" (6h avg " + strconv.Itoa(avgTx6h) + ")."
}

func signedPct(v int) string {
	if v > 0 {
		return "+" + strconv.Itoa(v) + "%"
	}
	return strconv.Itoa(v) + "%"
}

func buildTransferSizeInterpretation(p50, p95, p99 float64, txCount int) string {
	if txCount < 20 {
		return "Low sample size this hour, so percentile bands are less stable."
	}
	if p50 <= 0 || p95 <= 0 || p99 <= 0 {
		return "Not enough USD-enriched transfers yet to estimate transfer-size behavior."
	}
	spread95 := p95 / (p50 + 1)
	spread99 := p99 / (p50 + 1)
	switch {
	case spread99 >= 15:
		return "Whale-tail profile: median transfers are much smaller than top-tier transfers."
	case spread95 >= 6:
		return "Wide dispersion: the market mixes regular flow with periodic large transfers."
	default:
		return "Balanced size profile: transfer amounts are relatively consistent."
	}
}

func buildExchangeInterpretation(exchangeInUSD, exchangeOutUSD, exchangeRatio float64) string {
	net := exchangeInUSD - exchangeOutUSD
	absNet := math.Abs(net)
	switch {
	case exchangeRatio >= 60 && net > 0 && absNet > 0:
		return "Exchange-heavy hour with net inflow to exchanges, which can signal potential sell pressure."
	case exchangeRatio >= 60 && net < 0 && absNet > 0:
		return "Exchange-heavy hour with net outflow from exchanges, which can indicate accumulation/withdrawals."
	case exchangeRatio <= 20:
		return "Most activity is outside exchange-tagged wallets, suggesting more organic wallet flow."
	default:
		return "Exchange participation is moderate and does not dominate current token flow."
	}
}

func rawToTokenUnitsString(raw sql.NullString, decimals int64) string {
	if !raw.Valid {
		return "--"
	}
	s := strings.TrimSpace(raw.String)
	if s == "" {
		return "--"
	}
	v, ok := new(big.Int).SetString(s, 10)
	if !ok {
		return "--"
	}
	if decimals < 0 {
		decimals = 0
	}

	den := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil)
	if den.Sign() == 0 {
		return "--"
	}

	numF := new(big.Float).SetPrec(256).SetInt(v)
	denF := new(big.Float).SetPrec(256).SetInt(den)
	out := new(big.Float).SetPrec(256).Quo(numF, denF)

	txt := out.Text('f', 2)
	txt = strings.TrimRight(strings.TrimRight(txt, "0"), ".")
	if txt == "" {
		return "0"
	}
	return txt
}

func tokenQtyFromRawOrUSD(raw sql.NullString, decimals int64, usd float64, priceUSD sql.NullFloat64) (string, string) {
	if fromRaw := rawToTokenUnitsString(raw, decimals); fromRaw != "--" {
		return fromRaw, "raw"
	}
	if priceUSD.Valid && priceUSD.Float64 > 0 && usd > 0 {
		estimated := usd / priceUSD.Float64
		return strconv.FormatFloat(estimated, 'f', 2, 64), "estimated_from_usd"
	}
	return "--", "unavailable"
}

func sanitizeUSDMetric(v float64) interface{} {
	if !isFinite(v) || v <= 0 || v > 1e12 {
		return nil
	}
	return round(v, 2)
}

func isFinite(v float64) bool {
	return !math.IsNaN(v) && !math.IsInf(v, 0)
}

func clampValue(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

var txPerAddr float64

// Calculate txPerAddr inside the function where cur is defined, e.g., inside HandleTokenDashboardFront
