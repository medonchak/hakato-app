package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"math/big"

	"strings"
	"sync"
	"time"

	"encoding/hex"
	"encoding/json"
	"net/http"
	"os"

	"github.com/ethereum/go-ethereum"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"github.com/ethereum/go-ethereum/ethclient"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/joho/godotenv"
	"github.com/rs/cors"
	"golang.org/x/sync/singleflight"
)

type Result struct {
	TotalGas string `json:"total_gas"`
	TotalTx  int    `json:"total_tx"`
}
type EtherscanTx struct {
	BlockNumber       string `json:"blockNumber"`
	TimeStamp         string `json:"timeStamp"`
	Hash              string `json:"hash"`
	Nonce             string `json:"nonce"`
	BlockHash         string `json:"blockHash"`
	TransactionIndex  string `json:"transactionIndex"`
	From              string `json:"from"`
	To                string `json:"to"`
	Value             string `json:"value"`
	Gas               string `json:"gas"`
	GasPrice          string `json:"gasPrice"`
	GasUsed           string `json:"gasUsed"`
	IsError           string `json:"isError"`
	TxReceiptStatus   string `json:"txreceipt_status"`
	Input             string `json:"input"`
	ContractAddress   string `json:"contractAddress"`
	CumulativeGasUsed string `json:"cumulativeGasUsed"`
	Confirmations     string `json:"confirmations"`
}
type BlockSummary struct {
	Number    uint64
	Timestamp uint64
	Miner     string
	TxCount   int
	GasUsed   uint64
	GasLimit  uint64
	BaseFee   *big.Int
}
type EtherscanResponse struct {
	Status  string        `json:"status"`
	Message string        `json:"message"`
	Result  []EtherscanTx `json:"result"`
}
type TxStats struct {
	Date              time.Time
	ContractCreations int
	EOAToEOA          int
	ToContracts       int
	Total             int
	TotalGas          *big.Int
	MaxTotalGas       *big.Int
	TotalBurned       *big.Int
}
type SuspiciousEntry struct {
	Address string
	Reason  string
}
type GasStats struct {
	TotalGas uint64
	AvgGas   uint64
}
type AddressActivity struct {
	Address string
	Count   int
}
type ContractInfo struct {
	Address  string
	IsCreate bool
}
type BlockAnalytics struct {
	Number_block uint64
	SummaryTx    int
	GasUsed      *big.Int
	Transaction  []TxJSON
	// TxStats      TxStats
	// Contracts    []ContractInfo
	// Suspicious   []SuspiciousEntry
	// TopAddresses []AddressActivity
}
type ClientSession struct {
	Conn         *websocket.Conn
	SubscribedTo map[string]struct{}
	Mux          sync.RWMutex
}
type SwapMove struct {
	Token       string  `json:"token"`
	TokenSymbol string  `json:"tokenSymbol,omitempty"`
	Amount      float64 `json:"amount"`
	USD         float64 `json:"usd"`
	IsNative    bool    `json:"isNative,omitempty"`
	IsStable    bool    `json:"isStable,omitempty"`
	In          bool    `json:"in"`
}
type MatchedTxInfo struct {
	// =========================
	// 🔹 EXISTING CORE (НЕ ЧІПАЄМО)
	// =========================
	TxHash     string `json:"txHash"`
	From       string `json:"from"`
	To         string `json:"to"`
	ValueEth   string `json:"valueEth"` // msg.value (wei)
	GasUsed    uint64 `json:"gasUsed"`
	GasPrice   string `json:"gasPrice"` // effective gas price (wei)
	Nonce      uint64 `json:"nonce"`
	RuleName   string `json:"rule_name"`
	RuleID     int64  `json:"ruleId"`
	UserID     int64  `json:"userId"`
	RuleTag    string `json:"ruleTag"`
	NativToken string `json:"nativToken"`
	// =========================
	// 🔹 EXISTING SWAP FIELDS (НЕ ЧІПАЄМО)
	// =========================
	Token  string `json:"token,omitempty"`  // ERC20 contract
	Amount string `json:"amount,omitempty"` // RAW uint256

	// =========================
	// 🔹 EXECUTION CONTEXT (NEW)
	// =========================
	BlockNumber uint64 `json:"blockNumber,omitempty"`
	Timestamp   uint64 `json:"timestamp,omitempty"` // unix seconds

	GasLimit uint64 `json:"gasLimit,omitempty"`
	GasCost  string `json:"gasCost,omitempty"` // gasUsed * gasPrice (wei)

	Status uint64 `json:"status,omitempty"` // 1 = success, 0 = revert

	// =========================
	// 🔹 TOKEN METADATA (NEW)
	// =========================
	TokenSymbol   string `json:"tokenSymbol,omitempty"`   // USDT, USDC, DAI
	TokenDecimals uint8  `json:"tokenDecimals,omitempty"` // 6, 18, ...

	// =========================
	// 🔹 NORMALIZED AMOUNTS (NEW)
	// =========================
	AmountHuman string `json:"amountHuman,omitempty"` // decimals applied

	// =========================
	// 🔹 FINANCIAL CONTEXT (NEW)
	// =========================
	PriceUSD string `json:"priceUsd,omitempty"` // token price
	ValueUSD string `json:"valueUsd,omitempty"` // amountHuman * price

	// =========================
	// 🔹 FLOW / ANALYTICS (NEW)
	// =========================
	Direction      string     `json:"direction,omitempty"`      // IN | OUT | MIXED
	TransfersCount int        `json:"transfersCount,omitempty"` // ERC20 Transfer logs count
	SwapIns        []SwapMove `json:"swapIns,omitempty"`        // ← ДОДАСИ В СТРУКТУРУ
	SwapOuts       []SwapMove `json:"swapOuts,omitempty"`
	SwapRoute      []string
}

type BlockStat struct {
	Timestamp time.Time
	TxCount   int
	GasUsed   *big.Int
}
type WindowStats struct {
	LastHour   []BlockStat
	Last24Hour []BlockStat
}
type SwapRule struct {
	Enabled   bool     `json:"enabled"`
	MinAmount string   `json:"minAmount"`
	Currency  string   `json:"currency"`
	Tokens    []string `json:"tokens"`
}
type TxJSON struct {
	Hash     string `json:"hash"`
	From     string `json:"from"`
	To       string `json:"to,omitempty"`
	Value    string `json:"value"`
	Gas      uint64 `json:"gas"`
	GasPrice string `json:"gas_price"`
	Nonce    uint64 `json:"nonce"`
	Input    string `json:"input,omitempty"`
	Type     uint8  `json:"type"`
}
type Alert struct {
	ID           int64     `json:"id"`
	RuleID       int64     `json:"rule_id"`
	TxHash       string    `json:"tx_hash"`
	ShortMessage string    `json:"address"`
	Details      string    `json:"details"`
	CreatedAt    time.Time `json:"created_at"`
}

// Хто і що хоче зловити
type ReactionParam struct {
	ID          string   `json:"id"`
	Creator     string   `json:"creator"`
	Address     string   `json:"address"`
	Direction   string   `json:"direction"`   // "to" | "from"
	MinValueWei string   `json:"minValueWei"` // мін. ETH у wei (рядок)
	MethodID    string   `json:"methodId"`    // опц., 0xa9059cbb
	Categories  []string `json:"categories"`  // ["nft_mint","nft_purchase","swap","erc20_transfer","native_transfer"]
	MinTokenRaw string   `json:"minTokenRaw"` // мін. сума токена у "raw" (uint256 як рядок)
}

// Що відправляємо автору параметра
type Match struct {
	ParamID string
	Creator string
	Reason  string
	Tx      *types.Transaction
}

// зберігаємо останні N результатів BlockAnalytics
type BA_Buffer struct {
	mu   sync.RWMutex
	data []BlockAnalytics
	cap  int
}

var StatsWindow = map[int]*WindowStats{
	1: {LastHour: []BlockStat{}, Last24Hour: []BlockStat{}}, // eth
	2: {LastHour: []BlockStat{}, Last24Hour: []BlockStat{}}, // bsc
	3: {LastHour: []BlockStat{}, Last24Hour: []BlockStat{}}, // base
	4: {LastHour: []BlockStat{}, Last24Hour: []BlockStat{}}, // polygon
	5: {LastHour: []BlockStat{}, Last24Hour: []BlockStat{}}, // mantle
}
var blockTimes = map[int]time.Duration{
	1: 12 * time.Second, // ETH
	2: 3 * time.Second,  // BSC
	3: 2 * time.Second,  // Base
	4: 2 * time.Second,  // Polygon
	5: 2 * time.Second,  // Mantle
}
var tokenEventChan = make(chan []TokenOnchainEvent, 1024)

func main() {

	if err := godotenv.Load(); err != nil {
		log.Println("⚠️ .env not loaded:", err)
	}

	// тримає процес живим

	dsn := os.Getenv("DB_DSN")
	if dsn == "" {
		dsn = "root:1111@tcp(127.0.0.1:3306)/mini-app?parseTime=true&charset=utf8mb4&loc=Local&interpolateParams=true"
	}
	err := InitDB(dsn)
	if err != nil {
		log.Fatal("❌ DB ERROR: ", err)
	}
	log.Println("✅ DB connected")
	if err := EnsureTokenAnalyticsSchema(); err != nil {
		log.Fatal("schema migration error: ", err)
	}
	log.Println("schema ensured: token analytics tables")
	LoadKnownAddressesIntoDB("./output.json")
	// Аналітика стартує лише якщо ENV-перемінна ENABLE_ONCHAIN_ANALYTICS встановлена

	StartTokenEventWriter()

	err1 := ImportAllTokens("./all_tokens.json")
	if err1 != nil {
		log.Printf("import from ./all_tokens.json failed: %v", err1)
		err1 = ImportAllTokens("/root/mini_ap_research/scanner/all_tokens.json")
	}
	if err1 != nil {
		log.Fatal(err1)
	}
	router := mux.NewRouter()
	router.HandleFunc("/api/address-analytics", AddressAnalyticsHandler).Methods(http.MethodPost, http.MethodOptions)
	// router.HandleFunc("/api/alerts_send", CreateAlertRuleHandler).Methods(http.MethodPost, http.MethodOptions)
	// router.HandleFunc("/api/alerts", ListAlertRuleHandler).Methods(http.MethodGet, http.MethodOptions)
	// router.HandleFunc("/api/token_prices", ListTokenPrice).Methods(http.MethodPost, http.MethodOptions)
	// router.HandleFunc("/ws", WsHandler) // WS окремо
	// USER
	router.HandleFunc("/api/init-user", InitUserHandler).Methods(http.MethodPost, http.MethodOptions)

	// PORTFOLIOS
	router.HandleFunc("/api/portfolios/create", CreatePortfolioHandler).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/portfolios", ListPortfoliosHandler).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/portfolio/token/add", AddTokenToPortfolioHandler).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/portfolio/tokens", ListPortfolioTokensHandler).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/portfolio/assets", HandleGetPortfolioAssets).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/portfolio/operation", RealizePortfolioOperationHandler).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/portfolio/anomaly-alerts", HandleTogglePortfolioAnomalyAlerts).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/portfolio/asset-tracking", HandleUpsertPortfolioAssetTracking).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/portfolio/asset-tracking/network", HandleUpsertPortfolioAssetNetworkTracking).Methods(http.MethodPost, http.MethodOptions)

	// ALERT RULES
	router.HandleFunc("/api/alert-rule/name", CreateRuleNameHandler).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/alert-rule", CreateAlertRuleHandler).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/alert-rules", ListAlertRulesHandler).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/alert-rule/filters", ListFiltersByRuleHandler).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/alerts/by-rule", ListAlertsByRuleHandler).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/alert-filter", UpdateAlertFilterHandler).Methods(http.MethodPut, http.MethodOptions)
	router.HandleFunc("/api/alert-filter", DeleteAlertFilterHandler).Methods(http.MethodDelete, http.MethodOptions)
	// rules
	router.HandleFunc("/api/alert-rule", DeleteAlertRuleHandler).Methods(http.MethodDelete, http.MethodOptions)
	// portfolios
	router.HandleFunc("/api/portfolio", DeletePortfolioHandler).Methods(http.MethodDelete, http.MethodOptions)

	// ALERTS
	router.HandleFunc("/api/alerts", ListUserAlertsHandler).Methods(http.MethodGet, http.MethodOptions)

	// TOKEN PRICES
	router.HandleFunc("/api/token-prices", ListTokenPrice).Methods(http.MethodPost, http.MethodOptions)
	router.HandleFunc("/api/market/top-active", HandleTopActiveTokensFront).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/market/token-activity", HandleMarketTokenActivity).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/token/dashboard", HandleTokenDashboardFront).Methods(http.MethodGet, http.MethodOptions)
	// Логування стану таблиць при старті
	AnalyticsHealthCheck()

	//////TokenParse
	router.HandleFunc("/api/token", GetTokenHandler).Methods(http.MethodGet, http.MethodOptions)

	router.HandleFunc("/api/token", UpsertTokenHandler).Methods(http.MethodPost, http.MethodOptions)
	///Onchain / Tokens
	router.HandleFunc("/api/onchain/tokens/top", HandleGetTopActiveTokens).Methods(http.MethodGet, http.MethodOptions)

	router.HandleFunc("/api/portfolio/{portfolio_id}/token/{token}", HandleGetPortfolioTokenDashboard).Methods(http.MethodGet, http.MethodOptions)

	router.HandleFunc("/api/token/subscription/daily-report", HandleToggleDailyReport).Methods(http.MethodPost, http.MethodOptions)

	router.HandleFunc("/api/portfolio/{portfolio_id}/notifications", HandleGetPortfolioNotifications).Methods(http.MethodGet, http.MethodOptions)
	router.HandleFunc("/api/token/activity", HandleTokenActivity).Methods("GET")
	router.HandleFunc("/api/token/hourly", HandleTokenHourly).Methods("GET")
	router.HandleFunc("/api/token/anomalies", HandleTokenAnomalies).Methods("GET")
	router.HandleFunc("/api/market/top-tokens", HandleTopActiveTokens).Methods("GET")

	// AGENT
	router.HandleFunc("/api/agent/signals", HandleAgentSignals).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/agent/strategy", HandleAgentStrategy).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/agent/position", HandleAgentPosition).Methods("GET", "OPTIONS")
	router.HandleFunc("/api/agent/trade-config", HandleAgentTradeConfig).Methods("GET", "POST", "OPTIONS")

	// ANALYTICS
	router.HandleFunc("/api/address-analytics", AddressAnalyticsHandler).Methods(http.MethodPost, http.MethodOptions)
	ethClient := ClientOpen()
	bscClient := ClientOpenBSC()
	mantleClient := ClientOpenMantle()

	router.HandleFunc("/api/stats", HandleStats).Methods("GET")
	// CORS Middleware
	c := cors.New(cors.Options{
		AllowedOrigins: []string{
			"http://localhost:3000",
			"http://127.0.0.1:3000",
			"https://hakato.app",
			"https://www.hakato.app",
		},
		AllowedMethods: []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders: []string{"Content-Type", "Authorization", "X-Requested-With"},
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go StartTokenPriceDaemon(ctx, ethClient, 1)       // Ethereum
	go StartTokenPriceDaemon(ctx, bscClient, 56)      // BSC
	go StartTokenPriceDaemon(ctx, mantleClient, 5000) // Mantle

	buf := NewBA_Buffer(7200)
	analyticsCh := make(chan BlockAnalytics, 64)
	analyticsBSC := make(chan BlockAnalytics, 64)
	analyticsMantle := make(chan BlockAnalytics, 64)

	// 1) крутиться активна аналітика (підписка + fallback)
	go RunActiveAnalytics(ctx, ethClient, analyticsCh, 1)
	go RunActiveAnalytics(ctx, bscClient, analyticsBSC, 56)
	go RunActiveAnalytics(ctx, mantleClient, analyticsMantle, 5000)
	StartUnifiedAnalyticsOrchestrator(ctx, AnalyticsWorkersConfig{
		EnrichInterval:      5 * time.Minute,
		DailyInterval:       10 * time.Minute,
		RuleGenInterval:     15 * time.Minute,
		EnrichLagHours:      48,  // materialize останні 48 закритих годин
		RuleLookbackHours:   168, // 7d baseline
		RuleMinSamplesHours: 24,  // мінімум 24 точки, інакше правило не оновлюємо
	})
	StartTokenMetadataUpdater(3 * time.Hour)

	// 2) класична «прокладка": усе, що прилітає — зберегти в буфер
	go func() {
		for {
			select {
			case <-ctx.Done():
				return

			case a := <-analyticsCh:
				buf.Add(a)
				ProcessBlockAnalytics(a, ethClient)

			case a := <-analyticsBSC:
				buf.Add(a)
				ProcessBlockAnalytics(a, bscClient)

			case a := <-analyticsMantle:
				buf.Add(a)
				ProcessBlockAnalytics(a, mantleClient)
			}
		}
	}()

	// Start strategy optimizer (runs every 6h for Mantle tokens)
	StartOptimizerDaemon()

	// Start agent executor (reads signals, writes on-chain)
	agentCfg := AgentConfigFromEnv()
	agentExec, err2 := NewAgentExecutor(agentCfg)
	if err2 != nil {
		log.Printf("⚠️ Agent executor init error: %v", err2)
	} else if agentExec != nil {
		go agentExec.Run(ctx)
		log.Println("🤖 Agent executor started")
	}

	log.Println("🚀 Telegram bot started (polling)")
	go StartPolling()

	handler := preflight(c.Handler(router))
	handler = LoggingMiddleware(handler)
	log.Println("🚀 Server running at http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", handler))

}

func StartScanner(chainID_DB int, client *ethclient.Client, chainId int) {
	bt := blockTimes[chainID_DB]
	wait := bt + 2*time.Second

	for {
		block := Scan_block(client, chainId)

		UpdateHourStats(chainID_DB, block)

		time.Sleep(wait)
	}
}
func ProcessBlockAnalytics(
	a BlockAnalytics,
	client *ethclient.Client,
) {
	// =========================
	// 1) ОНЧЕЙН-АНАЛІТИКА ТОКЕНІВ
	// =========================
	for _, tx := range a.Transaction {

		ctx, err := CheckTxHashForKnownAddresses(
			client,
			common.HexToHash(tx.Hash),
		)
		if err != nil {
			log.Printf("[TOKEN_ANALYTICS][ERR] Addresses tx=%s err=%v", tx.Hash, err)
			continue
		}

		if ctx == nil {
			continue
		}

		if err := AnalyzeTokenActivityFromTx(ctx); err != nil {
			log.Printf("[TOKEN_ANALYTICS][ERR] TokenActivity tx=%s err=%v", tx.Hash, err)
		}
	}

	// =========================
	// 2) RULE-BASED АНАЛІТИКА
	// =========================
	rs, resultAlerts := CurrentAlertRules()
	if len(rs) == 0 {
		// ❗ правил нема — але ончейн-аналітика ВЖЕ виконана
		return
	}

	stats, matches := GetTransactionStatsAndMatchesJSON_Rules(
		a.Transaction,
		rs,
		resultAlerts,
		client,
	)

	_ = stats // якщо не використовуєш — залишаємо

	for _, m := range matches {
		BroadcastAlertWS(a.Number_block, m)
	}
}

func preflight(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent) // 204
			return
		}
		next.ServeHTTP(w, r)
	})
}

func ClientOpen() *ethclient.Client {
	client, err := ethclient.Dial("https://g.w.lavanet.xyz:443/gateway/eth/rpc-http/b83c6e28f703373fc5685beb1f481138")
	if err != nil {
		log.Fatalf("Помилка підключення до ноди: %v", err)
	}

	return client
}

func ClientOpenBSC() *ethclient.Client {
	client, err := ethclient.Dial("https://g.w.lavanet.xyz:443/gateway/bsc/rpc-http/b83c6e28f703373fc5685beb1f481138")
	if err != nil {
		log.Fatalf("Помилка підключення до ноди: %v", err)
	}

	return client
}

func ClientOpenBase() *ethclient.Client {
	client, err := ethclient.Dial("https://g.w.lavanet.xyz:443/gateway/base/rpc-http/b83c6e28f703373fc5685beb1f481138")
	if err != nil {
		log.Fatalf("Помилка підключення до ноди: %v", err)
	}

	return client
}

func ClientOpenPol() *ethclient.Client {
	client, err := ethclient.Dial("https://g.w.lavanet.xyz:443/gateway/polygon/rpc-http/b83c6e28f703373fc5685beb1f481138")
	if err != nil {
		log.Fatalf("Помилка підключення до ноди: %v", err)
	}

	return client
}

func ClientOpenHype() *ethclient.Client {
	client, err := ethclient.Dial("https://g.w.lavanet.xyz:443/gateway/hyperliquid/rpc-http/b83c6e28f703373fc5685beb1f481138")
	if err != nil {
		log.Fatalf("Помилка підключення до ноди: %v", err)
	}

	return client
}

func ClientOpenMantle() *ethclient.Client {
	client, err := ethclient.Dial("https://rpc.mantle.xyz")
	if err != nil {
		log.Fatalf("Помилка підключення до Mantle: %v", err)
	}
	return client
}

func All_gas_Eth_EtherScan(address common.Address) *Result {
	type TxResponse struct {
		Status  string `json:"status"`
		Message string `json:"message"`
		Result  []struct {
			BlockNumber string `json:"blockNumber"`
			Hash        string `json:"hash"`
			From        string `json:"from"`
			To          string `json:"to"`
			Value       string `json:"value"`
			GasUsed     string `json:"gasUsed"`
			GasPrice    string `json:"gasPrice"`
		} `json:"result"`
	}

	apiKey := "JPRXBXFXFDHY5UG6US7JEUQ7EEHTNHQDGB"

	// 🔹 Etherscan API V2 (ETH mainnet = chainid 1)
	url := fmt.Sprintf(
		"https://api.etherscan.io/v2/api?chainid=1&module=account&action=txlist&address=%s&startblock=0&endblock=99999999&sort=asc&apikey=%s",
		address.Hex(),
		apiKey,
	)

	resp, err := http.Get(url)
	if err != nil {
		fmt.Println("HTTP error:", err)
		return nil
	}
	defer resp.Body.Close()

	var txs TxResponse
	if err := json.NewDecoder(resp.Body).Decode(&txs); err != nil {
		fmt.Println("JSON decode error:", err)
		return nil
	}

	if txs.Status != "1" {
		fmt.Println("Etherscan error:", txs.Message)
		return nil
	}

	file, err := os.Create("transactions.json")
	if err != nil {
		fmt.Println("File create error:", err)
		return nil
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", " ")
	_ = encoder.Encode(txs.Result)

	totalWei := new(big.Int)

	for _, tx := range txs.Result {
		gasUsed := new(big.Int)
		gasPrice := new(big.Int)

		gasUsed.SetString(tx.GasUsed, 10)
		gasPrice.SetString(tx.GasPrice, 10)

		cost := new(big.Int).Mul(gasUsed, gasPrice)
		totalWei.Add(totalWei, cost)
	}

	totalETH := WeiToEth(totalWei)

	result := Result{
		TotalGas: totalETH,
		TotalTx:  len(txs.Result),
	}

	fmt.Printf("Загальна сума газу: %s wei\n", totalWei.String())
	return &result
}

func Scan_block(client *ethclient.Client, chainId int) BlockAnalytics {

	startBlock, err := client.BlockByNumber(context.Background(), nil)
	if err != nil {
		log.Printf("❌ [%d] Block fetch error: %v", chainId, err)

		return BlockAnalytics{} // просто пропускаємо
	}
	TotalGasCostWei := new(big.Int).Mul(new(big.Int).SetUint64(startBlock.GasUsed()), startBlock.BaseFee())
	analytics := BlockAnalytics{
		Number_block: startBlock.NumberU64(),
		SummaryTx:    len(startBlock.Transactions()),
		GasUsed:      TotalGasCostWei,
		Transaction:  SaveBlockTransactions(startBlock, chainId),
	}

	return analytics

}

func ethStringToWei(s string) (*big.Int, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return big.NewInt(0), true
	}

	r := new(big.Rat)
	if _, ok := r.SetString(s); !ok { // <-- правильно обробляємо два результати
		return nil, false
	}

	// ETH -> wei: помножити на 1e18 і взяти підлогу
	scale := new(big.Int).Exp(big.NewInt(10), big.NewInt(18), nil) // 1e18
	weiRat := new(big.Rat).Mul(r, new(big.Rat).SetInt(scale))

	out := new(big.Int).Quo(weiRat.Num(), weiRat.Denom())
	return out, true
}

func IsDateScanned(date string, filename string) (bool, error) {
	// Ініціалізація історії
	history := make(map[string]bool)

	// Пробуємо прочитати файл
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// Якщо файл не існує — створюємо новий із поточною датою
			history[date] = true
			return false, writeHistory(filename, history)
		}
		return false, err // інші помилки
	}

	// Якщо файл існує, але порожній — теж ініціалізуємо
	if len(data) == 0 {
		history[date] = true
		return false, writeHistory(filename, history)
	}

	// Парсимо існуючу історію
	if err := json.Unmarshal(data, &history); err != nil {
		return false, fmt.Errorf("не вдалося розпарсити історію: %v", err)
	}

	// Перевіряємо дату
	if history[date] {
		return true, nil
	}

	// Дати ще немає — додаємо і зберігаємо
	history[date] = true
	return false, writeHistory(filename, history)
}

func writeHistory(filename string, history map[string]bool) error {
	newData, err := json.MarshalIndent(history, "", "  ")
	if err != nil {
		return fmt.Errorf("не вдалося серіалізувати історію: %v", err)
	}
	return os.WriteFile(filename, newData, 0644)
}

func SaveTxStatsToFile(stats TxStats, filename_string string, date_stats time.Time) error {
	filename := fmt.Sprintf("%s_%s.json", filename_string, date_stats.Format("2006-01-02"))

	// Створюємо структуру для збереження
	data, err := json.MarshalIndent(stats, "", "  ")
	if err != nil {
		return fmt.Errorf("помилка серіалізації: %v", err)
	}

	// Створення або перезапис файлу
	err = os.WriteFile(filename, data, 0644)
	if err != nil {
		return fmt.Errorf("помилка запису у файл: %v", err)
	}

	return nil
}

func WeiToEth(wei *big.Int) string {
	ethValue := new(big.Float).SetInt(wei)
	return new(big.Float).Quo(ethValue, big.NewFloat(1e18)).Text('f', 6)
}

func SaveBlockTransactions(startBlock *types.Block, chainId int) []TxJSON {
	txs := startBlock.Transactions()
	var parsed []TxJSON

	signer := types.LatestSignerForChainID(big.NewInt(int64(chainId)))

	for _, tx := range txs {
		from, err := signer.Sender(tx)
		if err != nil {
			log.Println("❌ signer.Sender error:", err)
			continue
		}

		to := ""
		if tx.To() != nil {
			to = tx.To().Hex()
		}

		txJson := TxJSON{
			Hash:     tx.Hash().Hex(),
			From:     from.Hex(),
			To:       to,
			Value:    WeiToEth(tx.Value()),
			Gas:      tx.Gas(),
			GasPrice: WeiToEth(tx.GasPrice()),
			Nonce:    tx.Nonce(),
			Input:    hex.EncodeToString(tx.Data()),
			Type:     tx.Type(),
		}

		parsed = append(parsed, txJson)
	}

	return parsed
}

func RunActiveAnalytics(ctx context.Context, client *ethclient.Client, out chan<- BlockAnalytics, chainId int) {
	// 1) пробуємо підписку на нові хедери
	trySubscribe := func() (*ethclient.Client, chan *types.Header, ethereum.Subscription, error) {
		headers := make(chan *types.Header, 64)
		sub, err := client.SubscribeNewHead(ctx, headers)
		return client, headers, sub, err
	}

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		_, headers, sub, err := trySubscribe()
		if err != nil {
			log.Printf("[analytics] subscribe failed: %v — fallback to polling", err)
			pollLoop(ctx, client, out, chainId)
			// якщо pollLoop повертається — спробуємо знову підписатися
			continue
		}
		id, _ := client.ChainID(ctx)
		head, _ := client.HeaderByNumber(ctx, nil)
		log.Printf("[rpc] chain=%v latest=%v", id, head.Number)

		// підписка працює
		for {
			select {
			case <-ctx.Done():
				if sub != nil {
					sub.Unsubscribe()
				}
				return

			case err := <-sub.Err():
				log.Printf("[analytics] subscription error: %v — restart", err)
				time.Sleep(1 * time.Second)
				goto RESUB

			case h := <-headers:
				if h == nil {
					continue
				}
				log.Printf("[ws] head #%d", h.Number.Uint64()) // ← ПРИЙШОВ ХЕДЕР
				blk, err := client.BlockByNumber(ctx, h.Number)
				if err != nil {
					log.Printf("[ws] failed to get block %d: %v", h.Number.Uint64(), err)
					continue
				}

				analytics := Scan_block_from_block(blk, chainId)

				log.Printf("[ws] built block=%d tx=%d", analytics.Number_block, analytics.SummaryTx)
				select {
				case out <- analytics:
					log.Printf("[ws] sent block=%d", analytics.Number_block)
				case <-ctx.Done():
					if sub != nil {
						sub.Unsubscribe()
					}
					return
				}

			case <-time.After(25 * time.Second):
				log.Println("[ws] still waiting for headers (25s)...") // ← ЖИВІ, але подій ще нема
			}
		}

	RESUB:
		time.Sleep(1 * time.Second)
		continue
	}
}

// pollLoop — простий опитувач «останнього блоку» раз на 2с (коли підписки нема або впала).
func pollLoop(ctx context.Context, client *ethclient.Client, out chan<- BlockAnalytics, chainId int) {
	var last uint64
	blockFetchFailures := make(map[uint64]int)
	const maxBlockFetchFailures = 3

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			// 1️⃣ дізнаємось latest block number
			head, err := client.HeaderByNumber(ctx, nil)
			if err != nil {
				log.Printf("[analytics] polling head error: %v", err)
				continue
			}

			latest := head.Number.Uint64()

			// 2️⃣ ініціалізація last (перший запуск)
			if last == 0 {
				last = latest
				continue
			}

			// 3️⃣ якщо немає нових блоків
			if latest <= last {
				continue
			}

			// 4️⃣ обробляємо ВСІ блоки по порядку
			for n := last + 1; n <= latest; n++ {
				select {
				case <-ctx.Done():
					return
				default:
				}

				blk, err := client.BlockByNumber(ctx, new(big.Int).SetUint64(n))
				if err != nil {
					blockFetchFailures[n]++
					if blockFetchFailures[n] >= maxBlockFetchFailures {
						log.Printf("[analytics][WARN] skipping block %d after %d fetch failures: %v", n, blockFetchFailures[n], err)
						delete(blockFetchFailures, n)
						last = n
						continue
					}
					log.Printf("[analytics] failed to get block %d (attempt %d/%d): %v", n, blockFetchFailures[n], maxBlockFetchFailures, err)
					break // НЕ рухаємо last — доганяємо наступного тіку
				}

				delete(blockFetchFailures, n)
				analytics := Scan_block_from_block(blk, chainId)

				select {
				case out <- analytics:
					last = n // ✅ тільки після успішної відправки
				case <-ctx.Done():
					return
				}
			}
		}
	}
}

func Scan_block_from_block(startBlock *types.Block, chainID int) BlockAnalytics {
	// Беремо твою формулу загальних витрат gas (захистимося від nil BaseFee)
	baseFee := startBlock.BaseFee()
	if baseFee == nil {
		baseFee = big.NewInt(0)
	}
	totalGasCostWei := new(big.Int).Mul(new(big.Int).SetUint64(startBlock.GasUsed()), baseFee)

	return BlockAnalytics{
		Number_block: startBlock.NumberU64(),
		SummaryTx:    len(startBlock.Transactions()),
		GasUsed:      totalGasCostWei,
		Transaction:  SaveBlockTransactions(startBlock, chainID), // твоя існуюча функція
	}
}

func NewBA_Buffer(capacity int) *BA_Buffer {
	return &BA_Buffer{cap: capacity, data: make([]BlockAnalytics, 0, capacity)}
}

func (b *BA_Buffer) Add(x BlockAnalytics) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if len(b.data) == b.cap {
		b.data = b.data[1:]
	}
	b.data = append(b.data, x)
}

func (b *BA_Buffer) Snapshot() []BlockAnalytics {
	b.mu.RLock()
	defer b.mu.RUnlock()
	out := make([]BlockAnalytics, len(b.data))
	copy(out, b.data)
	return out
}

// /update price token
const TokenPriceTTL = 60 * time.Second // TTL актуальності
const DaemonTick = 5 * time.Second
const MaxUpdatesPerTick = 3

var StableOracleFeeds = map[int]map[string]common.Address{
	1: { // Ethereum

		// USDT
		strings.ToLower("0xdAC17F958D2ee523a2206206994597C13D831ec7"): common.HexToAddress("0x3E7d1eAB13ad0104d2750B8863b489D65364e32D"),

		// USDC
		strings.ToLower("0xA0b86991c6218b36c1d19D4a2e9Eb0cE3606e48"): common.HexToAddress("0x8fFfFfd4AfB6115b954Bd326cbe7B4BA576818f6"),

		// DAI
		strings.ToLower("0x6B175474E89094C44Da98b954EedeAC495271d0F"): common.HexToAddress("0xAed0c38402a5d19df6E4c03F4E2DceD6e29c1ee9"),

		// TUSD
		strings.ToLower("0x0000000000085d4780b73119b644ae5ecd22b376"): common.HexToAddress("0x3886BA987236181D98F2401c507Fb8BeA7871dF2"),

		// LUSD
		strings.ToLower("0x5f98805a4e8be255a32880fdec7f6728c6568ba0"): common.HexToAddress("0x3D7aE7E594f2f2091Ad8798313450130fC5c7d6E"),

		// FRAX
		strings.ToLower("0x853d955acef822db058eb8505911ed77f175b99e"): common.HexToAddress("0xB9E38A61cF6c2C3dC6F3E0c4A27CAd1C9F0a3f05"),
	},

	56: { // BSC

		// USDT (BEP20)
		strings.ToLower("0x55d398326f99059fF775485246999027B3197955"): common.HexToAddress("0xB97Ad0E74fa7d920791E90258A6E2085088b4320"),

		// USDC (BEP20)
		strings.ToLower("0x8AC76a51cc950d9822D68b83fE1Ad97B32Cd580d"): common.HexToAddress("0x51597f405303C4377E36123cBc172b13269EA163"),

		// BUSD (legacy, але oracle ще активний)
		strings.ToLower("0xe9e7cea3dedca5984780bafc599bd69add087d56"): common.HexToAddress("0xcBb98864Ef56E9042e7d2efef76141f15731B82f"),

		// FDUSD
		strings.ToLower("0x5f9e8f1d3e0a0e95c5f5b80c2e4d4aeb6e8d87f5"): common.HexToAddress("0x51597f405303C4377E36123cBc172b13269EA163"),
	},
}

var priceSingleflight singleflight.Group

func RefreshTokenPricesFromDB(client *ethclient.Client) {
	tokens, err := DB_GetAllTokenPrices()
	if err != nil {
		log.Printf("DB_GetAllTokenPrices error: %v", err)
		return
	}

	for _, t := range tokens {
		// перевірка актуальності
		if time.Since(t.UpdatedAt) <= TokenPriceTTL {
			continue
		}

		// отримуємо нову ціну
		price, err := GetTokenPriceUS(t.Contract)
		if err != nil {
			log.Printf("price fetch error %s: %v", t.Contract, err)
			continue
		}

		// символ (якщо вже є — не чіпаємо)
		symbol := t.Symbol
		if symbol == "" || symbol == "UNKNOWN" {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)

			s, err := GetTokenSymbolSafe(ctx, client, common.HexToAddress(t.Contract))
			cancel()
			if err == nil && s != "" {
				symbol = s
			}
		}

		// оновлюємо БД
		if err := DB_UpdateTokenPrice(t.ChainID, t.Contract, symbol, price); err != nil {
			log.Printf("DB_UpdateTokenPrice error %s: %v", t.Contract, err)
		}
	}
}
func RefreshSingleToken(ctx context.Context, client *ethclient.Client, t TokenPriceRow) {
	if time.Since(t.UpdatedAt) < TokenPriceTTL {
		return
	}

	ctx2, cancel := context.WithTimeout(ctx, 6*time.Second)
	defer cancel()

	price, _, err := FetchTokenPriceSingleflight(
		ctx2,
		client,
		t.ChainID,
		t.Contract,
	)
	if err != nil || price <= 0 {
		if strings.Contains(err.Error(), "no active liquidity") {
			log.Printf("[price][WARN] no liquidity for %s — touch only",
				t.Contract,
			)
			_ = DB_TouchTokenPrice(t.ChainID, t.Contract)
			return
		}

		log.Printf("[price] failed %s: %v", t.Contract, err)
		return
	}

	// symbol тільки якщо відсутній
	symbol := t.Symbol
	if symbol == "" || symbol == "UNKNOWN" {
		s, err := GetTokenSymbolSafe(ctx2, client, common.HexToAddress(t.Contract))
		if err == nil && s != "" {
			symbol = s
		}
	}
	if !isValidPrice(price) {
		log.Printf("[price][WARN] invalid price %.6f for %s — touch only",
			price, t.Contract,
		)

		_ = DB_TouchTokenPrice(t.ChainID, t.Contract)
		return
	}
	if err := DB_UpdateTokenPrice(t.ChainID, t.Contract, symbol, price); err != nil {
		log.Printf("[price][ERR] DB update failed %s: %v", t.Contract, err)
		return
	}
	// log.Printf("[price] %s %s = %.4f (%s)",
	// 	t.Contract, symbol, price, source)
}
func StartTokenPriceDaemon(ctx context.Context, client *ethclient.Client, chainID int) {

	ticker := time.NewTicker(DaemonTick)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-ticker.C:
			tokens, err := DB_GetTokensForRefresh(MaxUpdatesPerTick)

			if err != nil {
				continue
			}

			for _, t := range tokens {
				if t.ChainID != chainID {
					continue
				}
				RefreshSingleToken(ctx, client, t)
			}
		}
	}
}
func FetchTokenPriceSingleflight(ctx context.Context, client *ethclient.Client, chainID int, contract string) (float64, string, error) {

	key := fmt.Sprintf("%d:%s", chainID, strings.ToLower(contract))

	v, err, _ := priceSingleflight.Do(key, func() (any, error) {
		price, source, err := ResolveTokenPriceUSD(ctx, client, chainID, contract)
		if err != nil {
			return nil, err
		}
		return struct {
			Price  float64
			Source string
		}{price, source}, nil
	})

	if err != nil {
		return 0, "", err
	}

	r := v.(struct {
		Price  float64
		Source string
	})

	return r.Price, r.Source, nil
}
func ResolveTokenPriceUSD(ctx context.Context, client *ethclient.Client, chainID int, contract string) (price float64, source string, err error) {

	// 1️⃣ Native token
	if contract == "native" {

		price, err = GetNativePriceUSD(int64(chainID), client)
		return price, "native_oracle", err
	}

	// 2️⃣ Stablecoin (через oracle)
	if _, ok := StableOracleFeeds[chainID][strings.ToLower(contract)]; ok {
		price, err = GetStablePriceUSD(ctx, client, chainID, contract)
		return price, "stable_oracle", err
	}

	// 3️⃣ ERC20 (DEX)
	price, err = GetTokenPriceUSChain(ctx, chainID, contract)
	return price, "dex", err
}
func GetStablePriceUSD(ctx context.Context, client *ethclient.Client, chainID int, contract string) (float64, error) {

	feeds := StableOracleFeeds[chainID]
	if feeds == nil {
		return 0, errors.New("no oracle feeds for chain")
	}

	feed, ok := feeds[strings.ToLower(contract)]
	if !ok {
		return 0, errors.New("no oracle feed for token")
	}

	return ReadChainlinkPrice(ctx, client, feed)
}
func GetTokenPriceUSChain(ctx context.Context, chainID int, tokenAddr string) (float64, error) {
	switch chainID {

	case 1: // ETH
		return GetTokenPriceUS(tokenAddr)

	case 5000: // Mantle — fallback to ETH price feed for mETH; others via DEX
		return GetTokenPriceUS(tokenAddr)

	default:
		return 0, ErrNoLiquidity
	}
}
func isValidPrice(price float64) bool {
	if price <= 0 {
		return false
	}
	if price > 1_000_000 { // верхня межа здорового глузду
		return false
	}
	return true
}

// //////
func ProcessTokenAsync(chainID int, tokenPriceID int64, contract string, ctx context.Context) {
	go func() {
		var price float64
		var err error
		client := ClientOpenByChain(chainID)

		switch {
		case contract == "native":
			price, err = GetNativePriceUSD(int64(chainID), client)

		case contract == "btc":
			price, err = GetBTCPriceUSD(client)

		case IsStableToken(chainID, contract):
			price, err = GetStablePriceUSD(ctx, client, chainID, contract)

		default:
			price, err = GetTokenPriceUSChain(ctx, chainID, contract)
		}

		if err != nil || price <= 0 {
			rollbackTokenStub(tokenPriceID, contract)
			return
		}
		log.Printf("priceToken %v", price)
		// ⚠️ якщо ClientOpen() також ETH-only — для BSC теж треба вибір по chainID

		symbol, err := ResolveTokenSymbolOnly(ctx, chainID, contract, client)
		if err != nil || symbol == "" {
			rollbackTokenStub(tokenPriceID, contract)
			return
		}

		_, _ = DB.Exec(`
			UPDATE token_prices
			SET symbol = ?, price_usd = ?, updated_at = NOW()
			WHERE id = ?
		`, symbol, price, tokenPriceID)
	}()
}
func EnsureTokenPrice(chainID int, contract string, ctx context.Context) (int64, error) {
	contract = strings.ToLower(contract)

	// 1️⃣ перевірка
	id, err := DB_GetTokenPriceID(chainID, contract)
	if err == nil {
		return id, nil // ✔️ вже є
	}

	if err != sql.ErrNoRows {
		// ❌ реальна помилка БД
		return 0, err
	}

	// 2️⃣ СИНХРОННИЙ insert заглушки
	id, err = InsertTokenStub(chainID, contract)
	if err != nil {
		return 0, err
	}

	// 3️⃣ async обробка (ціна + symbol)
	ProcessTokenAsync(chainID, id, contract, ctx)

	return id, nil
}
func ResolveTokenSymbolOnly(ctx context.Context, chainID int, contract string, client *ethclient.Client) (string, error) {
	c := strings.ToLower(strings.TrimSpace(contract))

	// BTC (у тебе може бути "btc" або "BTC")
	if c == "btc" {
		return "BTC", nil
	}

	// Native (у тебе в БД "native")
	if c == "native" {
		switch chainID {
		case 1:
			return "ETH", nil
		case 56:
			return "BNB", nil
		case 137:
			return "MATIC", nil
		case 8453:
			return "BASE", nil
		case 5000:
			return "MNT", nil
		default:
			return "NATIVE", nil
		}
	}

	// Mantle known tokens
	if chainID == 5000 {
		switch c {
		case strings.ToLower("0xcDA86A272531e8640cD7F1a92c01839911B90bb0"):
			return "mETH", nil
		case strings.ToLower("0x5bE26527e817998A7206475496fDE1E68957c5A6"):
			return "USDY", nil
		case strings.ToLower("0x09Bc4E0D864854c6aFB6eB9A9cdF58aC190D0dF9"):
			return "USDC", nil
		case strings.ToLower("0x201EBa5CC46D216Ce6DC03F6a759e8E766e956aE"):
			return "USDT", nil
		}
	}

	// Stablecoins (інлайн-мапа, без IsStableToken / KnownStableSymbol)
	switch chainID {
	case 1: // Ethereum
		switch c {
		case strings.ToLower("0xa0b86991c6218b36c1d19d4a2e9eb0ce3606e48"):
			return "USDC", nil
		case strings.ToLower("0xdac17f958d2ee523a2206206994597c13d831ec7"):
			return "USDT", nil
		case strings.ToLower("0x6b175474e89094c44da98b954eedeac495271d0f"):
			return "DAI", nil
		}
	case 56: // BSC
		switch c {
		case strings.ToLower("0x55d398326f99059ff775485246999027b3197955"):
			return "USDT", nil
		case strings.ToLower("0x8ac76a51cc950d9822d68b83fe1ad97b32cd580d"):
			return "USDC", nil
		}
	}

	// ERC20 (єдина існуюча реалізація — GetTokenSymbolSafe)
	symbol, err := GetTokenSymbolSafe(ctx, client, common.HexToAddress(contract))
	if err != nil || symbol == "" {
		log.Printf("[token][WARN] symbol unresolved for %s: %v", contract, err)
		return "____", err
	}
	return symbol, nil
}

func almostZero(v float64) bool {
	if v < 0 {
		v = -v
	}
	return v < 1e-12
}

func ClientOpenByChain(chainID int) *ethclient.Client {
	switch chainID {
	case 1:
		return ClientOpen()
	case 56:
		return ClientOpenBSC()
	case 8453:
		return ClientOpenBase()
	case 137:
		return ClientOpenPol()
	case 999:
		return ClientOpenHype()
	case 5000:
		return ClientOpenMantle()
	default:
		return ClientOpen()
	}
}
func GetTokenSymbolWithFallback(ctx context.Context, chainID int, client *ethclient.Client, tokenAddr string) string {

	var symbol sql.NullString
	_ = DB.QueryRow(
		`SELECT symbol FROM token_prices WHERE chain_id = ? AND contract = ? LIMIT 1`,
		chainID, tokenAddr,
	).Scan(&symbol)

	if symbol.Valid && symbol.String != "" {
		return symbol.String
	}

	s, err := GetTokenSymbolSafe(ctx, client, common.HexToAddress(tokenAddr))
	if err == nil && s != "" {
		return s
	}

	return tokenAddr[:6] + "…"
}
func IsStableToken(chainID int, contract string) bool {
	feeds, ok := StableOracleFeeds[chainID]
	if !ok {
		return false
	}

	_, ok = feeds[strings.ToLower(contract)]
	return ok
}

func StartTokenEventWriter() {
	go func() {
		const (
			batchLimit     = 120
			flushInterval  = 350 * time.Millisecond
			warnPendingLen = 6000
		)

		ticker := time.NewTicker(flushInterval)
		defer ticker.Stop()

		pending := make([]TokenOnchainEvent, 0, batchLimit*2)

		flushOne := func(n int) {
			if n <= 0 || len(pending) == 0 {
				return
			}
			if n > len(pending) {
				n = len(pending)
			}

			batch := pending[:n]
			for {
				err := DB_InsertTokenTransferEvents(batch, nil)
				if err == nil {
					break
				}
				log.Printf("[token-writer][ERR] batch=%d err=%v; retry in 1s", len(batch), err)
				time.Sleep(1 * time.Second)
			}

			pending = pending[n:]
		}

		for {
			select {
			case events, ok := <-tokenEventChan:
				if !ok {
					for len(pending) > 0 {
						flushOne(batchLimit)
					}
					return
				}

				if len(events) == 0 {
					continue
				}

				pending = append(pending, events...)
				if len(pending) > warnPendingLen {
					log.Printf("[token-writer][WARN] pending queue=%d", len(pending))
				}

				if len(pending) >= batchLimit*3 {
					flushOne(batchLimit)
				}

			case <-ticker.C:
				flushOne(batchLimit)
			}
		}
	}()
}
