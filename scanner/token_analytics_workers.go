package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"math"
	"math/big"
	"sort"
	"strings"
	"time"
)

// Automated health check for analytics tables
func AnalyticsHealthCheck() {
	tables := []string{"token_hourly_activity", "token_hourly_metrics", "token_transfer_events"}
	for _, table := range tables {
		var count int
		err := DB.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			log.Printf("[HEALTH][%s] ERROR: %v", table, err)
		} else {
			log.Printf("[HEALTH][%s] rows: %d", table, count)
		}
	}
}

// (Removed duplicate StartUnifiedAnalyticsOrchestrator definition)

// Helper: log summary for each stage
func LogAnalyticsStageReport(stage string, cycle int, err error) {
	if err != nil {
		log.Printf("%s Stage '%s' cycle %d ERROR: %v", logPrefix, stage, cycle, err)
	} else {
		log.Printf("%s Stage '%s' cycle %d completed successfully", logPrefix, stage, cycle)
	}
}

// -- 1) Сирі події трансферів (append-only)

// -- 3) Daily метрики (materialized)

// -- 4) Системні правила аномалій на токен (генерує worker)

// -- 5) Виявлені системні аномалії (для дашборду/історії)

/*
========================================================
WORKERS ENTRYPOINT
========================================================
*/

type AnalyticsWorkersConfig struct {
	EnrichInterval      time.Duration // e.g. 2m
	DailyInterval       time.Duration // e.g. 10m
	RuleGenInterval     time.Duration // e.g. 15m
	EnrichLagHours      int64         // how many recent closed hours to ensure materialized
	RuleLookbackHours   int           // e.g. 168 (7d)
	RuleMinSamplesHours int           // e.g. 24 (need at least 24 hourly points)
}

const logPrefix = "[TOKEN_ANALYTICS]"

// Unified orchestrator for analytics workers

func StartUnifiedAnalyticsOrchestrator(ctx context.Context, cfg AnalyticsWorkersConfig) {
	log.Printf("%s Unified analytics orchestrator started: enrich=%s daily=%s rules=%s", logPrefix, cfg.EnrichInterval, cfg.DailyInterval, cfg.RuleGenInterval)

	stages := []struct {
		name     string
		interval time.Duration
		fn       func() error
	}{
		{"HourlyMetrics", cfg.EnrichInterval, func() error {
			return MaterializeRecentHourlyMetrics(cfg.EnrichLagHours)
		}},
		{"DailyMetrics", cfg.DailyInterval, func() error {
			return MaterializeRecentDailyMetrics(3)
		}},
		{"RuleGen", cfg.RuleGenInterval, func() error {
			return GenerateAndUpsertTokenAnomalyRules(cfg.RuleLookbackHours, cfg.RuleMinSamplesHours)
		}},
	}

	for _, stage := range stages {
		go func(stageName string, interval time.Duration, fn func() error) {
			t := time.NewTicker(interval)
			defer t.Stop()
			cycle := 0
			for {
				select {
				case <-ctx.Done():
					log.Printf("%s Stage '%s' stopped", logPrefix, stageName)
					return
				case <-t.C:
					cycle++
					log.Printf("%s Stage '%s' cycle %d started", logPrefix, stageName, cycle)
					err := fn()
					LogAnalyticsStageReport(stageName, cycle, err)
				}
			}
		}(stage.name, stage.interval, stage.fn)
	}
}

/*
========================================================
1) MATERIALIZE HOURLY METRICS
========================================================
*/

func MaterializeRecentHourlyMetrics(lagHours int64) error {
	nowHour := time.Now().UTC().Truncate(time.Hour).Unix()
	// беремо останні N ЗАКРИТИХ годин (не включаючи поточну)
	from := nowHour - lagHours*3600
	to := nowHour - 3600
	log.Printf("%s hourly scan range from=%d to=%d", logPrefix, from, to)
	// знайдемо всі (chain, token, hour) що є в token_hourly_activity у цьому діапазоні,
	// але відсутні у token_hourly_metrics
	q := `
		SELECT a.chain_id, a.token, a.hour_ts
		FROM token_hourly_activity a
		LEFT JOIN token_hourly_metrics m
		ON m.chain_id=a.chain_id AND m.token=a.token AND m.hour_ts=a.hour_ts
		WHERE a.hour_ts BETWEEN ? AND ?
		AND m.hour_ts IS NULL
		`
	rows, err := DB.Query(q, from, to)
	if err != nil {
		return err
	}
	defer rows.Close()

	type key struct {
		chain int64
		token string
		hour  int64
	}
	var todo []key
	for rows.Next() {
		var k key
		if err := rows.Scan(&k.chain, &k.token, &k.hour); err != nil {
			return err
		}
		todo = append(todo, k)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(todo) == 0 {
		log.Printf("%s hourly nothing to materialize", logPrefix)
		return nil
	}
	log.Printf("%s hourly materializing buckets=%d", logPrefix, len(todo))
	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, t := range todo {
		m, err := ComputeHourlyMetricsFromEvents(t.chain, strings.ToLower(t.token), t.hour)
		if err != nil {
			log.Printf("%s hourly compute failed chain=%d token=%s hour=%d err=%v",
				logPrefix, t.chain, t.token, t.hour, err)
			// якщо events відсутні — просто пропускаємо (але зазвичай будуть)
			continue
		}
		if err := DB_UpsertTokenHourlyMetricsTx(tx, m); err != nil {
			return err
		}
	}

	return tx.Commit()
}

type TokenHourlyMetrics struct {
	ChainID int64
	Token   string
	HourTS  int64

	Transfers       int64
	UniqueSenders   int64
	UniqueReceivers int64
	UniqueAddresses int64
	P50Raw          *big.Int
	P95Raw          *big.Int
	P99Raw          *big.Int
	P50USD          sql.NullFloat64
	P95USD          sql.NullFloat64
	P99USD          sql.NullFloat64
	Top1AddrShare   sql.NullFloat64
	Top3AddrShare   sql.NullFloat64
	Top5AddrShare   sql.NullFloat64
	ExchangeShare   sql.NullFloat64
	NetExchangeUSD  sql.NullFloat64
	USDLt100        int64
	USD100_1k       int64
	USD1k_10k       int64
	USD10k_100k     int64
	USDGt100k       int64
}

func ComputeHourlyMetricsFromEvents(chainID int64, tokenLower string, hourTS int64) (TokenHourlyMetrics, error) {
	// базова перевірка
	if tokenLower == "" {
		return TokenHourlyMetrics{}, errors.New("tokenLower empty")
	}

	// 1) counts + uniques
	var transfers sql.NullInt64
	var uniqSenders sql.NullInt64
	var uniqReceivers sql.NullInt64

	q1 := `
SELECT
  COUNT(*) as transfers,
  COUNT(DISTINCT from_addr) as uniq_senders,
  COUNT(DISTINCT to_addr) as uniq_receivers
FROM token_transfer_events
WHERE chain_id=? AND token=? AND hour_ts=?
`
	if err := DB.QueryRow(q1, chainID, tokenLower, hourTS).Scan(&transfers, &uniqSenders, &uniqReceivers); err != nil {
		return TokenHourlyMetrics{}, err
	}
	if !transfers.Valid || transfers.Int64 == 0 {
		return TokenHourlyMetrics{}, sql.ErrNoRows
	}

	// unique addresses total via UNION (from + to)
	var uniqAddr sql.NullInt64
	q2 := `
SELECT COUNT(DISTINCT addr) FROM (
  SELECT from_addr as addr FROM token_transfer_events WHERE chain_id=? AND token=? AND hour_ts=?
  UNION ALL
  SELECT to_addr as addr FROM token_transfer_events WHERE chain_id=? AND token=? AND hour_ts=?
) x
`
	_ = DB.QueryRow(q2, chainID, tokenLower, hourTS, chainID, tokenLower, hourTS).Scan(&uniqAddr)

	// 2) gather amounts (raw+usd) for quantiles + buckets
	// максимальний результат -> в Go рахуємо квантилі точно
	// (якщо в годині дуже багато записів, це може бути важко — але це “максимум” по точності)
	q3 := `
SELECT amount_raw, amount_usd
FROM token_transfer_events
WHERE chain_id=? AND token=? AND hour_ts=?
`
	rows, err := DB.Query(q3, chainID, tokenLower, hourTS)
	if err != nil {
		return TokenHourlyMetrics{}, err
	}
	defer rows.Close()

	var raws []*big.Int
	var usds []float64
	var usdValidCount int

	var bucketLt100, bucket100_1k, bucket1k_10k, bucket10k_100k, bucketGt100k int64

	for rows.Next() {
		var rawStr string
		var usd sql.NullFloat64
		if err := rows.Scan(&rawStr, &usd); err != nil {
			return TokenHourlyMetrics{}, err
		}
		if v, ok := new(big.Int).SetString(strings.TrimSpace(rawStr), 10); ok {
			raws = append(raws, v)
		} else {
			raws = append(raws, big.NewInt(0))
		}

		if usd.Valid {
			usds = append(usds, usd.Float64)
			usdValidCount++

			switch {
			case usd.Float64 < 100:
				bucketLt100++
			case usd.Float64 < 1000:
				bucket100_1k++
			case usd.Float64 < 10000:
				bucket1k_10k++
			case usd.Float64 < 100000:
				bucket10k_100k++
			default:
				bucketGt100k++
			}
		}
	}
	if err := rows.Err(); err != nil {
		return TokenHourlyMetrics{}, err
	}

	p50r, p95r, p99r := quantilesBigInt(raws)
	p50u, p95u, p99u := quantilesFloat(usds)

	// 3) concentration: top1/top3/top5 share (by USD if possible else RAW)
	// беремо sum per address (from + to) як “address involvement volume”
	topShares, err := computeTopAddrShares(chainID, tokenLower, hourTS)
	if err != nil {
		// ignore, keep nulls
	}

	// 4) exchange share & net exchange USD (від token_hourly_activity бо там уже агрегація in/out)
	exShare, netExUSD := loadExchangeShareAndNetUSD(chainID, tokenLower, hourTS)

	m := TokenHourlyMetrics{
		ChainID:         chainID,
		Token:           tokenLower,
		HourTS:          hourTS,
		Transfers:       transfers.Int64,
		UniqueSenders:   nzInt64(uniqSenders),
		UniqueReceivers: nzInt64(uniqReceivers),
		UniqueAddresses: nzInt64(uniqAddr),
		P50Raw:          p50r,
		P95Raw:          p95r,
		P99Raw:          p99r,
		P50USD:          p50u,
		P95USD:          p95u,
		P99USD:          p99u,
		Top1AddrShare:   topShares.Top1,
		Top3AddrShare:   topShares.Top3,
		Top5AddrShare:   topShares.Top5,
		ExchangeShare:   exShare,
		NetExchangeUSD:  netExUSD,
		USDLt100:        bucketLt100,
		USD100_1k:       bucket100_1k,
		USD1k_10k:       bucket1k_10k,
		USD10k_100k:     bucket10k_100k,
		USDGt100k:       bucketGt100k,
	}
	return m, nil
}

type TopShares struct {
	Top1 sql.NullFloat64
	Top3 sql.NullFloat64
	Top5 sql.NullFloat64
}

func computeTopAddrShares(chainID int64, tokenLower string, hourTS int64) (TopShares, error) {
	// total involvement volume (usd preferred)
	var totalUSD sql.NullFloat64
	var totalRawStr sql.NullString
	qTotal := `
SELECT
  SUM(amount_usd) as total_usd,
  SUM(amount_raw) as total_raw
FROM token_transfer_events
WHERE chain_id=? AND token=? AND hour_ts=?
`
	_ = DB.QueryRow(qTotal, chainID, tokenLower, hourTS).Scan(&totalUSD, &totalRawStr)

	useUSD := totalUSD.Valid && totalUSD.Float64 > 0
	var total float64
	if useUSD {
		total = totalUSD.Float64
	} else {
		if !totalRawStr.Valid {
			return TopShares{}, nil
		}
		tr, ok := new(big.Int).SetString(strings.TrimSpace(totalRawStr.String), 10)
		if !ok || tr.Sign() <= 0 {
			return TopShares{}, nil
		}
		// raw -> float is lossy, але shares все одно відносні; для максимуму можна було б через Rat,
		// але це важко в SQL. Лишаємо float.
		total = bigIntToFloat(tr)
	}

	// per address sum (from+to)
	q := `
SELECT addr, SUM(v) as vol
FROM (
  SELECT from_addr as addr, ` + chooseMetricExpr(useUSD) + ` as v
  FROM token_transfer_events
  WHERE chain_id=? AND token=? AND hour_ts=?
  UNION ALL
  SELECT to_addr as addr, ` + chooseMetricExpr(useUSD) + ` as v
  FROM token_transfer_events
  WHERE chain_id=? AND token=? AND hour_ts=?
) x
GROUP BY addr
ORDER BY vol DESC
LIMIT 5
`
	rows, err := DB.Query(q, chainID, tokenLower, hourTS, chainID, tokenLower, hourTS)
	if err != nil {
		return TopShares{}, err
	}
	defer rows.Close()

	var vols []float64
	for rows.Next() {
		var addr string
		var vol sql.NullFloat64
		if err := rows.Scan(&addr, &vol); err != nil {
			return TopShares{}, err
		}
		if vol.Valid {
			vols = append(vols, vol.Float64)
		}
	}
	if len(vols) == 0 || total <= 0 {
		return TopShares{}, nil
	}
	share := func(n int) sql.NullFloat64 {
		if n > len(vols) {
			n = len(vols)
		}
		var s float64
		for i := 0; i < n; i++ {
			s += vols[i]
		}
		return sql.NullFloat64{Valid: true, Float64: s / total}
	}
	return TopShares{Top1: share(1), Top3: share(3), Top5: share(5)}, nil
}

func chooseMetricExpr(useUSD bool) string {
	if useUSD {
		return "IFNULL(amount_usd,0)"
	}
	// amount_raw stored as DECIMAL(78,0) -> MySQL can SUM as DECIMAL; cast to DOUBLE for share calc
	return "CAST(amount_raw AS DOUBLE)"
}

func loadExchangeShareAndNetUSD(chainID int64, tokenLower string, hourTS int64) (sql.NullFloat64, sql.NullFloat64) {
	// беремо з token_hourly_activity (там already computed)
	q := `
SELECT total_volume_usd, exchange_in_usd, exchange_out_usd
FROM token_hourly_activity
WHERE chain_id=? AND token=? AND hour_ts=?
LIMIT 1
`
	var total sql.NullFloat64
	var inU sql.NullFloat64
	var outU sql.NullFloat64
	if err := DB.QueryRow(q, chainID, tokenLower, hourTS).Scan(&total, &inU, &outU); err != nil {
		return sql.NullFloat64{Valid: false}, sql.NullFloat64{Valid: false}
	}
	if total.Valid && total.Float64 > 0 && inU.Valid && outU.Valid {
		exVol := inU.Float64 + outU.Float64
		exShare := sql.NullFloat64{Valid: true, Float64: exVol / total.Float64}
		net := sql.NullFloat64{Valid: true, Float64: inU.Float64 - outU.Float64}
		return exShare, net
	}
	return sql.NullFloat64{Valid: false}, sql.NullFloat64{Valid: false}
}

func DB_UpsertTokenHourlyMetricsTx(tx *sql.Tx, m TokenHourlyMetrics) error {
	q := `
INSERT INTO token_hourly_metrics
(chain_id, token, hour_ts,
 transfers, unique_senders, unique_receivers, unique_addresses,
 p50_raw, p95_raw, p99_raw,
 p50_usd, p95_usd, p99_usd,
 top1_addr_share, top3_addr_share, top5_addr_share,
 exchange_share, net_exchange_usd,
 usd_lt_100, usd_100_1k, usd_1k_10k, usd_10k_100k, usd_gt_100k,
 updated_at)
VALUES
(?, ?, ?,
 ?, ?, ?, ?,
 ?, ?, ?,
 ?, ?, ?,
 ?, ?, ?,
 ?, ?,
 ?, ?, ?, ?, ?,
 NOW())
ON DUPLICATE KEY UPDATE
 transfers=VALUES(transfers),
 unique_senders=VALUES(unique_senders),
 unique_receivers=VALUES(unique_receivers),
 unique_addresses=VALUES(unique_addresses),
 p50_raw=VALUES(p50_raw),
 p95_raw=VALUES(p95_raw),
 p99_raw=VALUES(p99_raw),
 p50_usd=VALUES(p50_usd),
 p95_usd=VALUES(p95_usd),
 p99_usd=VALUES(p99_usd),
 top1_addr_share=VALUES(top1_addr_share),
 top3_addr_share=VALUES(top3_addr_share),
 top5_addr_share=VALUES(top5_addr_share),
 exchange_share=VALUES(exchange_share),
 net_exchange_usd=VALUES(net_exchange_usd),
 usd_lt_100=VALUES(usd_lt_100),
 usd_100_1k=VALUES(usd_100_1k),
 usd_1k_10k=VALUES(usd_1k_10k),
 usd_10k_100k=VALUES(usd_10k_100k),
 usd_gt_100k=VALUES(usd_gt_100k),
 updated_at=NOW()
`
	var p50r, p95r, p99r interface{} = nil, nil, nil
	if m.P50Raw != nil {
		p50r = m.P50Raw.String()
	}
	if m.P95Raw != nil {
		p95r = m.P95Raw.String()
	}
	if m.P99Raw != nil {
		p99r = m.P99Raw.String()
	}

	var p50u, p95u, p99u interface{} = nil, nil, nil
	if m.P50USD.Valid {
		p50u = m.P50USD.Float64
	}
	if m.P95USD.Valid {
		p95u = m.P95USD.Float64
	}
	if m.P99USD.Valid {
		p99u = m.P99USD.Float64
	}

	var top1, top3, top5 interface{} = nil, nil, nil
	if m.Top1AddrShare.Valid {
		top1 = m.Top1AddrShare.Float64
	}
	if m.Top3AddrShare.Valid {
		top3 = m.Top3AddrShare.Float64
	}
	if m.Top5AddrShare.Valid {
		top5 = m.Top5AddrShare.Float64
	}

	var exShare, netEx interface{} = nil, nil
	if m.ExchangeShare.Valid {
		exShare = m.ExchangeShare.Float64
	}
	if m.NetExchangeUSD.Valid {
		netEx = m.NetExchangeUSD.Float64
	}

	_, err := tx.Exec(q,
		m.ChainID, m.Token, m.HourTS,
		m.Transfers, m.UniqueSenders, m.UniqueReceivers, m.UniqueAddresses,
		p50r, p95r, p99r,
		p50u, p95u, p99u,
		top1, top3, top5,
		exShare, netEx,
		m.USDLt100, m.USD100_1k, m.USD1k_10k, m.USD10k_100k, m.USDGt100k,
	)
	return err
}

/*
========================================================
2) MATERIALIZE DAILY METRICS
========================================================
*/

func MaterializeRecentDailyMetrics(daysBack int64) error {
	nowDay := time.Now().UTC().Truncate(24 * time.Hour).Unix()
	fromDay := nowDay - daysBack*86400

	// materialize for all tokens that have hourly metrics
	q := `
SELECT DISTINCT chain_id, token, day_ts
FROM token_transfer_events
WHERE day_ts BETWEEN ? AND ?
`
	rows, err := DB.Query(q, fromDay, nowDay)
	if err != nil {
		return err
	}
	defer rows.Close()

	type k struct {
		chain int64
		token string
		day   int64
	}
	var keys []k
	for rows.Next() {
		var kk k
		if err := rows.Scan(&kk.chain, &kk.token, &kk.day); err != nil {
			return err
		}
		keys = append(keys, kk)
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(keys) == 0 {
		return nil
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, kk := range keys {
		dm, err := ComputeDailyMetricsFromHourly(kk.chain, strings.ToLower(kk.token), kk.day)
		if err != nil {
			continue
		}
		if err := DB_UpsertTokenDailyMetricsTx(tx, dm); err != nil {
			return err
		}
	}

	return tx.Commit()
}

type TokenDailyMetrics struct {
	ChainID int64
	Token   string
	DayTS   int64

	Transfers        int64
	VolumeUSD        sql.NullFloat64
	NetExchangeUSD   sql.NullFloat64
	AvgHourlyVolUSD  sql.NullFloat64
	PeakHourlyVolUSD sql.NullFloat64

	AvgUniqueAddrs  int64
	PeakUniqueAddrs int64

	AvgTop1Share  sql.NullFloat64
	PeakTop1Share sql.NullFloat64

	P95TransferUSD sql.NullFloat64
	P99TransferUSD sql.NullFloat64
}

func ComputeDailyMetricsFromHourly(chainID int64, tokenLower string, dayTS int64) (TokenDailyMetrics, error) {
	start := dayTS
	end := dayTS + 23*3600 // inclusive-ish; we will use BETWEEN

	q := `
SELECT
  SUM(m.transfers) as transfers,
  AVG(a.total_volume_usd) as avg_hourly_vol_usd,
  MAX(a.total_volume_usd) as peak_hourly_vol_usd,
  AVG(m.unique_addresses) as avg_unique_addrs,
  MAX(m.unique_addresses) as peak_unique_addrs,
  AVG(m.top1_addr_share) as avg_top1,
  MAX(m.top1_addr_share) as peak_top1,
  MAX(m.p95_usd) as p95_tx_usd,
  MAX(m.p99_usd) as p99_tx_usd,
  SUM(a.total_volume_usd) as day_vol_usd,
  SUM(a.exchange_in_usd) - SUM(a.exchange_out_usd) as day_net_ex_usd
FROM token_hourly_metrics m
JOIN token_hourly_activity a
  ON a.chain_id=m.chain_id AND a.token=m.token AND a.hour_ts=m.hour_ts
WHERE m.chain_id=? AND m.token=? AND m.hour_ts BETWEEN ? AND ?
`
	var transfers sql.NullInt64
	var avgVol, peakVol sql.NullFloat64
	var avgUA sql.NullFloat64
	var peakUA sql.NullInt64
	var avgTop1, peakTop1 sql.NullFloat64
	var p95, p99 sql.NullFloat64
	var dayVol sql.NullFloat64
	var dayNet sql.NullFloat64

	if err := DB.QueryRow(q, chainID, tokenLower, start, end).Scan(
		&transfers,
		&avgVol, &peakVol,
		&avgUA, &peakUA,
		&avgTop1, &peakTop1,
		&p95, &p99,
		&dayVol,
		&dayNet,
	); err != nil {
		return TokenDailyMetrics{}, err
	}
	if !transfers.Valid || transfers.Int64 == 0 {
		return TokenDailyMetrics{}, sql.ErrNoRows
	}

	dm := TokenDailyMetrics{
		ChainID:          chainID,
		Token:            tokenLower,
		DayTS:            dayTS,
		Transfers:        transfers.Int64,
		VolumeUSD:        dayVol,
		NetExchangeUSD:   dayNet,
		AvgHourlyVolUSD:  avgVol,
		PeakHourlyVolUSD: peakVol,
		AvgUniqueAddrs:   int64(math.Round(nzFloat(avgUA))),
		PeakUniqueAddrs:  nzInt64(peakUA),
		AvgTop1Share:     avgTop1,
		PeakTop1Share:    peakTop1,
		P95TransferUSD:   p95,
		P99TransferUSD:   p99,
	}
	return dm, nil
}

func DB_UpsertTokenDailyMetricsTx(tx *sql.Tx, d TokenDailyMetrics) error {
	q := `
INSERT INTO token_daily_metrics
(chain_id, token, day_ts,
 transfers, volume_usd, net_exchange_usd,
 avg_hourly_volume_usd, peak_hourly_volume_usd,
 avg_unique_addresses, peak_unique_addresses,
 avg_top1_share, peak_top1_share,
 p95_transfer_usd, p99_transfer_usd,
 updated_at)
VALUES
(?, ?, ?,
 ?, ?, ?,
 ?, ?,
 ?, ?,
 ?, ?,
 ?, ?,
 NOW())
ON DUPLICATE KEY UPDATE
 transfers=VALUES(transfers),
 volume_usd=VALUES(volume_usd),
 net_exchange_usd=VALUES(net_exchange_usd),
 avg_hourly_volume_usd=VALUES(avg_hourly_volume_usd),
 peak_hourly_volume_usd=VALUES(peak_hourly_volume_usd),
 avg_unique_addresses=VALUES(avg_unique_addresses),
 peak_unique_addresses=VALUES(peak_unique_addresses),
 avg_top1_share=VALUES(avg_top1_share),
 peak_top1_share=VALUES(peak_top1_share),
 p95_transfer_usd=VALUES(p95_transfer_usd),
 p99_transfer_usd=VALUES(p99_transfer_usd),
 updated_at=NOW()
`
	var vol, net, avgVol, peakVol, avgTop, peakTop, p95, p99 interface{} = nil, nil, nil, nil, nil, nil, nil, nil
	if d.VolumeUSD.Valid {
		vol = d.VolumeUSD.Float64
	}
	if d.NetExchangeUSD.Valid {
		net = d.NetExchangeUSD.Float64
	}
	if d.AvgHourlyVolUSD.Valid {
		avgVol = d.AvgHourlyVolUSD.Float64
	}
	if d.PeakHourlyVolUSD.Valid {
		peakVol = d.PeakHourlyVolUSD.Float64
	}
	if d.AvgTop1Share.Valid {
		avgTop = d.AvgTop1Share.Float64
	}
	if d.PeakTop1Share.Valid {
		peakTop = d.PeakTop1Share.Float64
	}
	if d.P95TransferUSD.Valid {
		p95 = d.P95TransferUSD.Float64
	}
	if d.P99TransferUSD.Valid {
		p99 = d.P99TransferUSD.Float64
	}

	_, err := tx.Exec(q,
		d.ChainID, d.Token, d.DayTS,
		d.Transfers, vol, net,
		avgVol, peakVol,
		d.AvgUniqueAddrs, d.PeakUniqueAddrs,
		avgTop, peakTop,
		p95, p99,
	)
	return err
}

/*
========================================================
3) RULE GENERATOR / UPDATER
========================================================
*/

type TokenAnomalyRule struct {
	ChainID int64
	Token   string

	LookbackHours int

	MinTransferUSD sql.NullFloat64
	MinTransferRaw sql.NullString

	SpikeMultHourlyVolume  float64
	SpikeMultHourlyTxCount float64
	DominancePct           float64
	MaxTop1Share           float64

	ExchangeOnly            bool
	NetExchangeUSDSpikeMult float64

	MaxSingleTxSupplyPct sql.NullFloat64
	MaxHourSupplyPct     sql.NullFloat64

	RuleVersion int
	RuleJSON    sql.NullString
}

func GenerateAndUpsertTokenAnomalyRules(lookbackHours int, minSamplesHours int) error {
	log.Printf("%s rulegen start lookback=%dh minSamples=%dh",
		logPrefix, lookbackHours, minSamplesHours)

	// беремо всі tracked tokens
	tracked, err := DB_LoadTrackedTokensAllChains()
	if err != nil {
		return err
	}
	if len(tracked) == 0 {
		log.Printf("%s rulegen no tracked tokens", logPrefix)
		return nil
	}

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	for _, it := range tracked {
		tokenLower := strings.ToLower(it.Token)
		// baseline з daily/hourly materialized metrics
		rule, err := BuildRuleFromMetrics(it.ChainID, tokenLower, lookbackHours, minSamplesHours)
		if err != nil {
			continue
		}
		if err := DB_UpsertTokenAnomalyRuleTx(tx, rule); err != nil {
			return err
		}
	}

	return tx.Commit()
}

type TrackedTokenItem struct {
	ChainID int64
	Token   string
}

func DB_LoadTrackedTokensAllChains() ([]TrackedTokenItem, error) {
	q := `
SELECT chain_id, LOWER(contract)
FROM tokens_metadata
WHERE onchain_tracking = 1
`
	rows, err := DB.Query(q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TrackedTokenItem
	for rows.Next() {
		var it TrackedTokenItem
		if err := rows.Scan(&it.ChainID, &it.Token); err != nil {
			return nil, err
		}
		out = append(out, it)
	}
	return out, rows.Err()
}

func BuildRuleFromMetrics(chainID int64, tokenLower string, lookbackHours int, minSamplesHours int) (TokenAnomalyRule, error) {
	nowHour := time.Now().UTC().Truncate(time.Hour).Unix()
	from := nowHour - int64(lookbackHours)*3600
	to := nowHour - 3600

	// 1) hourly baseline: avg volume usd, avg txcount, avg top1 share, avg unique addrs
	q := `
SELECT
  COUNT(*) as n,
  AVG(a.total_volume_usd) as avg_vol_usd,
  STDDEV_POP(a.total_volume_usd) as sd_vol_usd,
  AVG(a.transfer_count) as avg_tx,
  STDDEV_POP(a.transfer_count) as sd_tx,
  AVG(m.top1_addr_share) as avg_top1,
  AVG(m.unique_addresses) as avg_ua,
  MAX(m.p95_usd) as max_p95_tx_usd,
  MAX(m.p99_usd) as max_p99_tx_usd,
  AVG(m.exchange_share) as avg_ex_share,
  STDDEV_POP(m.net_exchange_usd) as sd_net_ex,
  AVG(m.net_exchange_usd) as avg_net_ex
FROM token_hourly_activity a
LEFT JOIN token_hourly_metrics m
  ON m.chain_id=a.chain_id AND m.token=a.token AND m.hour_ts=a.hour_ts
WHERE a.chain_id=? AND a.token=? AND a.hour_ts BETWEEN ? AND ?
`
	var n sql.NullInt64
	var avgVol, sdVol sql.NullFloat64
	var avgTx, sdTx sql.NullFloat64
	var avgTop1 sql.NullFloat64
	var avgUA sql.NullFloat64
	var maxP95, maxP99 sql.NullFloat64
	var avgExShare sql.NullFloat64
	var sdNetEx sql.NullFloat64
	var avgNetEx sql.NullFloat64

	if err := DB.QueryRow(q, chainID, tokenLower, from, to).Scan(
		&n,
		&avgVol, &sdVol,
		&avgTx, &sdTx,
		&avgTop1, &avgUA,
		&maxP95, &maxP99,
		&avgExShare,
		&sdNetEx, &avgNetEx,
	); err != nil {
		return TokenAnomalyRule{}, err
	}
	if !n.Valid || int(n.Int64) < minSamplesHours {
		return TokenAnomalyRule{}, fmt.Errorf("not enough samples hours n=%v", n.Int64)
	}

	// 2) token profile for supply-aware thresholds
	profile, _ := DB_LoadTokenProfile(chainID, tokenLower)

	// 3) build thresholds (максимум, але стабільні)
	// MinTransferUSD: якщо є p95/p99 — беремо max(p95*0.8, 250) і max(p99*0.5, 500) -> max
	minUSD := sql.NullFloat64{Valid: false}
	if maxP95.Valid || maxP99.Valid {
		var v float64
		if maxP95.Valid {
			v = math.Max(v, maxP95.Float64*0.8)
			v = math.Max(v, 250)
		}
		if maxP99.Valid {
			v = math.Max(v, maxP99.Float64*0.5)
			v = math.Max(v, 500)
		}
		if v > 0 {
			minUSD = sql.NullFloat64{Valid: true, Float64: v}
		}
	}

	// Spike mult: 2 + k*(sd/avg) + floor, clamped
	spikeVol := 5.0
	if avgVol.Valid && avgVol.Float64 > 0 && sdVol.Valid {
		r := sdVol.Float64 / avgVol.Float64
		spikeVol = clamp(3.0, 12.0, 2.5+3.0*r)
	}
	spikeTx := 4.0
	if avgTx.Valid && avgTx.Float64 > 0 && sdTx.Valid {
		r := sdTx.Float64 / avgTx.Float64
		spikeTx = clamp(2.0, 10.0, 2.0+3.0*r)
	}

	// DominancePct: базово 0.25, але якщо avgTop1 дуже високий — піднімаємо поріг
	dom := 0.25
	if avgTop1.Valid {
		dom = clamp(0.15, 0.60, avgTop1.Float64*0.7)
	}

	// MaxTop1Share threshold (концентрація в годині): avgTop1 + 0.2
	maxTop1 := 0.60
	if avgTop1.Valid {
		maxTop1 = clamp(0.40, 0.95, avgTop1.Float64+0.20)
	}

	// Net exchange USD spike: використовуємо sd_net_ex; якщо sd велика — робимо stricter
	netExSpike := 4.0
	if sdNetEx.Valid && avgVol.Valid && avgVol.Float64 > 0 {
		netExSpike = clamp(2.0, 12.0, 2.0+3.0*(math.Abs(sdNetEx.Float64)/avgVol.Float64))
	}

	// supply-aware
	var maxSingleSupply, maxHourSupply sql.NullFloat64
	if profile != nil && profile.MaxTotalSupplyRaw.Valid {
		// дефолт: single tx > 0.2% supply OR hour > 0.8% supply
		maxSingleSupply = sql.NullFloat64{Valid: true, Float64: 0.002}
		maxHourSupply = sql.NullFloat64{Valid: true, Float64: 0.008}
	}

	// rule json (for dashboard + trace)
	j, _ := json.Marshal(map[string]any{
		"avg_hourly_vol_usd": avgVol,
		"sd_hourly_vol_usd":  sdVol,
		"avg_hourly_tx":      avgTx,
		"avg_top1_share":     avgTop1,
		"avg_unique_addrs":   avgUA,
		"max_p95_tx_usd":     maxP95,
		"max_p99_tx_usd":     maxP99,
		"avg_exchange_share": avgExShare,
	})

	rule := TokenAnomalyRule{
		ChainID:        chainID,
		Token:          tokenLower,
		LookbackHours:  lookbackHours,
		MinTransferUSD: minUSD,
		MinTransferRaw: sql.NullString{Valid: false},

		SpikeMultHourlyVolume:  spikeVol,
		SpikeMultHourlyTxCount: spikeTx,
		DominancePct:           dom,
		MaxTop1Share:           maxTop1,

		ExchangeOnly:            false, // максимум охоплення; якщо захочеш — зробимо авто
		NetExchangeUSDSpikeMult: netExSpike,

		MaxSingleTxSupplyPct: maxSingleSupply,
		MaxHourSupplyPct:     maxHourSupply,

		RuleVersion: 1,
		RuleJSON:    sql.NullString{Valid: true, String: string(j)},
	}
	return rule, nil
}

func DB_UpsertTokenAnomalyRuleTx(tx *sql.Tx, r TokenAnomalyRule) error {

	q := `
INSERT INTO token_anomaly_rules
(chain_id, token,
 lookback_hours,
 min_transfer_usd, min_transfer_raw,
 spike_mult_hourly_volume, spike_mult_hourly_txcount,
 dominance_pct,
 max_top1_share,
 exchange_only,
 net_exchange_usd_spike_mult,
 max_single_tx_supply_pct, max_hour_supply_pct,
 rule_version,
 rule_json,
 updated_at)
VALUES
(?, ?,
 ?,
 ?, ?,
 ?, ?,
 ?,
 ?,
 ?,
 ?,
 ?, ?,
 ?,
 CAST(? AS JSON),
 NOW())
ON DUPLICATE KEY UPDATE
 lookback_hours=VALUES(lookback_hours),
 min_transfer_usd=VALUES(min_transfer_usd),
 min_transfer_raw=VALUES(min_transfer_raw),
 spike_mult_hourly_volume=VALUES(spike_mult_hourly_volume),
 spike_mult_hourly_txcount=VALUES(spike_mult_hourly_txcount),
 dominance_pct=VALUES(dominance_pct),
 max_top1_share=VALUES(max_top1_share),
 exchange_only=VALUES(exchange_only),
 net_exchange_usd_spike_mult=VALUES(net_exchange_usd_spike_mult),
 max_single_tx_supply_pct=VALUES(max_single_tx_supply_pct),
 max_hour_supply_pct=VALUES(max_hour_supply_pct),
 rule_version=VALUES(rule_version),
 rule_json=VALUES(rule_json),
 updated_at=NOW()
`
	var minUSD interface{} = nil
	var minRaw interface{} = nil
	if r.MinTransferUSD.Valid {
		minUSD = r.MinTransferUSD.Float64
	}
	if r.MinTransferRaw.Valid {
		minRaw = r.MinTransferRaw.String
	}
	var singleSupply interface{} = nil
	var hourSupply interface{} = nil
	if r.MaxSingleTxSupplyPct.Valid {
		singleSupply = r.MaxSingleTxSupplyPct.Float64
	}
	if r.MaxHourSupplyPct.Valid {
		hourSupply = r.MaxHourSupplyPct.Float64
	}

	_, err := tx.Exec(q,
		r.ChainID, r.Token,
		r.LookbackHours,
		minUSD, minRaw,
		r.SpikeMultHourlyVolume, r.SpikeMultHourlyTxCount,
		r.DominancePct,
		r.MaxTop1Share,
		boolToTinyint(r.ExchangeOnly),
		r.NetExchangeUSDSpikeMult,
		singleSupply, hourSupply,
		r.RuleVersion,
		r.RuleJSON.String,
	)

	return err
}

func boolToTinyint(b bool) int {
	if b {
		return 1
	}
	return 0
}

/*
========================================================
HELPERS: quantiles
========================================================
*/

func quantilesFloat(vals []float64) (p50, p95, p99 sql.NullFloat64) {
	if len(vals) == 0 {
		return sql.NullFloat64{Valid: false}, sql.NullFloat64{Valid: false}, sql.NullFloat64{Valid: false}
	}
	sort.Float64s(vals)
	get := func(p float64) float64 {
		if len(vals) == 1 {
			return vals[0]
		}
		// nearest-rank
		k := int(math.Ceil(p*float64(len(vals)))) - 1
		if k < 0 {
			k = 0
		}
		if k >= len(vals) {
			k = len(vals) - 1
		}
		return vals[k]
	}
	return sql.NullFloat64{Valid: true, Float64: get(0.50)},
		sql.NullFloat64{Valid: true, Float64: get(0.95)},
		sql.NullFloat64{Valid: true, Float64: get(0.99)}
}

func quantilesBigInt(vals []*big.Int) (p50, p95, p99 *big.Int) {
	if len(vals) == 0 {
		return nil, nil, nil
	}
	sort.Slice(vals, func(i, j int) bool {
		return vals[i].Cmp(vals[j]) < 0
	})
	get := func(p float64) *big.Int {
		if len(vals) == 1 {
			return new(big.Int).Set(vals[0])
		}
		k := int(math.Ceil(p*float64(len(vals)))) - 1
		if k < 0 {
			k = 0
		}
		if k >= len(vals) {
			k = len(vals) - 1
		}
		return new(big.Int).Set(vals[k])
	}
	return get(0.50), get(0.95), get(0.99)
}

func nzInt64(v sql.NullInt64) int64 {
	if v.Valid {
		return v.Int64
	}
	return 0
}

func nzFloat(v sql.NullFloat64) float64 {
	if v.Valid {
		return v.Float64
	}
	return 0
}

func clamp(lo, hi, x float64) float64 {
	if x < lo {
		return lo
	}
	if x > hi {
		return hi
	}
	return x
}

func bigIntToFloat(x *big.Int) float64 {
	if x == nil {
		return 0
	}
	f, _ := new(big.Rat).SetInt(x).Float64()
	return f
}
