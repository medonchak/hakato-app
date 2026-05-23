package main

import (
	"database/sql"
	"fmt"
	"log"
	"math"
	"sort"
	"time"
)

// OptimResult holds the best-found parameters for a token.
type OptimResult struct {
	ChainID         int64
	Token           string
	TokenSymbol     string
	VWAPPeriod      int     // hours
	BuyThresholdPct float64 // negative: buy when price < VWAP * (1 + threshold)
	SellThresholdPct float64 // positive: sell when price > VWAP * (1 + threshold)
	CooldownHours   int
	Sharpe          float64
	WinRate         float64
	TotalTrades     int
	UpdatedAt       time.Time
}

// HourlyPoint is one row from token_hourly_activity joined with token_prices.
type HourlyPoint struct {
	HourTS       int64
	TotalVolumeUSD float64
	TransferCount  int64
	PriceUSD      float64
}

// RunStrategyOptimizer fetches hourly data for a token and grid-searches
// VWAP period, buy/sell thresholds, returning the combination with best Sharpe ratio.
func RunStrategyOptimizer(chainID int64, tokenAddr, tokenSymbol string) (*OptimResult, error) {
	points, err := loadHourlyPoints(chainID, tokenAddr, 30*24) // last 30 days
	if err != nil {
		return nil, fmt.Errorf("loadHourlyPoints: %w", err)
	}
	if len(points) < 48 {
		return nil, fmt.Errorf("insufficient data: %d hours (need ≥48)", len(points))
	}

	vwapPeriods := []int{4, 6, 8, 12, 14, 16, 24}
	buyThresholds := []float64{-0.5, -1.0, -1.5, -2.0, -2.5, -2.8, -3.0, -3.5}
	sellThresholds := []float64{0.5, 1.0, 1.5, 2.0, 2.5, 3.0, 3.1, 3.5, 4.0}
	cooldowns := []int{1, 2, 4}

	best := OptimResult{
		ChainID:     chainID,
		Token:       tokenAddr,
		TokenSymbol: tokenSymbol,
		Sharpe:      -math.MaxFloat64,
	}

	for _, vp := range vwapPeriods {
		vwaps := computeVWAP(points, vp)
		for _, bt := range buyThresholds {
			for _, st := range sellThresholds {
				for _, cd := range cooldowns {
					sharpe, winRate, trades := backtest(points, vwaps, bt/100, st/100, cd)
					if trades >= 3 && sharpe > best.Sharpe {
						best.Sharpe = sharpe
						best.WinRate = winRate
						best.TotalTrades = trades
						best.VWAPPeriod = vp
						best.BuyThresholdPct = bt
						best.SellThresholdPct = st
						best.CooldownHours = cd
					}
				}
			}
		}
	}

	if best.Sharpe == -math.MaxFloat64 {
		return nil, fmt.Errorf("no valid strategy found for %s", tokenSymbol)
	}

	best.UpdatedAt = time.Now()

	if err := saveOptimResult(&best); err != nil {
		log.Printf("[optimizer] saveOptimResult %s: %v", tokenSymbol, err)
	}

	log.Printf("[optimizer] %s: VWAP=%dh buy=%.1f%% sell=+%.1f%% Sharpe=%.2f WinRate=%.0f%% trades=%d",
		tokenSymbol, best.VWAPPeriod, best.BuyThresholdPct, best.SellThresholdPct,
		best.Sharpe, best.WinRate*100, best.TotalTrades)

	return &best, nil
}

// StartOptimizerDaemon runs the optimizer for all tracked Mantle tokens every 6 hours.
func StartOptimizerDaemon() {
	mantleTokens := []struct{ addr, symbol string }{
		{"native", "MNT"},
		{"0xcda86a272531e8640cd7f1a92c01839911b90bb0", "mETH"},
		{"0x5be26527e817998a7206475496fde1e68957c5a6", "USDY"},
	}

	run := func() {
		for _, tok := range mantleTokens {
			res, err := RunStrategyOptimizer(5000, tok.addr, tok.symbol)
			if err != nil {
				log.Printf("[optimizer] %s: %v", tok.symbol, err)
				continue
			}
			log.Printf("[optimizer] best for %s: VWAP-%dh buy=%.1f%% sell=+%.1f%% Sharpe=%.2f",
				tok.symbol, res.VWAPPeriod, res.BuyThresholdPct, res.SellThresholdPct, res.Sharpe)
		}
	}

	// Run immediately at startup, then every 6 hours.
	go func() {
		run()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			run()
		}
	}()
}

// ─── helpers ─────────────────────────────────────────────────────────────────

func loadHourlyPoints(chainID int64, token string, limit int) ([]HourlyPoint, error) {
	q := `
SELECT
  h.hour_ts,
  COALESCE(h.total_volume_usd, 0),
  h.transfer_count,
  COALESCE(p.price_usd, 0)
FROM token_hourly_activity h
LEFT JOIN token_prices p ON p.chain_id = h.chain_id AND p.contract = h.token
WHERE h.chain_id = ? AND h.token = ?
ORDER BY h.hour_ts DESC
LIMIT ?
`
	rows, err := DB.Query(q, chainID, token, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pts []HourlyPoint
	for rows.Next() {
		var pt HourlyPoint
		if err := rows.Scan(&pt.HourTS, &pt.TotalVolumeUSD, &pt.TransferCount, &pt.PriceUSD); err != nil {
			return nil, err
		}
		pts = append(pts, pt)
	}

	// reverse to chronological order
	for i, j := 0, len(pts)-1; i < j; i, j = i+1, j-1 {
		pts[i], pts[j] = pts[j], pts[i]
	}
	return pts, rows.Err()
}

// computeVWAP returns the volume-weighted average price over a rolling window of `period` hours.
// Index i of the result corresponds to points[i].
func computeVWAP(pts []HourlyPoint, period int) []float64 {
	vwaps := make([]float64, len(pts))
	for i := range pts {
		start := i - period + 1
		if start < 0 {
			start = 0
		}
		sumPV := 0.0
		sumV := 0.0
		for j := start; j <= i; j++ {
			v := pts[j].TotalVolumeUSD
			p := pts[j].PriceUSD
			if v > 0 && p > 0 {
				sumPV += p * v
				sumV += v
			}
		}
		if sumV > 0 {
			vwaps[i] = sumPV / sumV
		}
	}
	return vwaps
}

// backtest simulates a simple long-only VWAP strategy and returns Sharpe, win-rate, trade count.
func backtest(pts []HourlyPoint, vwaps []float64, buyPct, sellPct float64, cooldownHours int) (sharpe, winRate float64, trades int) {
	type position struct {
		entryPrice float64
		entryHour  int
	}

	var pos *position
	var returns []float64
	lastTradeHour := -cooldownHours - 1

	for i, pt := range pts {
		if pt.PriceUSD <= 0 || vwaps[i] <= 0 {
			continue
		}

		deviation := (pt.PriceUSD - vwaps[i]) / vwaps[i]

		if pos == nil {
			// Entry signal: price dips below VWAP by buyPct
			if deviation <= buyPct && i-lastTradeHour >= cooldownHours {
				pos = &position{entryPrice: pt.PriceUSD, entryHour: i}
			}
		} else {
			// Exit signal: price rises above VWAP by sellPct
			if deviation >= sellPct {
				ret := (pt.PriceUSD - pos.entryPrice) / pos.entryPrice
				returns = append(returns, ret)
				trades++
				lastTradeHour = i
				pos = nil
			}
		}
	}

	if len(returns) < 3 {
		return -math.MaxFloat64, 0, trades
	}

	mean := stat_mean(returns)
	std := stat_std(returns, mean)
	if std < 1e-9 {
		return -math.MaxFloat64, 0, trades
	}

	sharpe = mean / std * math.Sqrt(float64(len(returns)))

	wins := 0
	for _, r := range returns {
		if r > 0 {
			wins++
		}
	}
	winRate = float64(wins) / float64(len(returns))

	return sharpe, winRate, trades
}

func stat_mean(xs []float64) float64 {
	s := 0.0
	for _, x := range xs {
		s += x
	}
	return s / float64(len(xs))
}

func stat_std(xs []float64, mean float64) float64 {
	s := 0.0
	for _, x := range xs {
		d := x - mean
		s += d * d
	}
	return math.Sqrt(s / float64(len(xs)))
}

func saveOptimResult(r *OptimResult) error {
	_, err := DB.Exec(`
INSERT INTO agent_strategies
  (chain_id, token, token_symbol, vwap_period, buy_threshold_pct, sell_threshold_pct, cooldown_hours, sharpe, win_rate, total_trades, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
ON DUPLICATE KEY UPDATE
  vwap_period = VALUES(vwap_period),
  buy_threshold_pct = VALUES(buy_threshold_pct),
  sell_threshold_pct = VALUES(sell_threshold_pct),
  cooldown_hours = VALUES(cooldown_hours),
  sharpe = VALUES(sharpe),
  win_rate = VALUES(win_rate),
  total_trades = VALUES(total_trades),
  updated_at = NOW()
`,
		r.ChainID, r.Token, r.TokenSymbol,
		r.VWAPPeriod, r.BuyThresholdPct, r.SellThresholdPct, r.CooldownHours,
		r.Sharpe, r.WinRate, r.TotalTrades,
	)
	return err
}

// DB_LoadBestStrategy loads the latest optimizer result for a token.
func DB_LoadBestStrategy(chainID int64, token string) (*OptimResult, error) {
	q := `
SELECT chain_id, token, token_symbol, vwap_period, buy_threshold_pct, sell_threshold_pct, cooldown_hours, sharpe, win_rate, total_trades, updated_at
FROM agent_strategies
WHERE chain_id = ? AND token = ?
LIMIT 1
`
	var r OptimResult
	err := DB.QueryRow(q, chainID, token).Scan(
		&r.ChainID, &r.Token, &r.TokenSymbol,
		&r.VWAPPeriod, &r.BuyThresholdPct, &r.SellThresholdPct, &r.CooldownHours,
		&r.Sharpe, &r.WinRate, &r.TotalTrades, &r.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return &r, err
}

// DB_ListBestStrategies returns all tokens' best strategies ordered by Sharpe desc.
func DB_ListBestStrategies(chainID int64) ([]OptimResult, error) {
	rows, err := DB.Query(`
SELECT chain_id, token, token_symbol, vwap_period, buy_threshold_pct, sell_threshold_pct, cooldown_hours, sharpe, win_rate, total_trades, updated_at
FROM agent_strategies
WHERE chain_id = ?
ORDER BY sharpe DESC
`, chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []OptimResult
	for rows.Next() {
		var r OptimResult
		if err := rows.Scan(
			&r.ChainID, &r.Token, &r.TokenSymbol,
			&r.VWAPPeriod, &r.BuyThresholdPct, &r.SellThresholdPct, &r.CooldownHours,
			&r.Sharpe, &r.WinRate, &r.TotalTrades, &r.UpdatedAt,
		); err != nil {
			return nil, err
		}
		results = append(results, r)
	}

	sort.Slice(results, func(i, j int) bool { return results[i].Sharpe > results[j].Sharpe })
	return results, rows.Err()
}
