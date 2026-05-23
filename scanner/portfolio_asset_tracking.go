package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

type PortfolioAssetNetworkDTO struct {
	ID                     int64   `json:"id"`
	ChainID                int64   `json:"chain_id"`
	Chain                  string  `json:"chain"`
	Contract               string  `json:"contract"`
	Symbol                 string  `json:"symbol"`
	Amount                 float64 `json:"amount"`
	Invested               float64 `json:"invested"`
	Realized               float64 `json:"realized"`
	BuyPriceUSD            float64 `json:"buy_price_usd"`
	CurrentPriceUSD        float64 `json:"current_price_usd"`
	Value                  float64 `json:"value"`
	TotalPnL               float64 `json:"total_pnl"`
	NetworkTrackingEnabled bool    `json:"network_tracking_enabled"`
	TrackingEnabled        bool    `json:"tracking_enabled"`
}

type PortfolioAssetDTO struct {
	AssetKey                 string                     `json:"asset_key"`
	Symbol                   string                     `json:"symbol"`
	PortfolioTrackingEnabled bool                       `json:"portfolio_tracking_enabled"`
	AssetTrackingEnabled     bool                       `json:"asset_tracking_enabled"`
	TrackingEnabled          bool                       `json:"tracking_enabled"`
	DailyReportEnabled       bool                       `json:"daily_report_enabled"`
	TrackingProfile          string                     `json:"tracking_profile"`
	TotalAmount              float64                    `json:"total_amount"`
	TotalInvested            float64                    `json:"total_invested"`
	TotalValue               float64                    `json:"total_value"`
	TotalRealized            float64                    `json:"total_realized"`
	TotalPnL                 float64                    `json:"total_pnl"`
	TrackedNetworks          int                        `json:"tracked_networks"`
	NetworkCount             int                        `json:"network_count"`
	Networks                 []PortfolioAssetNetworkDTO `json:"networks"`
}

func EnsurePortfolioTrackingSchema() error {
	if err := ensurePortfolioAssetTrackingSettingsSchema(); err != nil {
		return err
	}
	return ensurePortfolioAssetTrackingNetworksSchema()
}

func ensurePortfolioAssetTrackingSettingsSchema() error {
	const tableName = "portfolio_asset_tracking_settings"
	createSQL := `
CREATE TABLE IF NOT EXISTS portfolio_asset_tracking_settings (
  portfolio_id BIGINT NOT NULL,
  asset_key VARCHAR(96) NOT NULL,
  asset_symbol VARCHAR(96) NOT NULL DEFAULT '',
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  daily_report_enabled TINYINT(1) NOT NULL DEFAULT 0,
  profile VARCHAR(24) NOT NULL DEFAULT 'balanced',
  min_usd DOUBLE NULL,
  min_raw VARCHAR(80) NULL,
  direction VARCHAR(8) NULL,
  spike_mult DOUBLE NULL,
  dominance_pct DOUBLE NULL,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (portfolio_id, asset_key)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	requiredCols := []schemaColumn{
		{name: "portfolio_id", def: "portfolio_id BIGINT NOT NULL"},
		{name: "asset_key", def: "asset_key VARCHAR(96) NOT NULL"},
		{name: "asset_symbol", def: "asset_symbol VARCHAR(96) NOT NULL DEFAULT ''"},
		{name: "enabled", def: "enabled TINYINT(1) NOT NULL DEFAULT 1"},
		{name: "daily_report_enabled", def: "daily_report_enabled TINYINT(1) NOT NULL DEFAULT 0"},
		{name: "profile", def: "profile VARCHAR(24) NOT NULL DEFAULT 'balanced'"},
		{name: "min_usd", def: "min_usd DOUBLE NULL"},
		{name: "min_raw", def: "min_raw VARCHAR(80) NULL"},
		{name: "direction", def: "direction VARCHAR(8) NULL"},
		{name: "spike_mult", def: "spike_mult DOUBLE NULL"},
		{name: "dominance_pct", def: "dominance_pct DOUBLE NULL"},
		{name: "created_at", def: "created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP"},
		{name: "updated_at", def: "updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"},
	}

	requiredIdx := []schemaIndex{
		{name: "idx_pats_portfolio_enabled", def: "ADD INDEX idx_pats_portfolio_enabled (portfolio_id, enabled)"},
		{name: "idx_pats_asset_key", def: "ADD INDEX idx_pats_asset_key (asset_key)"},
	}

	return ensureTableSchema(tableName, createSQL, requiredCols, requiredIdx)
}

func ensurePortfolioAssetTrackingNetworksSchema() error {
	const tableName = "portfolio_asset_tracking_networks"
	createSQL := `
CREATE TABLE IF NOT EXISTS portfolio_asset_tracking_networks (
  portfolio_id BIGINT NOT NULL,
  asset_key VARCHAR(96) NOT NULL,
  chain_id BIGINT NOT NULL,
  token CHAR(42) NOT NULL,
  enabled TINYINT(1) NOT NULL DEFAULT 1,
  created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP,
  updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
  PRIMARY KEY (portfolio_id, chain_id, token)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

	requiredCols := []schemaColumn{
		{name: "portfolio_id", def: "portfolio_id BIGINT NOT NULL"},
		{name: "asset_key", def: "asset_key VARCHAR(96) NOT NULL"},
		{name: "chain_id", def: "chain_id BIGINT NOT NULL"},
		{name: "token", def: "token CHAR(42) NOT NULL"},
		{name: "enabled", def: "enabled TINYINT(1) NOT NULL DEFAULT 1"},
		{name: "created_at", def: "created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP"},
		{name: "updated_at", def: "updated_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP"},
	}

	requiredIdx := []schemaIndex{
		{name: "idx_patn_portfolio_asset", def: "ADD INDEX idx_patn_portfolio_asset (portfolio_id, asset_key)"},
		{name: "idx_patn_asset_chain", def: "ADD INDEX idx_patn_asset_chain (asset_key, chain_id)"},
	}

	return ensureTableSchema(tableName, createSQL, requiredCols, requiredIdx)
}

func portfolioAssetKeyExprSQL(tpAlias, tmAlias string) string {
	return fmt.Sprintf(
		"UPPER(COALESCE(NULLIF(%s.symbol,''), NULLIF(%s.symbol,''), %s.contract))",
		tmAlias, tpAlias, tpAlias,
	)
}

func portfolioAssetSymbolExprSQL(tpAlias, tmAlias string) string {
	return fmt.Sprintf(
		"COALESCE(NULLIF(%s.symbol,''), NULLIF(%s.symbol,''), %s.contract)",
		tmAlias, tpAlias, tpAlias,
	)
}

func normalizeAssetKey(symbol, contract string) string {
	key := strings.ToUpper(strings.TrimSpace(symbol))
	if key != "" {
		return key
	}
	return strings.ToLower(strings.TrimSpace(contract))
}

func normalizeTrackingProfile(profile string) string {
	switch strings.ToLower(strings.TrimSpace(profile)) {
	case "quiet":
		return "quiet"
	case "aggressive":
		return "aggressive"
	default:
		return "balanced"
	}
}

func trackingProfileDefaults(profile string) AlertRuleForToken {
	switch normalizeTrackingProfile(profile) {
	case "quiet":
		return AlertRuleForToken{
			MinUSD:       75000,
			SpikeMult:    4,
			DominancePct: 0.20,
		}
	case "aggressive":
		return AlertRuleForToken{
			MinUSD:       10000,
			SpikeMult:    2,
			DominancePct: 0.10,
		}
	default:
		return AlertRuleForToken{
			MinUSD:       25000,
			SpikeMult:    3,
			DominancePct: 0.15,
		}
	}
}

func EnsurePortfolioAssetTrackingDefaults(portfolioID int64) error {
	if portfolioID == 0 {
		return nil
	}

	assetKeyExpr := portfolioAssetKeyExprSQL("tp", "tm")
	assetSymbolExpr := portfolioAssetSymbolExprSQL("tp", "tm")
	defaultRule := trackingProfileDefaults("balanced")

	settingsQuery := fmt.Sprintf(`
INSERT INTO portfolio_asset_tracking_settings
	(portfolio_id, asset_key, asset_symbol, enabled, daily_report_enabled, profile, min_usd, spike_mult, dominance_pct, created_at, updated_at)
SELECT DISTINCT
	t.portfolio_id,
	%s AS asset_key,
	%s AS asset_symbol,
	1,
	0,
	'balanced',
	?,
	?,
	?,
	NOW(),
	NOW()
FROM tokens t
JOIN token_prices tp ON tp.id = t.token_price_id
LEFT JOIN tokens_metadata tm ON tm.chain_id = tp.chain_id AND LOWER(tm.contract) = LOWER(tp.contract)
WHERE t.portfolio_id = ?
ON DUPLICATE KEY UPDATE
	asset_symbol = VALUES(asset_symbol),
	updated_at = NOW()`,
		assetKeyExpr,
		assetSymbolExpr,
	)
	if _, err := DB.Exec(settingsQuery, defaultRule.MinUSD, defaultRule.SpikeMult, defaultRule.DominancePct, portfolioID); err != nil {
		return err
	}

	networksQuery := fmt.Sprintf(`
INSERT INTO portfolio_asset_tracking_networks
	(portfolio_id, asset_key, chain_id, token, enabled, created_at, updated_at)
SELECT DISTINCT
	t.portfolio_id,
	%s AS asset_key,
	tp.chain_id,
	LOWER(tp.contract),
	1,
	NOW(),
	NOW()
FROM tokens t
JOIN token_prices tp ON tp.id = t.token_price_id
LEFT JOIN tokens_metadata tm ON tm.chain_id = tp.chain_id AND LOWER(tm.contract) = LOWER(tp.contract)
WHERE t.portfolio_id = ?
ON DUPLICATE KEY UPDATE
	asset_key = VALUES(asset_key),
	updated_at = NOW()`,
		assetKeyExpr,
	)
	_, err := DB.Exec(networksQuery, portfolioID)
	return err
}

func portfolioHasAsset(portfolioID int64, assetKey string) (bool, error) {
	assetKey = normalizeAssetKey(assetKey, "")
	var cnt int
	query := fmt.Sprintf(`
SELECT COUNT(*)
FROM tokens t
JOIN token_prices tp ON tp.id = t.token_price_id
LEFT JOIN tokens_metadata tm ON tm.chain_id = tp.chain_id AND LOWER(tm.contract) = LOWER(tp.contract)
WHERE t.portfolio_id = ? AND %s = ?`,
		portfolioAssetKeyExprSQL("tp", "tm"),
	)
	err := DB.QueryRow(query, portfolioID, assetKey).Scan(&cnt)
	return cnt > 0, err
}

func HandleGetPortfolioAssets(w http.ResponseWriter, r *http.Request) {
	portfolioID := mustInt64(r.URL.Query().Get("portfolio_id"))
	if portfolioID == 0 {
		http.Error(w, "portfolio_id required", http.StatusBadRequest)
		return
	}

	query := fmt.Sprintf(`
SELECT
	p.onchain_alerts_enabled,
	t.id,
	tp.chain_id,
	LOWER(tp.contract) AS contract,
	%s AS asset_key,
	%s AS asset_symbol,
	t.amount,
	t.invested,
	t.realized,
	t.buy_price_usd,
	IFNULL(tp.price_usd, 0) AS current_price_usd,
	COALESCE(ats.enabled, 1) AS asset_tracking_enabled,
	COALESCE(ats.daily_report_enabled, 0) AS daily_report_enabled,
	COALESCE(NULLIF(ats.profile, ''), 'balanced') AS tracking_profile,
	COALESCE(atn.enabled, 1) AS network_tracking_enabled
FROM tokens t
JOIN portfolios p ON p.id = t.portfolio_id
JOIN token_prices tp ON tp.id = t.token_price_id
LEFT JOIN tokens_metadata tm ON tm.chain_id = tp.chain_id AND LOWER(tm.contract) = LOWER(tp.contract)
LEFT JOIN portfolio_asset_tracking_settings ats
	ON ats.portfolio_id = t.portfolio_id
	AND ats.asset_key = %s
LEFT JOIN portfolio_asset_tracking_networks atn
	ON atn.portfolio_id = t.portfolio_id
	AND atn.chain_id = tp.chain_id
	AND atn.token = LOWER(tp.contract)
WHERE t.portfolio_id = ?
ORDER BY asset_symbol ASC, tp.chain_id ASC, contract ASC`,
		portfolioAssetKeyExprSQL("tp", "tm"),
		portfolioAssetSymbolExprSQL("tp", "tm"),
		portfolioAssetKeyExprSQL("tp", "tm"),
	)

	rows, err := DB.Query(query, portfolioID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	assetsByKey := make(map[string]*PortfolioAssetDTO)
	order := make([]string, 0)

	for rows.Next() {
		var (
			portfolioTrackingEnabled bool
			tokenID                  int64
			chainID                  int64
			contract                 string
			assetKey                 string
			assetSymbol              string
			amount                   float64
			invested                 float64
			realized                 float64
			buyPrice                 float64
			currentPrice             float64
			assetTrackingEnabled     bool
			dailyReportEnabled       bool
			trackingProfile          string
			networkTrackingEnabled   bool
		)

		if err := rows.Scan(
			&portfolioTrackingEnabled,
			&tokenID,
			&chainID,
			&contract,
			&assetKey,
			&assetSymbol,
			&amount,
			&invested,
			&realized,
			&buyPrice,
			&currentPrice,
			&assetTrackingEnabled,
			&dailyReportEnabled,
			&trackingProfile,
			&networkTrackingEnabled,
		); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if _, ok := assetsByKey[assetKey]; !ok {
			assetsByKey[assetKey] = &PortfolioAssetDTO{
				AssetKey:                 assetKey,
				Symbol:                   assetSymbol,
				PortfolioTrackingEnabled: portfolioTrackingEnabled,
				AssetTrackingEnabled:     assetTrackingEnabled,
				TrackingEnabled:          portfolioTrackingEnabled && assetTrackingEnabled,
				DailyReportEnabled:       dailyReportEnabled,
				TrackingProfile:          normalizeTrackingProfile(trackingProfile),
				Networks:                 []PortfolioAssetNetworkDTO{},
			}
			order = append(order, assetKey)
		}

		asset := assetsByKey[assetKey]
		value := currentPrice * amount
		totalPnL := value - invested + realized
		effectiveNetworkTracking := asset.TrackingEnabled && networkTrackingEnabled

		asset.TotalAmount += amount
		asset.TotalInvested += invested
		asset.TotalValue += value
		asset.TotalRealized += realized
		asset.TotalPnL += totalPnL
		asset.NetworkCount++
		if effectiveNetworkTracking {
			asset.TrackedNetworks++
		}

		asset.Networks = append(asset.Networks, PortfolioAssetNetworkDTO{
			ID:                     tokenID,
			ChainID:                chainID,
			Chain:                  ChainName(int(chainID)),
			Contract:               contract,
			Symbol:                 assetSymbol,
			Amount:                 amount,
			Invested:               invested,
			Realized:               realized,
			BuyPriceUSD:            buyPrice,
			CurrentPriceUSD:        currentPrice,
			Value:                  value,
			TotalPnL:               totalPnL,
			NetworkTrackingEnabled: networkTrackingEnabled,
			TrackingEnabled:        effectiveNetworkTracking,
		})
	}

	out := make([]PortfolioAssetDTO, 0, len(order))
	for _, key := range order {
		out = append(out, *assetsByKey[key])
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func HandleUpsertPortfolioAssetTracking(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PortfolioID        int64  `json:"portfolio_id"`
		AssetKey           string `json:"asset_key"`
		AssetSymbol        string `json:"asset_symbol"`
		Enabled            *bool  `json:"enabled"`
		DailyReportEnabled *bool  `json:"daily_report_enabled"`
		Profile            string `json:"profile"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.AssetKey = normalizeAssetKey(req.AssetKey, "")
	req.AssetSymbol = strings.ToUpper(strings.TrimSpace(req.AssetSymbol))
	if req.PortfolioID == 0 || req.AssetKey == "" {
		http.Error(w, "portfolio_id and asset_key required", http.StatusBadRequest)
		return
	}

	ok, err := portfolioHasAsset(req.PortfolioID, req.AssetKey)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !ok {
		http.Error(w, "asset not found in portfolio", http.StatusNotFound)
		return
	}

	currentEnabled := true
	currentDaily := false
	currentProfile := "balanced"
	err = DB.QueryRow(`
SELECT enabled, daily_report_enabled, profile
FROM portfolio_asset_tracking_settings
WHERE portfolio_id = ? AND asset_key = ?
LIMIT 1
`, req.PortfolioID, req.AssetKey).Scan(&currentEnabled, &currentDaily, &currentProfile)
	existing := err == nil
	if err != nil && err != sql.ErrNoRows {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	enabled := currentEnabled
	if req.Enabled != nil {
		enabled = *req.Enabled
	}
	daily := currentDaily
	if req.DailyReportEnabled != nil {
		daily = *req.DailyReportEnabled
	}
	profile := currentProfile
	if !existing {
		profile = "balanced"
	}
	if req.Profile != "" {
		profile = normalizeTrackingProfile(req.Profile)
	}

	rule := trackingProfileDefaults(profile)
	if req.AssetSymbol == "" {
		req.AssetSymbol = req.AssetKey
	}

	if _, err := DB.Exec(`
INSERT INTO portfolio_asset_tracking_settings
	(portfolio_id, asset_key, asset_symbol, enabled, daily_report_enabled, profile, min_usd, spike_mult, dominance_pct, updated_at)
VALUES
	(?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
ON DUPLICATE KEY UPDATE
	asset_symbol = VALUES(asset_symbol),
	enabled = VALUES(enabled),
	daily_report_enabled = VALUES(daily_report_enabled),
	profile = VALUES(profile),
	min_usd = VALUES(min_usd),
	spike_mult = VALUES(spike_mult),
	dominance_pct = VALUES(dominance_pct),
	updated_at = NOW()
`, req.PortfolioID, req.AssetKey, req.AssetSymbol, enabled, daily, profile, rule.MinUSD, rule.SpikeMult, rule.DominancePct); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = EnsurePortfolioAssetTrackingDefaults(req.PortfolioID)
	w.WriteHeader(http.StatusOK)
}

func HandleUpsertPortfolioAssetNetworkTracking(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PortfolioID int64  `json:"portfolio_id"`
		AssetKey    string `json:"asset_key"`
		ChainID     int64  `json:"chain_id"`
		Token       string `json:"token"`
		Enabled     bool   `json:"enabled"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	req.AssetKey = normalizeAssetKey(req.AssetKey, "")
	req.Token = strings.ToLower(strings.TrimSpace(req.Token))
	if req.PortfolioID == 0 || req.AssetKey == "" || req.ChainID == 0 || req.Token == "" {
		http.Error(w, "portfolio_id, asset_key, chain_id and token required", http.StatusBadRequest)
		return
	}

	if _, err := DB.Exec(`
INSERT INTO portfolio_asset_tracking_networks
	(portfolio_id, asset_key, chain_id, token, enabled, updated_at)
VALUES
	(?, ?, ?, ?, ?, NOW())
ON DUPLICATE KEY UPDATE
	asset_key = VALUES(asset_key),
	enabled = VALUES(enabled),
	updated_at = NOW()
`, req.PortfolioID, req.AssetKey, req.ChainID, req.Token, req.Enabled); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
