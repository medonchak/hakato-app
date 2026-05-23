package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
	"github.com/ethereum/go-ethereum/ethclient"
)

// ─── Contract ABIs (minimal) ─────────────────────────────────────────────────

const signalRegistryABI = `[
  {"name":"recordSignal","type":"function","inputs":[
    {"name":"chainId","type":"uint64"},
    {"name":"token","type":"address"},
    {"name":"tokenSymbol","type":"string"},
    {"name":"signalType","type":"uint8"},
    {"name":"reason","type":"string"},
    {"name":"confidence","type":"uint32"},
    {"name":"vwapPeriod","type":"uint32"},
    {"name":"buyThresholdBps","type":"int32"},
    {"name":"sellThresholdBps","type":"int32"},
    {"name":"priceUsd","type":"uint256"}
  ],"outputs":[{"name":"id","type":"uint256"}]},
  {"name":"markExecuted","type":"function","inputs":[{"name":"id","type":"uint256"}],"outputs":[]}
]`

const anomalyLoggerABI = `[
  {"name":"logAnomaly","type":"function","inputs":[
    {"name":"chainId","type":"uint64"},
    {"name":"token","type":"address"},
    {"name":"reason","type":"string"},
    {"name":"severity","type":"uint32"},
    {"name":"hourTs","type":"uint64"},
    {"name":"txHash","type":"string"}
  ],"outputs":[{"name":"id","type":"uint256"}]}
]`

// ─── Agent config ─────────────────────────────────────────────────────────────

type AgentConfig struct {
	Enabled             bool
	ChainID             int64
	SignalRegistryAddr  string
	AnomalyLoggerAddr   string
	AgentWalletAddr     string
	PrivateKeyHex       string
	TickInterval        time.Duration
}

func AgentConfigFromEnv() AgentConfig {
	return AgentConfig{
		Enabled:            os.Getenv("AGENT_ENABLED") == "true",
		ChainID:            5000,
		SignalRegistryAddr: os.Getenv("SIGNAL_REGISTRY_ADDR"),
		AnomalyLoggerAddr:  os.Getenv("ANOMALY_LOGGER_ADDR"),
		AgentWalletAddr:    os.Getenv("AGENT_WALLET_ADDR"),
		PrivateKeyHex:      os.Getenv("AGENT_PRIVATE_KEY"),
		TickInterval:       15 * time.Minute,
	}
}

// ─── Agent executor ──────────────────────────────────────────────────────────

type AgentExecutor struct {
	cfg    AgentConfig
	client *ethclient.Client
	sigReg *bind.BoundContract
	anomLog *bind.BoundContract
}

func NewAgentExecutor(cfg AgentConfig) (*AgentExecutor, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	client, err := ethclient.Dial("https://rpc.mantle.xyz")
	if err != nil {
		return nil, fmt.Errorf("dial mantle: %w", err)
	}

	parsedSig, err := abi.JSON(strings.NewReader(signalRegistryABI))
	if err != nil {
		return nil, err
	}
	parsedLog, err := abi.JSON(strings.NewReader(anomalyLoggerABI))
	if err != nil {
		return nil, err
	}

	var sigReg, anomLog *bind.BoundContract
	if cfg.SignalRegistryAddr != "" {
		sigReg = bind.NewBoundContract(
			common.HexToAddress(cfg.SignalRegistryAddr),
			parsedSig, client, client, client,
		)
	}
	if cfg.AnomalyLoggerAddr != "" {
		anomLog = bind.NewBoundContract(
			common.HexToAddress(cfg.AnomalyLoggerAddr),
			parsedLog, client, client, client,
		)
	}

	return &AgentExecutor{cfg: cfg, client: client, sigReg: sigReg, anomLog: anomLog}, nil
}

func (a *AgentExecutor) txOpts(ctx context.Context) (*bind.TransactOpts, error) {
	pk, err := crypto.HexToECDSA(strings.TrimPrefix(a.cfg.PrivateKeyHex, "0x"))
	if err != nil {
		return nil, err
	}
	opts, err := bind.NewKeyedTransactorWithChainID(pk, big.NewInt(a.cfg.ChainID))
	if err != nil {
		return nil, err
	}
	opts.Context = ctx
	return opts, nil
}

// Run starts the executor loop.
func (a *AgentExecutor) Run(ctx context.Context) {
	mantleTokens := []struct{ addr, symbol string }{
		{"native", "MNT"},
		{"0xcda86a272531e8640cd7f1a92c01839911b90bb0", "mETH"},
		{"0x5be26527e817998a7206475496fde1e68957c5a6", "USDY"},
	}

	ticker := time.NewTicker(a.cfg.TickInterval)
	defer ticker.Stop()

	log.Println("[agent] executor started")

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for _, tok := range mantleTokens {
				if err := a.evaluateToken(ctx, tok.addr, tok.symbol); err != nil {
					log.Printf("[agent] evaluateToken %s: %v", tok.symbol, err)
				}
			}
		}
	}
}

func (a *AgentExecutor) evaluateToken(ctx context.Context, tokenAddr, tokenSymbol string) error {
	strat, err := DB_LoadBestStrategy(a.cfg.ChainID, tokenAddr)
	if err != nil || strat == nil {
		return nil // no strategy yet
	}

	pts, err := loadHourlyPoints(a.cfg.ChainID, tokenAddr, strat.VWAPPeriod+1)
	if err != nil || len(pts) < 2 {
		return nil
	}

	vwaps := computeVWAP(pts, strat.VWAPPeriod)
	last := pts[len(pts)-1]
	vwap := vwaps[len(vwaps)-1]

	if last.PriceUSD <= 0 || vwap <= 0 {
		return nil
	}

	deviation := (last.PriceUSD - vwap) / vwap

	var sigType string
	var reason string

	switch {
	case deviation <= strat.BuyThresholdPct/100:
		sigType = "BUY"
		reason = fmt.Sprintf("price %.4f < VWAP %.4f (dev %.2f%%, thr %.1f%%)",
			last.PriceUSD, vwap, deviation*100, strat.BuyThresholdPct)

	case deviation >= strat.SellThresholdPct/100:
		sigType = "SELL"
		reason = fmt.Sprintf("price %.4f > VWAP %.4f (dev +%.2f%%, thr +%.1f%%)",
			last.PriceUSD, vwap, deviation*100, strat.SellThresholdPct)

	default:
		return nil // no signal
	}

	// Check cooldown in DB
	lastSig, _ := db_lastSignal(a.cfg.ChainID, tokenAddr)
	if lastSig != nil && time.Since(lastSig.CreatedAt) < time.Duration(strat.CooldownHours)*time.Hour {
		return nil
	}

	confidence := 0.75
	sigID, err := a.persistSignal(a.cfg.ChainID, tokenAddr, tokenSymbol, sigType, reason, confidence, last.PriceUSD, vwap)
	if err != nil {
		return fmt.Errorf("persistSignal: %w", err)
	}

	// Record on-chain
	if a.sigReg != nil && a.cfg.PrivateKeyHex != "" {
		if onChainID, err := a.recordSignalOnChain(ctx, tokenAddr, tokenSymbol, sigType, reason, strat, last.PriceUSD); err != nil {
			log.Printf("[agent] recordSignalOnChain %s: %v", tokenSymbol, err)
		} else {
			_ = db_setSignalOnChainID(sigID, onChainID)
		}
	}

	log.Printf("[agent] signal %s %s: %s", sigType, tokenSymbol, reason)

	// Notify Telegram
	NotifyAgentTrade(sigType, tokenSymbol, last.PriceUSD, reason)

	return nil
}

func (a *AgentExecutor) recordSignalOnChain(
	ctx context.Context,
	tokenAddr, tokenSymbol, sigType, reason string,
	strat *OptimResult,
	priceUSD float64,
) (int64, error) {
	opts, err := a.txOpts(ctx)
	if err != nil {
		return 0, err
	}

	sigTypeUint := uint8(0) // NONE
	switch sigType {
	case "BUY":
		sigTypeUint = 1
	case "SELL":
		sigTypeUint = 2
	case "HOLD":
		sigTypeUint = 3
	}

	token := common.HexToAddress(tokenAddr)
	if tokenAddr == "native" {
		token = common.HexToAddress("0x78c1b0C915c4FAA5FffA6CAbf0219DA63d7f4cb8")
	}

	priceScaled := new(big.Int).SetInt64(int64(priceUSD * 1e8))

	tx, err := a.sigReg.Transact(opts, "recordSignal",
		uint64(a.cfg.ChainID),
		token,
		tokenSymbol,
		sigTypeUint,
		reason,
		uint32(7500), // 75% confidence in bps
		uint32(strat.VWAPPeriod),
		int32(strat.BuyThresholdPct*100),
		int32(strat.SellThresholdPct*100),
		priceScaled,
	)
	if err != nil {
		return 0, err
	}
	log.Printf("[agent] SignalRegistry.recordSignal tx=%s", tx.Hash().Hex())
	return 0, nil // on-chain id available after receipt parse
}

// ─── DB helpers ───────────────────────────────────────────────────────────────

type agentSignalRow struct {
	ID        int64
	SignalType string
	CreatedAt time.Time
}

func db_lastSignal(chainID int64, token string) (*agentSignalRow, error) {
	var r agentSignalRow
	err := DB.QueryRow(`
SELECT id, signal_type, created_at FROM agent_signals
WHERE chain_id=? AND token=?
ORDER BY created_at DESC LIMIT 1
`, chainID, token).Scan(&r.ID, &r.SignalType, &r.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &r, err
}

func persistSignal(chainID int64, token, symbol, sigType, reason string, confidence, priceUSD, vwap float64) (int64, error) {
	res, err := DB.Exec(`
INSERT INTO agent_signals (chain_id, token, token_symbol, signal_type, reason, confidence, price_usd, vwap, created_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, NOW())
`, chainID, token, symbol, sigType, reason, confidence, priceUSD, vwap)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (a *AgentExecutor) persistSignal(chainID int64, token, symbol, sigType, reason string, confidence, priceUSD, vwap float64) (int64, error) {
	return persistSignal(chainID, token, symbol, sigType, reason, confidence, priceUSD, vwap)
}

func db_setSignalOnChainID(sigID, onChainID int64) error {
	_, err := DB.Exec(`UPDATE agent_signals SET on_chain_id=? WHERE id=?`, onChainID, sigID)
	return err
}

// ─── HTTP handlers for Agent tab ─────────────────────────────────────────────

func HandleAgentSignals(w http.ResponseWriter, r *http.Request) {
	chainID := int64(5000)
	token := strings.ToLower(r.URL.Query().Get("token"))
	// map symbol to address
	switch token {
	case "meth":
		token = "0xcda86a272531e8640cd7f1a92c01839911b90bb0"
	case "mnt":
		token = "native"
	case "usdy":
		token = "0x5be26527e817998a7206475496fde1e68957c5a6"
	}

	rows, err := DB.Query(`
SELECT id, chain_id, token, token_symbol, signal_type, reason, confidence, price_usd, vwap, size_usd, tx_hash, executed, created_at
FROM agent_signals
WHERE chain_id=? AND token=?
ORDER BY created_at DESC
LIMIT 100
`, chainID, token)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	type row struct {
		ID         int64    `json:"id"`
		ChainID    int64    `json:"chain_id"`
		Token      string   `json:"token"`
		Symbol     string   `json:"token_symbol"`
		SignalType string   `json:"signal_type"`
		Reason     string   `json:"reason"`
		Confidence float64  `json:"confidence"`
		PriceUSD   float64  `json:"price_usd"`
		VWAP       float64  `json:"vwap"`
		SizeUSD    *float64 `json:"size_usd"`
		TxHash     *string  `json:"tx_hash"`
		Executed   bool     `json:"executed"`
		CreatedAt  time.Time `json:"created_at"`
	}

	var out []row
	for rows.Next() {
		var item row
		var exec int
		var sizeUSD sql.NullFloat64
		var txHash sql.NullString
		if err := rows.Scan(&item.ID, &item.ChainID, &item.Token, &item.Symbol,
			&item.SignalType, &item.Reason, &item.Confidence,
			&item.PriceUSD, &item.VWAP, &sizeUSD, &txHash,
			&exec, &item.CreatedAt,
		); err != nil {
			continue
		}
		item.Executed = exec == 1
		if sizeUSD.Valid {
			item.SizeUSD = &sizeUSD.Float64
		}
		if txHash.Valid {
			item.TxHash = &txHash.String
		}
		out = append(out, item)
	}

	writeJSON(w, out)
}

func HandleAgentStrategy(w http.ResponseWriter, r *http.Request) {
	chainID := int64(5000)
	token := strings.ToLower(r.URL.Query().Get("token"))
	switch token {
	case "meth":
		token = "0xcda86a272531e8640cd7f1a92c01839911b90bb0"
	case "mnt":
		token = "native"
	case "usdy":
		token = "0x5be26527e817998a7206475496fde1e68957c5a6"
	}

	strat, err := DB_LoadBestStrategy(chainID, token)
	if err != nil || strat == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNoContent)
		return
	}

	type resp struct {
		VWAPPeriod       int     `json:"vwap_period"`
		BuyThreshold     float64 `json:"buy_threshold"`
		SellThreshold    float64 `json:"sell_threshold"`
		CooldownHours    int     `json:"cooldown_hours"`
		Sharpe           float64 `json:"sharpe"`
		WinRate          float64 `json:"win_rate"`
		TotalTrades      int     `json:"total_trades"`
	}

	writeJSON(w, resp{
		VWAPPeriod:    strat.VWAPPeriod,
		BuyThreshold:  strat.BuyThresholdPct,
		SellThreshold: strat.SellThresholdPct,
		CooldownHours: strat.CooldownHours,
		Sharpe:        strat.Sharpe,
		WinRate:       strat.WinRate,
		TotalTrades:   strat.TotalTrades,
	})
}

func HandleAgentPosition(w http.ResponseWriter, r *http.Request) {
	chainID := int64(5000)

	rows, err := DB.Query(`SELECT token, token_symbol, size_usd, entry_price, pnl_usd, updated_at FROM agent_positions WHERE chain_id=?`, chainID)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	type pos struct {
		Token     string    `json:"token"`
		Symbol    string    `json:"token_symbol"`
		SizeUSD   float64   `json:"size_usd"`
		EntryPrice float64  `json:"entry_price"`
		PnlUSD    float64   `json:"pnl_usd"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	var out []pos
	for rows.Next() {
		var p pos
		if err := rows.Scan(&p.Token, &p.Symbol, &p.SizeUSD, &p.EntryPrice, &p.PnlUSD, &p.UpdatedAt); err == nil {
			out = append(out, p)
		}
	}
	if err := rows.Err(); err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	if len(out) == 0 {
		writeJSON(w, map[string]any{"size_usd": nil, "pnl_usd": nil})
		return
	}
	writeJSON(w, out[0])
}

func HandleAgentTradeConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		var chainID int64
		var wallet, token string
		var amount float64
		var updatedAt time.Time

		err := DB.QueryRow(`
SELECT chain_id, wallet_addr, trade_token, amount_usd, updated_at
FROM agent_trade_config WHERE chain_id=5000 LIMIT 1
`).Scan(&chainID, &wallet, &token, &amount, &updatedAt)

		if err == sql.ErrNoRows {
			writeJSON(w, map[string]any{"amount_usd": 5, "trade_token": "USDC", "wallet_addr": ""})
			return
		}
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		writeJSON(w, map[string]any{
			"chain_id":    chainID,
			"wallet_addr": wallet,
			"trade_token": token,
			"amount_usd":  amount,
			"updated_at":  updatedAt,
		})
		return
	}

	// POST
	var body struct {
		ChainID    int64   `json:"chain_id"`
		WalletAddr string  `json:"wallet_addr"`
		Token      string  `json:"token"`
		AmountUSD  float64 `json:"amount_usd"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "bad json", 400)
		return
	}
	if body.AmountUSD <= 0 {
		http.Error(w, "amount_usd must be > 0", 400)
		return
	}
	if body.ChainID == 0 {
		body.ChainID = 5000
	}

	_, err := DB.Exec(`
INSERT INTO agent_trade_config (chain_id, wallet_addr, trade_token, amount_usd, updated_at)
VALUES (?, ?, ?, ?, NOW())
ON DUPLICATE KEY UPDATE
  wallet_addr = VALUES(wallet_addr),
  trade_token = VALUES(trade_token),
  amount_usd  = VALUES(amount_usd),
  updated_at  = NOW()
`, body.ChainID, strings.ToLower(body.WalletAddr), body.Token, body.AmountUSD)

	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	log.Printf("[agent] trade config updated: wallet=%s token=%s amount=%.2f",
		body.WalletAddr, body.Token, body.AmountUSD)

	w.WriteHeader(http.StatusOK)
	writeJSON(w, map[string]any{"ok": true})
}

// DB_LoadTradeConfig returns the active trade config or default values.
func DB_LoadTradeConfig(chainID int64) (token string, amountUSD float64, walletAddr string) {
	token, amountUSD, walletAddr = "USDC", 5.0, ""
	_ = DB.QueryRow(`SELECT trade_token, amount_usd, wallet_addr FROM agent_trade_config WHERE chain_id=? LIMIT 1`, chainID).
		Scan(&token, &amountUSD, &walletAddr)
	return
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("[api] writeJSON encode err: %v", err)
	}
}

// NotifyAgentTrade sends a Telegram message about a new trade signal.
func NotifyAgentTrade(sigType, symbol string, priceUSD float64, reason string) {
	icon := "🟢"
	if sigType == "SELL" {
		icon = "🔴"
	}
	msg := fmt.Sprintf("%s Agent Signal: %s %s\nPrice: $%.4f\n%s", icon, sigType, symbol, priceUSD, reason)
	BroadcastTelegramMessage(msg)
}
