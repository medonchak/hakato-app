package main

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"
)

func DB_InsertTokenTransferEvents(
	events []TokenOnchainEvent,
	profiles map[string]*TokenProfile,
) error {

	if len(events) == 0 {
		return nil
	}

	base := `
INSERT INTO token_transfer_events
(
	chain_id,
	token,
	tx_hash,
	block_time,
	hour_ts,
	day_ts,
	from_addr,
	to_addr,
	amount_raw,
	amount_usd,
	direction,
	exchange_addr,
	exchange_name,
	created_at
)
VALUES
`

	values := make([]string, 0, len(events))
	args := make([]interface{}, 0, len(events)*13)

	for _, ev := range events {
		blockTime := ev.BlockTime.UTC().Truncate(time.Second)
		if blockTime.IsZero() {
			blockTime = time.Unix(0, 0).UTC()
		}

		amountRaw := "0"
		if ev.Amount != nil {
			amountRaw = ev.Amount.String()
		}

		direction := normalizeDirection(string(ev.Direction))
		exchangeAddr := normalizeNullableText(ev.ExchangeAddr, 64)
		exchangeName := normalizeNullableText(ev.ExchangeName, 255)

		var amountUSD interface{} = nil

		if profiles != nil {
			tokenLower := strings.ToLower(ev.Token.Hex())
			if p := profiles[tokenLower]; p != nil {
				if usd, ok := eventAmountUSD(ev, p); ok {
					amountUSD = normalizeUSD(usd)
				}
			}
		}

		values = append(values, "(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())")
		args = append(args,
			ev.ChainID,
			strings.ToLower(ev.Token.Hex()),
			strings.ToLower(ev.TxHash.Hex()),
			blockTime,
			ev.HourTS,
			ev.DayTS,
			strings.ToLower(ev.From.Hex()),
			strings.ToLower(ev.To.Hex()),
			amountRaw,
			amountUSD,
			direction,
			exchangeAddr,
			exchangeName,
		)
	}

	q := base + strings.Join(values, ",\n")


	if _, err := DB.Exec(q, args...); err != nil {
		log.Printf("[DEBUG][INSERT][ERROR] %v", err)
		return fmt.Errorf("bulk insert token_transfer_events failed: %w", err)
	}

	return nil
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func normalizeDirection(direction string) string {
	d := strings.ToUpper(strings.TrimSpace(direction))
	switch d {
	case "IN", "OUT", "MIX", "NONE":
		return d
	default:
		return "NONE"
	}
}

func normalizeNullableText(raw string, maxLen int) interface{} {
	v := strings.TrimSpace(raw)
	if v == "" {
		return nil
	}
	if maxLen > 0 && len(v) > maxLen {
		v = v[:maxLen]
	}
	return v
}

func normalizeUSD(usd float64) interface{} {
	if math.IsNaN(usd) || math.IsInf(usd, 0) {
		return nil
	}
	if usd > 1e15 || usd < -1e15 {
		return nil
	}
	return usd
}
