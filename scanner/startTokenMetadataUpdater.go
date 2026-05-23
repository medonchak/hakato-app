package main

import (
	"context"
	"log"
	"math"
	"strings"
	"time"
)

// clampPriceChange limits price_change to DECIMAL(10,4) range to prevent DB overflow.
func clampPriceChange(v *float64) *float64 {
	if v == nil {
		return nil
	}
	clamped := math.Max(-99999.9999, math.Min(99999.9999, *v))
	return &clamped
}

type TrackedToken struct {
	ChainID int
	Address string
}

var tokenMetaUpdaterStarted = false

func StartTokenMetadataUpdater(interval time.Duration) {
	if tokenMetaUpdaterStarted {
		return
	}
	tokenMetaUpdaterStarted = true

	const workerCount = 6

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			log.Println("[TOKEN_META] update tick")

			tokens, err := DB_LoadTrackedTokensForMetadata()
			if err != nil {
				log.Println("[TOKEN_META][ERR] load tokens:", err)
				<-ticker.C
				continue
			}

			jobs := make(chan TrackedToken)
			ctx, cancel := context.WithCancel(context.Background())

			// --- workers ---
			for i := 0; i < workerCount; i++ {
				go func(id int) {
					for t := range jobs {
						if err := UpdateTokenMetadataFromScan(t.ChainID, t.Address); err != nil {
							log.Printf(
								"[TOKEN_META][ERR] worker=%d chain=%d token=%s err=%v",
								id, t.ChainID, t.Address, err,
							)
						}
						time.Sleep(3 * time.Second)
					}
				}(i + 1)
			}

			// --- feed jobs ---
			go func() {
				for _, t := range tokens {
					select {
					case jobs <- t:
					case <-ctx.Done():
						return
					}
				}
				close(jobs)
			}()

			// чекаємо до наступного тіку
			<-ticker.C
			cancel()
		}
	}()
}

func DB_LoadTrackedTokensForMetadata() ([]TrackedToken, error) {
	rows, err := DB.Query(`
		SELECT chain_id, LOWER(contract)
		FROM tokens_metadata
		WHERE onchain_tracking = 1
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []TrackedToken
	for rows.Next() {
		var t TrackedToken
		if err := rows.Scan(&t.ChainID, &t.Address); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func UpdateTokenMetadataFromScan(chainID int, tokenAddr string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	data, err := ParseTokenOverview(chainID, tokenAddr)
	if err != nil {
		return err
	}

	_, err = DB.ExecContext(ctx, `
		UPDATE tokens_metadata SET
			symbol = ?,
			max_total_supply = ?,
			holders = ?,
			holders_change = ?,
			transfers_total = ?,
			transfers_24h = ?,
			price_usd = ?,
			price_eth = ?,
			price_change = ?,
			onchain_market_cap = ?,
			circulating_market_cap = ?,
			decimals = ?,
			updated_at = NOW()
		WHERE chain_id = ? AND LOWER(contract) = ?
		`,
		data.Symbol,
		data.MaxTotalSupply,
		data.Holders,
		data.HoldersChange,
		data.TransfersTotal,
		data.Transfers24h,
		data.PriceUSD,
		data.PriceETH,
		clampPriceChange(data.PriceChange),
		data.OnchainMarketCap,
		data.CirculatingMarketCap,
		data.Decimals,
		data.ChainID,
		strings.ToLower(data.Address),
	)

	return err
}
