package main

import (
	"database/sql"
	"fmt"
	"math"
	"math/big"
	"strings"
)

type LoadedAnomalyRule struct {
	ChainID int64
	Token   string

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
}

func DB_LoadTokenAnomalyRule(chainID int64, tokenLower string) (*LoadedAnomalyRule, error) {
	q := `
SELECT
  chain_id, token,
  min_transfer_usd, min_transfer_raw,
  spike_mult_hourly_volume, spike_mult_hourly_txcount,
  dominance_pct,
  max_top1_share,
  exchange_only,
  net_exchange_usd_spike_mult,
  max_single_tx_supply_pct, max_hour_supply_pct
FROM token_anomaly_rules
WHERE chain_id=? AND token=?
LIMIT 1
`
	var r LoadedAnomalyRule
	var exOnly sql.NullInt64

	err := DB.QueryRow(q, chainID, tokenLower).Scan(
		&r.ChainID, &r.Token,
		&r.MinTransferUSD, &r.MinTransferRaw,
		&r.SpikeMultHourlyVolume, &r.SpikeMultHourlyTxCount,
		&r.DominancePct,
		&r.MaxTop1Share,
		&exOnly,
		&r.NetExchangeUSDSpikeMult,
		&r.MaxSingleTxSupplyPct, &r.MaxHourSupplyPct,
	)
	if err != nil {
		return nil, err
	}
	r.ExchangeOnly = exOnly.Valid && exOnly.Int64 == 1
	return &r, nil
}

type SystemAnomalyResult struct {
	Matched  bool
	Severity float64
	Reason   string
}

func EvaluateSystemAnomaly(
	ev TokenOnchainEvent,
	ref TokenFilterContext, // має HourTotal/Avg + Profile
	hm *TokenHourlyMetrics, // може бути nil (якщо ще не materialized)
	rule *LoadedAnomalyRule,
) SystemAnomalyResult {
	if rule == nil {
		return SystemAnomalyResult{}
	}

	// 0) Exchange-only gate (якщо увімкнули)
	if rule.ExchangeOnly && ev.ExchangeAddr == "" {
		return SystemAnomalyResult{}
	}

	amountUSD, hasUSD := eventAmountUSD(ev, ref.Profile)

	// 1) Min thresholds (event-level)
	minOK := false
	if rule.MinTransferUSD.Valid && hasUSD && amountUSD >= rule.MinTransferUSD.Float64 {
		minOK = true
	}
	if !minOK && rule.MinTransferRaw.Valid {
		minRaw, ok := new(big.Int).SetString(strings.TrimSpace(rule.MinTransferRaw.String), 10)
		if ok && ev.Amount.Cmp(minRaw) >= 0 {
			minOK = true
		}
	}
	// якщо мін-пороги не задані — не блокуємо (бо “максимум охоплення”)
	if (rule.MinTransferUSD.Valid || rule.MinTransferRaw.Valid) && !minOK {
		return SystemAnomalyResult{}
	}

	// 2) dominance: event share of hour total
	domOK := false
	if rule.DominancePct > 0 && rule.DominancePct <= 1 {
		if hasUSD && ref.HourTotalUSD.Valid && ref.HourTotalUSD.Float64 > 0 {
			if amountUSD/ref.HourTotalUSD.Float64 >= rule.DominancePct {
				domOK = true
			}
		} else if ref.HourTotalRaw != nil && ref.HourTotalRaw.Sign() > 0 {
			thr := new(big.Rat).Mul(new(big.Rat).SetInt(ref.HourTotalRaw), new(big.Rat).SetFloat64(rule.DominancePct))
			thrInt := new(big.Int).Quo(thr.Num(), thr.Denom())
			if ev.Amount.Cmp(thrInt) >= 0 {
				domOK = true
			}
		}
	}

	// 3) spike by hourly volume and txcount (needs hourly avg)
	spikeVolOK := false
	if rule.SpikeMultHourlyVolume > 0 {
		if ref.HourAvgUSD.Valid && ref.HourAvgUSD.Float64 > 0 && ref.HourTotalUSD.Valid {
			if ref.HourTotalUSD.Float64 >= ref.HourAvgUSD.Float64*rule.SpikeMultHourlyVolume {
				spikeVolOK = true
			}
		} else if ref.HourAvgRaw != nil && ref.HourAvgRaw.Sign() > 0 && ref.HourTotalRaw != nil {
			thr := new(big.Rat).Mul(new(big.Rat).SetInt(ref.HourAvgRaw), new(big.Rat).SetFloat64(rule.SpikeMultHourlyVolume))
			thrInt := new(big.Int).Quo(thr.Num(), thr.Denom())
			if ref.HourTotalRaw.Cmp(thrInt) >= 0 {
				spikeVolOK = true
			}
		}
	}

	// txcount spike uses token_hourly_activity.transfer_count; беремо окремо (cheap)
	txCountOK := false
	if rule.SpikeMultHourlyTxCount > 0 {
		cnt, avgCnt := DB_LoadHourlyTxCountAndAvg(ev.ChainID, strings.ToLower(ev.Token.Hex()), ev.HourTS)
		if avgCnt > 0 && float64(cnt) >= float64(avgCnt)*rule.SpikeMultHourlyTxCount {
			txCountOK = true
		}
	}

	// 4) concentration (top1 share) from hourly_metrics if available
	concOK := false
	if hm != nil && hm.Top1AddrShare.Valid && rule.MaxTop1Share > 0 {
		// anomaly if concentration exceeds threshold
		if hm.Top1AddrShare.Float64 >= rule.MaxTop1Share {
			concOK = true
		}
	}

	// 5) supply-aware (optional)
	supplyOK := false
	if ref.Profile != nil && ref.Profile.MaxTotalSupplyRaw.Valid && (rule.MaxSingleTxSupplyPct.Valid || rule.MaxHourSupplyPct.Valid) {
		supRaw, ok := new(big.Int).SetString(strings.TrimSpace(ref.Profile.MaxTotalSupplyRaw.String), 10)
		if ok && supRaw.Sign() > 0 {
			if rule.MaxSingleTxSupplyPct.Valid {
				pct := new(big.Rat).SetFrac(ev.Amount, supRaw)
				f, _ := pct.Float64()
				if f >= rule.MaxSingleTxSupplyPct.Float64 {
					supplyOK = true
				}
			}
			if !supplyOK && rule.MaxHourSupplyPct.Valid && ref.HourTotalRaw != nil && ref.HourTotalRaw.Sign() > 0 {
				pct := new(big.Rat).SetFrac(ref.HourTotalRaw, supRaw)
				f, _ := pct.Float64()
				if f >= rule.MaxHourSupplyPct.Float64 {
					supplyOK = true
				}
			}
		}
	}

	// MATCH POLICY (максимум):
	// якщо спрацювало ХОЧ ЩОСЬ суттєве з сигналів — це аномалія
	matched := domOK || spikeVolOK || txCountOK || concOK || supplyOK

	if !matched {
		return SystemAnomalyResult{}
	}

	// severity score (0..1+): композитний
	severity := 0.0
	reasons := ""
	add := func(ok bool, label string, w float64) {
		if ok {
			severity += w
			if reasons != "" {
				reasons += "; "
			}
			reasons += label
		}
	}

	add(domOK, "dominance", 0.35)
	add(spikeVolOK, "spike_volume", 0.25)
	add(txCountOK, "spike_txcount", 0.20)
	add(concOK, "concentration", 0.20)
	add(supplyOK, "supply_pct", 0.30)

	// normalize
	severity = math.Min(1.5, severity)

	return SystemAnomalyResult{
		Matched:  true,
		Severity: severity,
		Reason:   reasons,
	}
}

func DB_LoadHourlyTxCountAndAvg(chainID int64, tokenLower string, hourTS int64) (cnt int64, avg24 float64) {
	// current hour count
	_ = DB.QueryRow(`SELECT transfer_count FROM token_hourly_activity WHERE chain_id=? AND token=? AND hour_ts=? LIMIT 1`,
		chainID, tokenLower, hourTS).Scan(&cnt)

	// avg last 24h
	var avg sql.NullFloat64
	from := hourTS - 24*3600
	to := hourTS - 3600
	_ = DB.QueryRow(`SELECT AVG(transfer_count) FROM token_hourly_activity WHERE chain_id=? AND token=? AND hour_ts BETWEEN ? AND ?`,
		chainID, tokenLower, from, to).Scan(&avg)

	if avg.Valid {
		avg24 = avg.Float64
	}
	return
}

func DB_InsertTokenAnomalyEvent(ev TokenOnchainEvent, ref TokenFilterContext, res SystemAnomalyResult) error {
	amountUSD, hasUSD := eventAmountUSD(ev, ref.Profile)

	var usd interface{} = nil
	if hasUSD {
		usd = amountUSD
	}

	q := `
INSERT IGNORE INTO token_anomaly_events
(chain_id, token, tx_hash, block_time, hour_ts, day_ts,
 severity, reason,
 amount_raw, amount_usd,
 direction, exchange_name,
 created_at)
VALUES
(?, ?, ?, ?, ?, ?,
 ?, ?,
 ?, ?,
 ?, ?,
 NOW())
`
	_, err := DB.Exec(q,
		ev.ChainID,
		strings.ToLower(ev.Token.Hex()),
		strings.ToLower(ev.TxHash.Hex()),
		ev.BlockTime.UTC(),
		ev.HourTS,
		ev.DayTS,
		res.Severity,
		res.Reason,
		ev.Amount.String(),
		usd,
		string(ev.Direction),
		ev.ExchangeName,
	)
	return err
}

func BuildSystemAnomalyText(ev TokenOnchainEvent, res SystemAnomalyResult) string {
	return fmt.Sprintf("[SYSTEM ANOMALY] token=%s dir=%s ex=%s sev=%.2f reason=%s tx=%s",
		ev.Token.Hex(), string(ev.Direction), ev.ExchangeName, res.Severity, res.Reason, ev.TxHash.Hex(),
	)
}
