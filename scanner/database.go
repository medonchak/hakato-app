package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math/big"

	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

var DB *sql.DB

// ======================================================
//  INIT DB
//  Викликаєш у своєму main.go:
//  err := InitDB("user:pass@tcp(127.0.0.1:3306)/dbname?parseTime=true&charset=utf8mb4&loc=Local")
// ======================================================

func InitDB(dsn string) error {
	var err error
	DB, err = sql.Open("mysql", dsn)
	if err != nil {
		return err
	}
	// Protect MySQL from connection storms under parallel analytics workers.
	DB.SetMaxOpenConns(20)
	DB.SetMaxIdleConns(10)
	DB.SetConnMaxIdleTime(2 * time.Minute)
	DB.SetConnMaxLifetime(15 * time.Minute)
	return DB.Ping()
}

type schemaColumn struct {
	name string
	def  string
}

type schemaIndex struct {
	name string
	def  string
}

func EnsureTokenAnalyticsSchema() error {
	if DB == nil {
		return errors.New("db not initialized")
	}

	if err := EnsurePortfolioTrackingSchema(); err != nil {
		return err
	}
	if err := ensureTokenHourlyActivitySchema(); err != nil {
		return err
	}
	if err := ensureTokenHourlyMetricsSchema(); err != nil {
		return err
	}
	if err := ensureAgentTablesSchema(); err != nil {
		return err
	}
	return nil
}

func ensureAgentTablesSchema() error {
	tables := []string{
		`CREATE TABLE IF NOT EXISTS agent_strategies (
  chain_id BIGINT NOT NULL,
  token VARCHAR(42) NOT NULL,
  token_symbol VARCHAR(20) NOT NULL DEFAULT '',
  vwap_period INT NOT NULL DEFAULT 12,
  buy_threshold_pct DOUBLE NOT NULL DEFAULT -2.0,
  sell_threshold_pct DOUBLE NOT NULL DEFAULT 3.0,
  cooldown_hours INT NOT NULL DEFAULT 2,
  sharpe DOUBLE NOT NULL DEFAULT 0,
  win_rate DOUBLE NOT NULL DEFAULT 0,
  total_trades INT NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (chain_id, token)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS agent_signals (
  id BIGINT NOT NULL AUTO_INCREMENT,
  chain_id BIGINT NOT NULL,
  token VARCHAR(42) NOT NULL,
  token_symbol VARCHAR(20) NOT NULL DEFAULT '',
  signal_type ENUM('BUY','SELL','HOLD') NOT NULL,
  reason TEXT,
  confidence DOUBLE NOT NULL DEFAULT 0,
  price_usd DOUBLE NOT NULL DEFAULT 0,
  vwap DOUBLE NOT NULL DEFAULT 0,
  size_usd DOUBLE NULL,
  tx_hash VARCHAR(66) NULL,
  on_chain_id BIGINT NULL COMMENT 'SignalRegistry.sol signal id',
  executed TINYINT(1) NOT NULL DEFAULT 0,
  created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
  PRIMARY KEY (id),
  KEY idx_agent_signals_token (chain_id, token, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS agent_positions (
  chain_id BIGINT NOT NULL,
  token VARCHAR(42) NOT NULL,
  token_symbol VARCHAR(20) NOT NULL DEFAULT '',
  size_usd DOUBLE NOT NULL DEFAULT 0,
  entry_price DOUBLE NOT NULL DEFAULT 0,
  pnl_usd DOUBLE NOT NULL DEFAULT 0,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (chain_id, token)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,

		`CREATE TABLE IF NOT EXISTS agent_trade_config (
  chain_id BIGINT NOT NULL,
  wallet_addr VARCHAR(42) NOT NULL DEFAULT '',
  trade_token VARCHAR(20) NOT NULL DEFAULT 'USDC',
  amount_usd DOUBLE NOT NULL DEFAULT 5,
  updated_at DATETIME NOT NULL,
  PRIMARY KEY (chain_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`,
	}

	for _, ddl := range tables {
		if _, err := DB.Exec(ddl); err != nil {
			return fmt.Errorf("ensureAgentTablesSchema: %w", err)
		}
	}
	return nil
}

func ensureTokenHourlyActivitySchema() error {
	const tableName = "token_hourly_activity"
	createSQL := `
CREATE TABLE IF NOT EXISTS token_hourly_activity (
  chain_id BIGINT NOT NULL,
  token CHAR(42) NOT NULL,
  hour_ts BIGINT NOT NULL,
  transfer_count BIGINT NOT NULL DEFAULT 0,
  total_volume_raw DECIMAL(65,0) NOT NULL DEFAULT 0,
  exchange_in_raw DECIMAL(65,0) NOT NULL DEFAULT 0,
  exchange_out_raw DECIMAL(65,0) NOT NULL DEFAULT 0,
  max_transfer_raw DECIMAL(65,0) NOT NULL DEFAULT 0,
  total_volume_usd DOUBLE NULL,
  exchange_in_usd DOUBLE NULL,
  exchange_out_usd DOUBLE NULL,
  max_transfer_usd DOUBLE NULL,
  exchange_transfer_count BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (chain_id, token, hour_ts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	requiredCols := []schemaColumn{
		{name: "chain_id", def: "chain_id BIGINT NOT NULL"},
		{name: "token", def: "token CHAR(42) NOT NULL"},
		{name: "hour_ts", def: "hour_ts BIGINT NOT NULL"},
		{name: "transfer_count", def: "transfer_count BIGINT NOT NULL DEFAULT 0"},
		{name: "total_volume_raw", def: "total_volume_raw DECIMAL(65,0) NOT NULL DEFAULT 0"},
		{name: "exchange_in_raw", def: "exchange_in_raw DECIMAL(65,0) NOT NULL DEFAULT 0"},
		{name: "exchange_out_raw", def: "exchange_out_raw DECIMAL(65,0) NOT NULL DEFAULT 0"},
		{name: "max_transfer_raw", def: "max_transfer_raw DECIMAL(65,0) NOT NULL DEFAULT 0"},
		{name: "total_volume_usd", def: "total_volume_usd DOUBLE NULL"},
		{name: "exchange_in_usd", def: "exchange_in_usd DOUBLE NULL"},
		{name: "exchange_out_usd", def: "exchange_out_usd DOUBLE NULL"},
		{name: "max_transfer_usd", def: "max_transfer_usd DOUBLE NULL"},
		{name: "exchange_transfer_count", def: "exchange_transfer_count BIGINT NOT NULL DEFAULT 0"},
		{name: "updated_at", def: "updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"},
	}

	requiredIdx := []schemaIndex{
		{name: "idx_tha_hour", def: "ADD INDEX idx_tha_hour (hour_ts)"},
		{name: "idx_tha_chain_hour", def: "ADD INDEX idx_tha_chain_hour (chain_id, hour_ts)"},
		{name: "idx_tha_token_hour", def: "ADD INDEX idx_tha_token_hour (token, hour_ts)"},
	}

	return ensureTableSchema(tableName, createSQL, requiredCols, requiredIdx)
}

func ensureTokenHourlyMetricsSchema() error {
	const tableName = "token_hourly_metrics"
	createSQL := `
CREATE TABLE IF NOT EXISTS token_hourly_metrics (
  chain_id BIGINT NOT NULL,
  token CHAR(42) NOT NULL,
  hour_ts BIGINT NOT NULL,
  transfers BIGINT NOT NULL DEFAULT 0,
  unique_senders BIGINT NOT NULL DEFAULT 0,
  unique_receivers BIGINT NOT NULL DEFAULT 0,
  unique_addresses BIGINT NOT NULL DEFAULT 0,
  p50_raw DECIMAL(65,0) NULL,
  p95_raw DECIMAL(65,0) NULL,
  p99_raw DECIMAL(65,0) NULL,
  p50_usd DOUBLE NULL,
  p95_usd DOUBLE NULL,
  p99_usd DOUBLE NULL,
  top1_addr_share DOUBLE NULL,
  top3_addr_share DOUBLE NULL,
  top5_addr_share DOUBLE NULL,
  exchange_share DOUBLE NULL,
  net_exchange_usd DOUBLE NULL,
  usd_lt_100 BIGINT NOT NULL DEFAULT 0,
  usd_100_1k BIGINT NOT NULL DEFAULT 0,
  usd_1k_10k BIGINT NOT NULL DEFAULT 0,
  usd_10k_100k BIGINT NOT NULL DEFAULT 0,
  usd_gt_100k BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (chain_id, token, hour_ts)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	requiredCols := []schemaColumn{
		{name: "chain_id", def: "chain_id BIGINT NOT NULL"},
		{name: "token", def: "token CHAR(42) NOT NULL"},
		{name: "hour_ts", def: "hour_ts BIGINT NOT NULL"},
		{name: "transfers", def: "transfers BIGINT NOT NULL DEFAULT 0"},
		{name: "unique_senders", def: "unique_senders BIGINT NOT NULL DEFAULT 0"},
		{name: "unique_receivers", def: "unique_receivers BIGINT NOT NULL DEFAULT 0"},
		{name: "unique_addresses", def: "unique_addresses BIGINT NOT NULL DEFAULT 0"},
		{name: "p50_raw", def: "p50_raw DECIMAL(65,0) NULL"},
		{name: "p95_raw", def: "p95_raw DECIMAL(65,0) NULL"},
		{name: "p99_raw", def: "p99_raw DECIMAL(65,0) NULL"},
		{name: "p50_usd", def: "p50_usd DOUBLE NULL"},
		{name: "p95_usd", def: "p95_usd DOUBLE NULL"},
		{name: "p99_usd", def: "p99_usd DOUBLE NULL"},
		{name: "top1_addr_share", def: "top1_addr_share DOUBLE NULL"},
		{name: "top3_addr_share", def: "top3_addr_share DOUBLE NULL"},
		{name: "top5_addr_share", def: "top5_addr_share DOUBLE NULL"},
		{name: "exchange_share", def: "exchange_share DOUBLE NULL"},
		{name: "net_exchange_usd", def: "net_exchange_usd DOUBLE NULL"},
		{name: "usd_lt_100", def: "usd_lt_100 BIGINT NOT NULL DEFAULT 0"},
		{name: "usd_100_1k", def: "usd_100_1k BIGINT NOT NULL DEFAULT 0"},
		{name: "usd_1k_10k", def: "usd_1k_10k BIGINT NOT NULL DEFAULT 0"},
		{name: "usd_10k_100k", def: "usd_10k_100k BIGINT NOT NULL DEFAULT 0"},
		{name: "usd_gt_100k", def: "usd_gt_100k BIGINT NOT NULL DEFAULT 0"},
		{name: "updated_at", def: "updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"},
	}

	requiredIdx := []schemaIndex{
		{name: "idx_thm_hour", def: "ADD INDEX idx_thm_hour (hour_ts)"},
		{name: "idx_thm_chain_hour", def: "ADD INDEX idx_thm_chain_hour (chain_id, hour_ts)"},
		{name: "idx_thm_token_hour", def: "ADD INDEX idx_thm_token_hour (token, hour_ts)"},
	}

	return ensureTableSchema(tableName, createSQL, requiredCols, requiredIdx)
}

func ensureTableSchema(tableName, createSQL string, requiredCols []schemaColumn, requiredIdx []schemaIndex) error {
	if _, err := DB.Exec(createSQL); err != nil {
		return fmt.Errorf("create table %s: %w", tableName, err)
	}

	for _, col := range requiredCols {
		ok, err := columnExists(tableName, col.name)
		if err != nil {
			return fmt.Errorf("check column %s.%s: %w", tableName, col.name, err)
		}
		if ok {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s", tableName, col.def)
		if _, err := DB.Exec(stmt); err != nil {
			return fmt.Errorf("add column %s.%s: %w", tableName, col.name, err)
		}
		log.Printf("schema: added column %s.%s", tableName, col.name)
	}

	for _, idx := range requiredIdx {
		ok, err := indexExists(tableName, idx.name)
		if err != nil {
			return fmt.Errorf("check index %s.%s: %w", tableName, idx.name, err)
		}
		if ok {
			continue
		}
		stmt := fmt.Sprintf("ALTER TABLE %s %s", tableName, idx.def)
		if _, err := DB.Exec(stmt); err != nil {
			return fmt.Errorf("add index %s.%s: %w", tableName, idx.name, err)
		}
		log.Printf("schema: added index %s.%s", tableName, idx.name)
	}

	return nil
}

func columnExists(tableName, columnName string) (bool, error) {
	var cnt int
	err := DB.QueryRow(`
SELECT COUNT(*)
FROM information_schema.COLUMNS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = ?
  AND COLUMN_NAME = ?`,
		tableName, columnName,
	).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

func indexExists(tableName, indexName string) (bool, error) {
	var cnt int
	err := DB.QueryRow(`
SELECT COUNT(*)
FROM information_schema.STATISTICS
WHERE TABLE_SCHEMA = DATABASE()
  AND TABLE_NAME = ?
  AND INDEX_NAME = ?`,
		tableName, indexName,
	).Scan(&cnt)
	if err != nil {
		return false, err
	}
	return cnt > 0, nil
}

// ======================================================
//
//	USERS (таблиця users)
//	id INT PK, telegram_id BIGINT UNIQUE, username, first_name, created_at
//
// ======================================================
type TokenPriceRow struct {
	ChainID   int
	Contract  string
	Symbol    string
	PriceUSD  float64
	UpdatedAt time.Time
}

type User struct {
	ID         int64
	TelegramID int64
	Username   *string
	FirstName  *string
	CreatedAt  time.Time
}
type PortfolioOperationRequest struct {
	Type        string `json:"type"`
	PortfolioID int64  `json:"portfolioId"`

	From     OperationFrom      `json:"from"`
	To       *OperationTo       `json:"to,omitempty"`
	NewToken *OperationNewToken `json:"newToken,omitempty"`
}
type OperationFrom struct {
	TokenID       int64   `json:"tokenId"`
	AmountDelta   float64 `json:"amountDelta"`   // відʼємне число
	RealizedDelta float64 `json:"realizedDelta"` // +USD
}
type OperationTo struct {
	TokenID       int64   `json:"tokenId"`
	AmountDelta   float64 `json:"amountDelta"`   // +token qty
	InvestedDelta float64 `json:"investedDelta"` // +USD
}
type OperationNewToken struct {
	Contract string  `json:"contract"`
	Amount   float64 `json:"amount"`
	Invested float64 `json:"invested"`
}

// Якщо юзер існує — повертаємо його id, якщо ні — створюємо.
func DB_GetOrCreateUser(telegramID int64, username, firstName string) (int64, error) {
	if DB == nil {
		return 0, errors.New("db not initialized")
	}

	var id int64
	err := DB.QueryRow(`SELECT id FROM users WHERE telegram_id = ?`, telegramID).Scan(&id)
	if err == nil {
		return id, nil
	}
	if err != sql.ErrNoRows {
		return 0, err
	}

	res, err := DB.Exec(
		`INSERT INTO users (telegram_id, username, first_name, created_at)
         VALUES (?, ?, ?, NOW())`,
		telegramID, username, firstName,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func DB_GetUserByTelegramID(telegramID int64) (*User, error) {
	if DB == nil {
		return nil, errors.New("db not initialized")
	}

	row := DB.QueryRow(`
		SELECT id, telegram_id, username, first_name, created_at
		FROM users WHERE telegram_id = ?`,
		telegramID,
	)

	var u User
	if err := row.Scan(&u.ID, &u.TelegramID, &u.Username, &u.FirstName, &u.CreatedAt); err != nil {
		return nil, err
	}
	return &u, nil
}

func DB_GetTelegramIDByUserID(userID int64) (int64, error) {
	if DB == nil {
		return 0, errors.New("db not initialized")
	}

	var telegramID int64
	err := DB.QueryRow(`SELECT telegram_id FROM users WHERE id = ? LIMIT 1`, userID).Scan(&telegramID)
	if err != nil {
		return 0, err
	}
	return telegramID, nil
}

// ======================================================
//  PORTFOLIOS (таблиця portfolios)
//  id, user_id, name, total_invested, total_pnl, created_at
// ======================================================

type Portfolio struct {
	ID                   int64
	UserID               int64
	Name                 string
	TotalInvested        float64
	TotalPnL             float64
	CreatedAt            time.Time
	OnchainAlertsEnabled bool
}

func DB_CreatePortfolio(userID int64, name string) (int64, error) {
	if DB == nil {
		return 0, errors.New("db not initialized")
	}

	res, err := DB.Exec(`
		INSERT INTO portfolios (user_id, name, total_invested, total_pnl, created_at)
		VALUES (?, ?, 0, 0, NOW())`,
		userID, name,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func DB_GetPortfoliosByUser(userID int64) ([]Portfolio, error) {
	if DB == nil {
		return nil, errors.New("db not initialized")
	}

	rows, err := DB.Query(`
		SELECT id, user_id, name, total_invested, total_pnl, created_at, onchain_alerts_enabled
		FROM portfolios WHERE user_id = ?`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Portfolio
	for rows.Next() {
		var p Portfolio
		if err := rows.Scan(&p.ID, &p.UserID, &p.Name, &p.TotalInvested, &p.TotalPnL, &p.CreatedAt, &p.OnchainAlertsEnabled); err != nil {
			return nil, err
		}
		res = append(res, p)
	}
	return res, nil
}

func DB_GetPortfolioByID(id int64) (*Portfolio, error) {
	if DB == nil {
		return nil, errors.New("db not initialized")
	}

	row := DB.QueryRow(`
		SELECT id, user_id, name, total_invested, total_pnl, created_at, onchain_alerts_enabled
		FROM portfolios WHERE id = ?`,
		id,
	)
	var p Portfolio
	if err := row.Scan(&p.ID, &p.UserID, &p.Name, &p.TotalInvested, &p.TotalPnL, &p.CreatedAt, &p.OnchainAlertsEnabled); err != nil {
		return nil, err
	}
	return &p, nil
}

func DB_UpdatePortfolioTotals(id int64, totalInvested, totalPnL float64) error {
	if DB == nil {
		return errors.New("db not initialized")
	}

	_, err := DB.Exec(`
		UPDATE portfolios
		SET total_invested = ?, total_pnl = ?
		WHERE id = ?`,
		totalInvested, totalPnL, id,
	)
	return err
}

// ======================================================
//  TOKENS (таблиця tokens)
//  id, portfolio_id, contract, symbol, amount, invested, buy_price_usd, current_price_usd ...
// ======================================================

type Token struct {
	ID              int64
	PortfolioID     int64
	Contract        string
	Chain           int
	Symbol          string
	Amount          float64
	Invested        float64
	BuyPriceUSD     float64
	CurrentPriceUSD float64
	Realized        float64
	CreatedAt       time.Time
	UpdatedAt       time.Time
	TokenPriceID    int64 `json:"tokenPriceId"`
}

func DB_AddToken(t Token) (int64, error) {
	if DB == nil {
		return 0, errors.New("db not initialized")
	}

	res, err := DB.Exec(`
        INSERT INTO tokens
            (portfolio_id, token_price_id, amount, invested, buy_price_usd, created_at, updated_at, realized, chain)
        VALUES (?, ?, ?, ?, ?, NOW(), NOW(), ?, ?)`,
		t.PortfolioID,
		t.TokenPriceID,
		t.Amount,
		t.Invested,
		t.BuyPriceUSD,
		t.Realized,
		t.Chain,
	)
	if err != nil {
		log.Printf("DB_AddToken error: %v", err)
		return 0, err
	}

	id, err := res.LastInsertId()
	if err != nil {
		log.Printf("DB_AddToken LastInsertId error: %v", err)
		return 0, err
	}

	log.Printf(
		"DB_AddToken OK: id=%d, portfolio_id=%d, token_price_id=%d, amount=%f, invested=%f",
		id, t.PortfolioID, t.TokenPriceID, t.Amount, t.Invested,
	)

	return id, nil
}
func DB_UpdateToken(
	id int64,
	amount float64,
	invested float64,
	buyPrice float64,
) error {
	_, err := DB.Exec(`
		UPDATE tokens
		SET
			amount = ?,
			invested = ?,
			buy_price_usd = ?,
			updated_at = NOW()
		WHERE id = ?
	`, amount, invested, buyPrice, id)

	return err
}

func DB_GetTokensByPortfolio(portfolioID int64) ([]Token, error) {
	if DB == nil {
		return nil, errors.New("db not initialized")
	}

	rows, err := DB.Query(`
		SELECT
			t.id,
			t.portfolio_id,
			t.amount,
			t.invested,
			t.buy_price_usd,
			tp.symbol,
			tp.price_usd,
			t.updated_at,
			t.created_at,
			t.realized
		FROM tokens t
		JOIN token_prices tp ON tp.id = t.token_price_id
		WHERE t.portfolio_id = ?`,
		portfolioID,
	)
	if err != nil {
		log.Printf("DB_GetTokensByPortfolio error: %v", err)
		return nil, err
	}
	defer rows.Close()

	var res []Token
	for rows.Next() {
		var t Token
		if err := rows.Scan(
			&t.ID, //
			&t.PortfolioID,
			&t.Amount,          // t.amount
			&t.Invested,        // t.invested
			&t.BuyPriceUSD,     // t.buy_price_usd
			&t.Symbol,          // tp.symbol
			&t.CurrentPriceUSD, // tp.price_usd
			&t.UpdatedAt,       // tp.updated_at
			&t.CreatedAt,
			&t.Realized,
		); err != nil {
			return nil, err
		}

		res = append(res, t)
	}
	return res, nil
}

// ======================================================
//  PORTFOLIO SNAPSHOTS (таблиця portfolio_snapshots)
//  id, portfolio_id, snapshot(JSON), created_at
// ======================================================

func DB_SavePortfolioSnapshot(portfolioID int64, snapshot any) error {
	if DB == nil {
		return errors.New("db not initialized")
	}

	raw, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		INSERT INTO portfolio_snapshots (portfolio_id, snapshot, created_at)
		VALUES (?, ?, NOW())`,
		portfolioID, raw,
	)
	return err
}

// ======================================================
//  ALERT RULES + FILTERS (alert_rules, alert_filters)
// ======================================================

type AlertRuleRow struct {
	ID        int64
	UserID    int64
	Name      string
	CreatedAt time.Time
}

// створюємо "порожнє" правило з назвою
func DB_CreateAlertRule(userID int64, name string) (int64, error) {
	if DB == nil {
		return 0, errors.New("db not initialized")
	}

	res, err := DB.Exec(`
		INSERT INTO alert_rules (user_id, name, created_at)
		VALUES (?, ?, NOW())`,
		userID, name,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// зберегти JSON-фільтр для правила
func DB_SaveAlertFilter(ruleID int64, filterStruct any, telegram_id int64) error {
	if DB == nil {
		return errors.New("db not initialized")
	}

	raw, err := json.Marshal(filterStruct)
	if err != nil {
		return err
	}

	_, err = DB.Exec(`
		INSERT INTO alert_filters (rule_id, filter, created_at, telegram_id)
		VALUES (?, ?, NOW(), ?)`,
		ruleID, string(raw), telegram_id,
	)
	return err
}

// отримаємо всі фільтри + user_id (для сканера)
type DBAlertFilter struct {
	RuleID   int64
	UserID   int64
	RuleName string
	Filter   AlertRule
}

func DB_GetAllAlertFilters() ([]DBAlertFilter, error) {
	if DB == nil {
		return nil, errors.New("db not initialized")
	}

	rows, err := DB.Query(`
		SELECT
			af.rule_id,
			af.telegram_id,
			ar.name AS rule_name,
			af.filter
		FROM alert_filters af
		JOIN alert_rules ar ON ar.id = af.rule_id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []DBAlertFilter

	for rows.Next() {
		var (
			ruleID   int64
			userID   int64
			ruleName string
			raw      []byte
		)

		if err := rows.Scan(&ruleID, &userID, &ruleName, &raw); err != nil {
			return nil, err
		}

		var filter AlertRule
		if err := json.Unmarshal(raw, &filter); err != nil {
			continue
		}

		res = append(res, DBAlertFilter{
			RuleID:   ruleID,
			UserID:   userID,
			RuleName: ruleName,
			Filter:   filter,
		})
	}

	return res, nil
}

// отримати фільтри для конкретного юзера (по user_id)
func DB_GetAlertFiltersByUserID(userID int64) ([]AlertRule, error) {
	if DB == nil {
		return nil, errors.New("db not initialized")
	}

	rows, err := DB.Query(`
		SELECT af.filter
		FROM alert_filters af
		JOIN alert_rules ar ON ar.id = af.rule_id
		WHERE ar.user_id = ?`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []AlertRule
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var r AlertRule
		if err := json.Unmarshal(raw, &r); err != nil {
			continue
		}
		res = append(res, r)
	}
	return res, nil
}

// ======================================================
//  ALERTS (таблиця alerts)
//  id, rule_id (INT), user_id (INT), tx_hash, short_message, details(JSON), created_at
// ======================================================

func DB_SaveAlert(ruleID, userID int64, txHash, shortMsg string, details any) error {
	if DB == nil {
		return errors.New("db not initialized")
	}

	raw, _ := json.Marshal(details)

	_, err := DB.Exec(`
		INSERT INTO alerts (rule_id, user_id, tx_hash, short_message, details, created_at)
		VALUES (?, ?, ?, ?, ?, NOW())`,
		ruleID, userID, txHash, shortMsg, raw,
	)
	return err
}

type AlertRow struct {
	ID        int64
	RuleID    int64
	UserID    int64
	TxHash    string
	Short     string
	Details   json.RawMessage
	CreatedAt time.Time
}

func DB_GetAlertsByUserID(userID int64, limit int) ([]AlertRow, error) {
	if DB == nil {
		return nil, errors.New("db not initialized")
	}
	if limit <= 0 {
		limit = 50
	}

	rows, err := DB.Query(`
		SELECT id, rule_id, user_id, tx_hash, short_message, details, created_at
		FROM alerts
		WHERE user_id = ?
		ORDER BY id DESC
		LIMIT ?`,
		userID, limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []AlertRow
	for rows.Next() {
		var a AlertRow
		if err := rows.Scan(
			&a.ID, &a.RuleID, &a.UserID,
			&a.TxHash, &a.Short, &a.Details, &a.CreatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, nil
}
func DB_GetAllAlertRules() ([]AlertRule, []DBAlertFilter, error) {
	if DB == nil {
		return nil, nil, errors.New("db not initialized")
	}

	resultAlerts, err := DB_GetAllAlertFilters()
	if err != nil {
		return nil, nil, err
	}

	res := make([]AlertRule, 0, len(resultAlerts))
	for _, f := range resultAlerts {
		res = append(res, f.Filter)
	}
	return res, resultAlerts, nil
}

// десь зверху, біля інших моделей
type AlertRuleCard struct {
	ID           int64      `json:"id"`
	Name         string     `json:"name"`
	FiltersCount int64      `json:"filtersCount"`
	NewAlerts    int64      `json:"newAlerts"`
	LastAlertAt  *time.Time `json:"lastAlertAt,omitempty"`
}

// DB_GetAlertRuleCardsByUser повертає список правил для юзера
// з кількістю фільтрів і кількістю нових алертів
func DB_GetAlertRuleCardsByUser(userID int64) ([]AlertRuleCard, error) {
	const q = `
		SELECT
			ar.id,
			ar.name,
			COUNT(DISTINCT af.id) AS filters_count,
			COALESCE(SUM(CASE WHEN a.is_read = 0 THEN 1 ELSE 0 END), 0) AS new_alerts,
			MAX(a.created_at) AS last_alert_at
		FROM alert_rules ar
		LEFT JOIN alert_filters af ON af.rule_id = ar.id
		LEFT JOIN alerts a ON a.rule_id = ar.id
		WHERE ar.user_id = ?
		GROUP BY ar.id, ar.name
		ORDER BY ar.created_at DESC
	`

	rows, err := DB.Query(q, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []AlertRuleCard

	for rows.Next() {
		var (
			card      AlertRuleCard
			lastAlert sql.NullTime
		)

		if err := rows.Scan(
			&card.ID,
			&card.Name,
			&card.FiltersCount,
			&card.NewAlerts,
			&lastAlert,
		); err != nil {
			return nil, err
		}

		if lastAlert.Valid {
			card.LastAlertAt = &lastAlert.Time
		}

		res = append(res, card)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return res, nil
}

type AlertFilterRow struct {
	ID         int64     `json:"id"`
	RuleID     int64     `json:"rule_id"`
	TelegramID int64     `json:"telegram_id"`
	Filter     AlertRule `json:"filter"`
	CreatedAt  time.Time `json:"created_at"`
}

// Отримати всі фільтри конкретного правила
func DB_GetFiltersByRuleID(ruleID int64) ([]AlertFilterRow, error) {
	if DB == nil {
		return nil, fmt.Errorf("db not initialized")
	}

	rows, err := DB.Query(`
		SELECT id, rule_id, telegram_id, filter, created_at
		FROM alert_filters
		WHERE rule_id = ?
		ORDER BY id DESC
	`, ruleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []AlertFilterRow

	for rows.Next() {
		var (
			row AlertFilterRow
			raw []byte
		)

		if err := rows.Scan(
			&row.ID,
			&row.RuleID,
			&row.TelegramID,
			&raw,
			&row.CreatedAt,
		); err != nil {
			return nil, err
		}

		if err := json.Unmarshal(raw, &row.Filter); err != nil {
			continue
		}

		res = append(res, row)
	}

	return res, nil
}

func hourBucketNow() time.Time {
	return time.Now().UTC().Truncate(time.Hour)
}
func UpdateHourStats(chainID int, b BlockAnalytics) {
	hour := hourBucketNow()

	gasStr := "0"
	if b.GasUsed != nil {
		gasStr = b.GasUsed.String()
	}

	_, err := DB.Exec(`
		INSERT INTO chain_activity_hour
		  (chain_id, hour_ts, tx_count, gas_used)
		VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  tx_count = tx_count + VALUES(tx_count),
		  gas_used = gas_used + VALUES(gas_used)
	`,
		chainID,
		hour,
		b.SummaryTx,
		gasStr,
	)

	if err != nil {
		log.Println("UpdateHourStats:", err)
	}
}

func DB_GetAlertsByRule(ruleID, afterID int64, limit int) ([]Alert, error) {
	rows, err := DB.Query(`
		SELECT id, tx_hash, short_message, details, created_at 
		FROM alerts
		WHERE rule_id = ?
		  AND id > ?
		ORDER BY id ASC
		LIMIT ?
	`, ruleID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []Alert
	for rows.Next() {
		var a Alert
		if err := rows.Scan(
			&a.ID,
			&a.TxHash,
			&a.ShortMessage,
			&a.Details,
			&a.CreatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, a)
	}
	return res, nil
}
func DB_DeleteAlertFilter(id int64) error {
	_, err := DB.Exec(`DELETE FROM alert_filters WHERE id = ?`, id)
	return err
}
func DB_UpdateAlertFilter(id int64, raw []byte) error {
	_, err := DB.Exec(`
		UPDATE alert_filters 
		SET filter = ? 
		WHERE id = ?
	`, raw, id)
	return err
}
func DB_DeleteAlertRule(id int64) error {
	_, err := DB.Exec(`DELETE FROM alert_rules WHERE id = ?`, id)
	return err
}
func DB_DeletePortfolio(id int64) error {
	// Видалити всі токени, що належать портфелю
	if _, err := DB.Exec(`DELETE FROM tokens WHERE portfolio_id = ?`, id); err != nil {
		return err
	}
	// Видалити сам портфель
	_, err := DB.Exec(`DELETE FROM portfolios WHERE id = ?`, id)
	return err
}
func DB_UpdateTokenPriceAndSymbol(tokenID int64, price float64, symbol string) error {
	_, err := DB.Exec(`
		UPDATE portfolio_tokens
		SET current_price_usd = ?, symbol = ?, updated_at = NOW()
		WHERE id = ?
	`, price, symbol, tokenID)

	return err
}
func DB_GetAllTokenPrices() ([]TokenPriceRow, error) {
	rows, err := DB.Query(`
		SELECT chain_id, contract, symbol, price_usd, updated_at
		FROM token_prices
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var res []TokenPriceRow
	for rows.Next() {
		var t TokenPriceRow
		if err := rows.Scan(
			&t.ChainID,
			&t.Contract,
			&t.Symbol,
			&t.PriceUSD,
			&t.UpdatedAt,
		); err != nil {
			return nil, err
		}
		res = append(res, t)
	}
	return res, rows.Err()
}

func DB_UpdateTokenPrice(chainID int, contract, symbol string, price float64) error {
	_, err := DB.Exec(`
		UPDATE token_prices
		SET symbol = ?, price_usd = ?, updated_at = NOW()
		WHERE chain_id = ? AND contract = ?
	`, symbol, price, chainID, contract)
	return err
}
func DB_GetTokensForRefresh(limit int) ([]TokenPriceRow, error) {
	// log.Printf("[price] selecting up to %d tokens for refresh", limit)

	rows, err := DB.Query(`
		SELECT chain_id, contract, symbol, price_usd, updated_at
		FROM token_prices
		ORDER BY updated_at ASC
		LIMIT ?
	`, limit)
	if err != nil {
		log.Printf("[price][ERR] query failed: %v", err)
		return nil, err
	}
	defer rows.Close()

	var res []TokenPriceRow

	for rows.Next() {
		var t TokenPriceRow
		if err := rows.Scan(
			&t.ChainID,
			&t.Contract,
			&t.Symbol,
			&t.PriceUSD,
			&t.UpdatedAt,
		); err != nil {
			log.Printf("[price][ERR] scan failed: %v", err)
			return nil, err
		}

		res = append(res, t)
	}

	if err := rows.Err(); err != nil {
		log.Printf("[price][ERR] rows error: %v", err)
		return nil, err
	}

	// log.Printf("[price] selected %d tokens for refresh", len(res))
	return res, nil
}

func InsertTokenStub(chainID int, contract string) (int64, error) {
	contract = strings.ToLower(contract)

	res, err := DB.Exec(`
		INSERT INTO token_prices (
			chain_id,
			contract,
			symbol,
			price_usd,
			updated_at
		)
		VALUES (?, ?, 'UNKNOWN', 0, '1970-01-01 00:00:00')
	`, chainID, contract)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}
func DB_GetTokenPriceID(chainID int, contract string) (int64, error) {
	contract = strings.ToLower(contract)

	var id int64
	err := DB.QueryRow(`
		SELECT id
		FROM token_prices
		WHERE chain_id = ? AND contract = ?
		LIMIT 1
	`, chainID, contract).Scan(&id)

	if err == sql.ErrNoRows {
		return 0, sql.ErrNoRows
	}
	if err != nil {
		return 0, err
	}
	return id, nil
}
func DB_GetTokenByPortfolioAndPriceID(portfolioID, tokenPriceID int64) (*Token, error) {
	if DB == nil {
		return nil, errors.New("db not initialized")
	}

	var t Token
	err := DB.QueryRow(`
		SELECT id, amount, invested
		FROM tokens
		WHERE portfolio_id = ?
		  AND token_price_id = ?
		LIMIT 1
	`, portfolioID, tokenPriceID).Scan(
		&t.ID,
		&t.Amount,
		&t.Invested,
	)

	if err == sql.ErrNoRows {
		return nil, nil // токена немає — це ОК
	}
	if err != nil {
		return nil, err
	}

	return &t, nil
}
func DB_DeleteToken(tokenID int64) error {
	_, err := DB.Exec(`
		DELETE FROM tokens
		WHERE id = ?
	`, tokenID)
	return err
}
func rollbackTokenStub(tokenPriceID int64, contract string) {
	// 1️⃣ видаляємо з портфелів
	_, _ = DB.Exec(`
		DELETE FROM tokens
		WHERE token_price_id = ?
	`, tokenPriceID)

	// 2️⃣ чистимо token_prices
	_, _ = DB.Exec(`
		DELETE FROM token_prices
		WHERE id = ?
	`, tokenPriceID)

	// 3️⃣ лог (або твоя система нотифікацій)
	log.Printf("❌ Token removed: %s (price/symbol not resolved)", contract)
}
func DB_TouchTokenPrice(chainID int, contract string) error {
	_, err := DB.Exec(`
		UPDATE token_prices
		SET updated_at = NOW()
		WHERE chain_id = ? AND contract = ?
	`, chainID, contract)
	return err
}

// the func db for parser token
func DBUpsertTokenMetadata(t TokenData) error {
	const q = `
	INSERT INTO tokens_metadata (
		chain_id, contract, symbol, decimals,
		max_total_supply,
		holders, holders_change,
		transfers_total, transfers_24h,
		price_usd, price_eth, price_change,
		onchain_market_cap, circulating_market_cap
	) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)
	ON DUPLICATE KEY UPDATE
		symbol=VALUES(symbol),
		decimals=VALUES(decimals),
		max_total_supply=VALUES(max_total_supply),
		holders=VALUES(holders),
		holders_change=VALUES(holders_change),
		transfers_total=VALUES(transfers_total),
		transfers_24h=VALUES(transfers_24h),
		price_usd=VALUES(price_usd),
		price_eth=VALUES(price_eth),
		price_change=VALUES(price_change),
		onchain_market_cap=VALUES(onchain_market_cap),
		circulating_market_cap=VALUES(circulating_market_cap),
		updated_at=NOW()
	`
	_, err := DB.Exec(
		q,
		t.ChainID,
		strings.ToLower(t.Address),
		t.Symbol,
		t.Decimals,
		t.MaxTotalSupply,
		t.Holders,
		t.HoldersChange,
		t.TransfersTotal,
		t.Transfers24h,
		t.PriceUSD,
		t.PriceETH,
		t.PriceChange,
		t.OnchainMarketCap,
		t.CirculatingMarketCap,
	)
	return err
}
func DBGetTokenMetadata(chainID int, address string) (*TokenData, error) {
	const q = `
	SELECT
		chain_id, contract, symbol, decimals,
		max_total_supply,
		holders, holders_change,
		transfers_total, transfers_24h,
		price_usd, price_eth, price_change,
		onchain_market_cap, circulating_market_cap
	FROM tokens_metadata
	WHERE chain_id=? AND contract=?
	LIMIT 1
	`

	var t TokenData
	err := DB.QueryRow(q, chainID, strings.ToLower(address)).Scan(
		&t.ChainID,
		&t.Address,
		&t.Symbol,
		&t.Decimals,
		&t.MaxTotalSupply,
		&t.Holders,
		&t.HoldersChange,
		&t.TransfersTotal,
		&t.Transfers24h,
		&t.PriceUSD,
		&t.PriceETH,
		&t.PriceChange,
		&t.OnchainMarketCap,
		&t.CirculatingMarketCap,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

type KnownAddressDB struct {
	Address string
	Name    string
	Source  string
}

func DB_GetKnownAddress(address string) (*KnownAddressDB, error) {
	row := DB.QueryRow(`
		SELECT address, label_name, source
		FROM address_classification
		WHERE address = ?
		  AND is_disabled = FALSE
		LIMIT 1
	`, address)

	var k KnownAddressDB
	if err := row.Scan(&k.Address, &k.Name, &k.Source); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &k, nil
}
func DB_AddressExists(address string) (bool, error) {
	var v int
	err := DB.QueryRow(`
		SELECT 1 FROM address_classification
		WHERE address = ?
		LIMIT 1
	`, address).Scan(&v)

	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}

func DB_InsertAddress(
	address, label, class string,
	confidence float64,
	source, rule string,
) error {
	_, err := DB.Exec(`
		INSERT INTO address_classification
		(address, label_name, class, confidence, source, rule_applied)
		VALUES (?, ?, ?, ?, ?, ?)
	`, address, label, class, confidence, source, rule)
	return err
}

func DB_UpdateAddress(
	address, class string,
	confidence float64,
	rule string,
) error {
	_, err := DB.Exec(`
		UPDATE address_classification
		SET class = ?, confidence = ?, rule_applied = ?
		WHERE address = ?
		  AND manual_class IS NULL
	`, class, confidence, rule, address)
	return err
}
func DB_TryLoadHourlyMetrics(chainID int64, tokenLower string, hourTS int64) *TokenHourlyMetrics {
	q := `SELECT
            chain_id, token, hour_ts,
            transfers, unique_senders, unique_receivers, unique_addresses,
            p50_raw, p95_raw, p99_raw,
            p50_usd, p95_usd, p99_usd,
            top1_addr_share, top3_addr_share, top5_addr_share,
            exchange_share, net_exchange_usd,
            usd_lt_100, usd_100_1k, usd_1k_10k, usd_10k_100k, usd_gt_100k
          FROM token_hourly_metrics
          WHERE chain_id=? AND token=? AND hour_ts=? LIMIT 1`
	var m TokenHourlyMetrics
	var p50r, p95r, p99r sql.NullString
	err := DB.QueryRow(q, chainID, tokenLower, hourTS).Scan(
		&m.ChainID, &m.Token, &m.HourTS,
		&m.Transfers, &m.UniqueSenders, &m.UniqueReceivers, &m.UniqueAddresses,
		&p50r, &p95r, &p99r,
		&m.P50USD, &m.P95USD, &m.P99USD,
		&m.Top1AddrShare, &m.Top3AddrShare, &m.Top5AddrShare,
		&m.ExchangeShare, &m.NetExchangeUSD,
		&m.USDLt100, &m.USD100_1k, &m.USD1k_10k, &m.USD10k_100k, &m.USDGt100k,
	)
	if err != nil {
		return nil
	}
	if p50r.Valid {
		m.P50Raw, _ = new(big.Int).SetString(p50r.String, 10)
	}
	if p95r.Valid {
		m.P95Raw, _ = new(big.Int).SetString(p95r.String, 10)
	}
	if p99r.Valid {
		m.P99Raw, _ = new(big.Int).SetString(p99r.String, 10)
	}
	return &m
}

// func DB_MatchRule(name string) (string, float64, string, error) {
// 	row := DB.QueryRow(`
// 		SELECT class, confidence,
// 		       CONCAT(match_type, ':', match_value)
// 		FROM classification_rules
// 		WHERE enabled = TRUE
// 		  AND (
// 		    (match_type = 'CONTAINS' AND ? LIKE CONCAT('%', match_value, '%'))
// 		 OR (match_type = 'PREFIX'   AND ? LIKE CONCAT(match_value, '%'))
// 		 OR (match_type = 'EXACT'    AND ? = match_value)
// 		 OR (match_type = 'REGEX'    AND ? REGEXP match_value)
// 		  )
// 		ORDER BY priority ASC
// 		LIMIT 1
// 	`, name, name, name, name)

// 	var class string
// 	var conf float64
// 	var rule string

// 	if err := row.Scan(&class, &conf, &rule); err != nil {
// 		if err == sql.ErrNoRows {
// 			return "UNKNOWN", 0.50, "NONE", nil
// 		}
// 		return "", 0, "", err
// 	}

// 	return class, conf, rule, nil
// }

// func DB_LoadTrackedTokens(chainID int64) (map[string]bool, error) {
// 	// таблиця tokens має містити хоча б:
// 	// chain_id, contract, onchain_tracking TINYINT
// 	q := `
// SELECT LOWER(contract)
// FROM tokens_metadata
// WHERE chain_id = ? AND onchain_tracking = 1
// `
// 	rows, err := DB.Query(q, chainID)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	out := make(map[string]bool)
// 	for rows.Next() {
// 		var c string
// 		if err := rows.Scan(&c); err != nil {
// 			return nil, err
// 		}
// 		out[c] = true
// 	}
// 	return out, rows.Err()
// }

// /*
// ========================================================
// DB — HOURLY ANALYTICS UPSERT
// ========================================================
// */

// func DB_UpsertTokenHourlyActivity(agg map[string]*HourlyTokenActivity) error {
// 	// MySQL upsert (token_hourly_activity):
// 	// UNIQUE (chain_id, token, hour_ts)
// 	//
// 	// Ми зберігаємо big.Int як DECIMAL(78,0) рядком.
// 	//
// 	q := `
// INSERT INTO token_hourly_activity
// (chain_id, token, hour_ts, transfer_count, total_volume_raw, exchange_in_raw, exchange_out_raw, max_transfer_raw, updated_at)
// VALUES
// (?, ?, ?, ?, ?, ?, ?, ?, NOW())
// ON DUPLICATE KEY UPDATE
// transfer_count = transfer_count + VALUES(transfer_count),
// total_volume_raw = total_volume_raw + VALUES(total_volume_raw),
// exchange_in_raw = exchange_in_raw + VALUES(exchange_in_raw),
// exchange_out_raw = exchange_out_raw + VALUES(exchange_out_raw),
// max_transfer_raw = GREATEST(max_transfer_raw, VALUES(max_transfer_raw)),
// updated_at = NOW()
// `

// 	tx, err := DB.Begin()
// 	if err != nil {
// 		return err
// 	}
// 	defer func() { _ = tx.Rollback() }()

// 	stmt, err := tx.Prepare(q)
// 	if err != nil {
// 		return err
// 	}
// 	defer stmt.Close()

// 	for _, h := range agg {
// 		_, err := stmt.Exec(
// 			h.ChainID,
// 			strings.ToLower(h.Token.Hex()),
// 			h.HourTS,
// 			h.TransferCount,
// 			h.TotalVolume.String(),
// 			h.ExchangeIn.String(),
// 			h.ExchangeOut.String(),
// 			h.MaxTransfer.String(),
// 		)
// 		if err != nil {
// 			return err
// 		}
// 	}

// 	return tx.Commit()
// }

// /*
// ========================================================
// DB — ПІДПИСКИ ПОРТФЕЛІВ НА ТОКЕН
// ========================================================

// Мінімальна логіка:
// - portfolio_token_alert_settings: налаштування фільтрів на токен у портфелі
// - portfolios: має onchain_alerts_enabled
// - portfolio_tokens: де токени в портфелі

// */

// func DB_LoadPortfolioTokenSubscriptions(chainID int64, tokenLower string) ([]PortfolioTokenSubscription, error) {
// 	q := `
// SELECT
// 	s.portfolio_id,
// 	p.user_id,
// 	LOWER(tp.contract) AS token_lower,
// 	s.min_usd,
// 	s.min_raw,
// 	s.direction,
// 	s.spike_mult,
// 	s.dominance_pct
// FROM portfolio_token_alert_settings s
// JOIN portfolios p
//   ON p.id = s.portfolio_id
// JOIN tokens t
//   ON t.portfolio_id = s.portfolio_id
// JOIN token_prices tp
//   ON tp.id = t.token_price_id
// WHERE
// 	tp.chain_id = ?
// 	AND LOWER(tp.contract) = ?
// 	AND s.enabled = 1
// 	AND p.onchain_alerts_enabled = 1
// `
// 	rows, err := DB.Query(q, chainID, tokenLower)
// 	if err != nil {
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	var out []PortfolioTokenSubscription

// 	for rows.Next() {
// 		var sub PortfolioTokenSubscription
// 		var minUSD sql.NullFloat64
// 		var spike sql.NullFloat64
// 		var dom sql.NullFloat64
// 		var minRaw sql.NullString
// 		var direction sql.NullString

// 		if err := rows.Scan(
// 			&sub.PortfolioID,
// 			&sub.UserID,
// 			&sub.TokenLower,
// 			&minUSD,
// 			&minRaw,
// 			&direction,
// 			&spike,
// 			&dom,
// 		); err != nil {
// 			return nil, err
// 		}

// 		r := AlertRuleForToken{}
// 		if minUSD.Valid {
// 			r.MinUSD = minUSD.Float64
// 		}
// 		if minRaw.Valid {
// 			r.MinRaw = minRaw.String
// 		}
// 		if direction.Valid {
// 			r.Direction = direction.String
// 		}
// 		if spike.Valid {
// 			r.SpikeMult = spike.Float64
// 		}
// 		if dom.Valid {
// 			r.DominancePct = dom.Float64
// 		}

// 		sub.Rule = r
// 		out = append(out, sub)
// 	}

// 	return out, rows.Err()
// }

// /*
// ========================================================
// DB — ДАНІ ДЛЯ ФІЛЬТРІВ (історія/avg/ціна)
// ========================================================
// */

// func DB_LoadTokenFilterContext(chainID int64, tokenLower string, hourTS int64) (TokenFilterContext, error) {
// 	var ref TokenFilterContext
// 	ref.HourTotalRaw = big.NewInt(0)
// 	ref.HourAvgRaw = big.NewInt(0)

// 	// 1) price_usd з tokens_meta
// 	{
// 		q := `
// SELECT price_usd
// FROM tokens_meta
// WHERE chain_id = ? AND LOWER(contract) = ?
// LIMIT 1
// `
// 		var price sql.NullFloat64
// 		_ = DB.QueryRow(q, chainID, tokenLower).Scan(&price)
// 		ref.PriceUSD = price
// 	}

// 	// 2) total volume за годину
// 	{
// 		q := `
// SELECT total_volume_raw
// FROM token_hourly_activity
// WHERE chain_id = ? AND token = ? AND hour_ts = ?
// LIMIT 1
// `
// 		var totalStr sql.NullString
// 		err := DB.QueryRow(q, chainID, tokenLower, hourTS).Scan(&totalStr)
// 		if err == nil && totalStr.Valid {
// 			if v, ok := new(big.Int).SetString(totalStr.String, 10); ok {
// 				ref.HourTotalRaw = v
// 			}
// 		}
// 	}

// 	// 3) середній обʼєм за останні 24 години
// 	{
// 		from := hourTS - 24*3600
// 		to := hourTS - 1
// 		q := `
// SELECT AVG(total_volume_raw)
// FROM token_hourly_activity
// WHERE chain_id = ? AND token = ? AND hour_ts BETWEEN ? AND ?
// `
// 		var avg sql.NullString
// 		err := DB.QueryRow(q, chainID, tokenLower, from, to).Scan(&avg)
// 		if err == nil && avg.Valid {
// 			s := avg.String
// 			if i := strings.IndexByte(s, '.'); i >= 0 {
// 				s = s[:i]
// 			}
// 			if v, ok := new(big.Int).SetString(strings.TrimSpace(s), 10); ok {
// 				ref.HourAvgRaw = v
// 			}
// 		}
// 	}

// 	return ref, nil
// }

// /*
// ========================================================
// DB — PORTFOLIO NOTIFICATIONS INSERT
// ========================================================
// */

// func DB_InsertPortfolioNotification(

// 	portfolioID int64,
// 	ev TokenOnchainEvent,
// 	rule AlertRuleForToken,
// 	ref TokenFilterContext,
// ) (int64, error) {

// 	// Зберігаємо:
// 	// - txHash
// 	// - token
// 	// - direction
// 	// - amount_raw
// 	// - exchange name
// 	// - час
// 	// - rule snapshot (мінімально) як hex/json; тут зробимо простий “rule_hash”
// 	//
// 	// Якщо хочеш — заміниш на JSON.
// 	ruleHash := hashRule(rule)

// 	q := `
// INSERT INTO portfolio_notifications
// (portfolio_id, chain_id, token, tx_hash, block_time, direction, amount_raw, exchange_name, rule_hash, created_at)
// VALUES
// (?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
// `
// 	res, err := DB.Exec(
// 		q,
// 		portfolioID,
// 		ev.ChainID,
// 		strings.ToLower(ev.Token.Hex()),
// 		strings.ToLower(ev.TxHash.Hex()),
// 		ev.BlockTime.UTC(),
// 		string(ev.Direction),
// 		ev.Amount.String(),
// 		ev.ExchangeName,
// 		ruleHash,
// 	)
// 	if err != nil {
// 		return 0, err
// 	}
// 	return res.LastInsertId()
// }
// func DB_LoadTokenProfile(chainID int64, tokenLower string) (*TokenProfile, error) {
// 	var p TokenProfile

// 	q := `
// SELECT
// 	chain_id,
// 	LOWER(contract),
// 	price_usd,
// 	circulating_market_cap,
// 	onchain_market_cap,
// 	max_total_supply,
// 	decimals,
// 	holders,
// 	top10_pct,
// 	top50_pct,
// 	top100_pct
// FROM tokens_metadata
// WHERE chain_id = ? AND LOWER(contract) = ?
// LIMIT 1
// `
// 	err := DB.QueryRow(q, chainID, tokenLower).Scan(
// 		&p.ChainID,
// 		&p.TokenLower,
// 		&p.PriceUSD,
// 		&p.CirculatingMarketCap,
// 		&p.OnchainMarketCap,
// 		&p.MaxTotalSupply,
// 		&p.Decimals,
// 		&p.Holders,
// 		&p.Top10Pct,
// 		&p.Top50Pct,
// 		&p.Top100Pct,
// 	)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return &p, nil
// }
