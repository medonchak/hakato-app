package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
)

/*
========================================================
JSON STRUCTURE (all_tokens.json)
========================================================
*/

type AllTokenJSON struct {
	Rank                 int    `json:"rank"`
	Name                 string `json:"name"`
	Symbol               string `json:"symbol"`
	Address              string `json:"address"`
	PriceUSD             string `json:"price_usd"`
	PriceETH             string `json:"price_eth"`
	Change24h            string `json:"change_24h"`
	Volume24h            string `json:"volume_24h"`
	CirculatingMarketCap string `json:"circulating_market_cap"`
	OnchainMarketCap     string `json:"onchain_market_cap"`
	Holders              string `json:"holders"`
	Chain                string `json:"chain"`
}

/*
========================================================
CHAIN RESOLVER
========================================================
*/

func ChainURLToChainID(chain string) int {
	normalized := strings.ToLower(strings.TrimSpace(chain))
	normalized = strings.TrimSuffix(normalized, "/")

	switch {
	case strings.Contains(normalized, "etherscan.io"):
		return 1
	case strings.Contains(normalized, "bscscan.com"):
		return 56
	case strings.Contains(normalized, "basescan.org"):
		return 8453
	default:
		return 0
	}
}

/*
========================================================
VALIDATION
========================================================
*/

func IsValidToken(t AllTokenJSON) bool {
	if t.Address == "" {
		return false
	}
	if t.Symbol == "" {
		return false
	}

	if ChainURLToChainID(t.Chain) == 0 {
		return false
	}
	return true
}

/*
========================================================
LOAD FILE
========================================================
*/

func LoadAllTokens(path string) ([]AllTokenJSON, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var tokens []AllTokenJSON
	if err := json.Unmarshal(data, &tokens); err != nil {
		return nil, err
	}

	return tokens, nil
}

/*
========================================================
DB UPSERT
========================================================
*/

func DBUpsertTokenFromAllTokens(t AllTokenJSON) error {

	chainID := ChainURLToChainID(t.Chain)
	if chainID == 0 {
		return nil
	}

	// ===== NORMALIZE HOLDERS =====

	holders := strings.ReplaceAll(strings.TrimSpace(t.Holders), ",", "")
	if holders == "" {
		holders = "0"
	}
	priceUSD := normalizeNumericString(t.PriceUSD)
	priceETH := normalizeNumericString(t.PriceETH)
	change24h := normalizeNumericString(t.Change24h)

	onchainMarketCap := normalizeMarketCap(t.OnchainMarketCap)
	circulatingMarketCap := normalizeMarketCap(t.CirculatingMarketCap)
	const q = `
	INSERT INTO tokens_metadata (
		chain_id,
		contract,
		symbol,
		holders,
		price_usd,
		price_eth,
		price_change,
		circulating_market_cap,
		onchain_market_cap,
		onchain_tracking
	) VALUES (?,?,?,?,?,?,?,?,?,?)
	ON DUPLICATE KEY UPDATE
		symbol=VALUES(symbol),
		holders=VALUES(holders),
		price_usd=VALUES(price_usd),
		price_eth=VALUES(price_eth),
		price_change=VALUES(price_change),
		circulating_market_cap=VALUES(circulating_market_cap),
		onchain_market_cap=VALUES(onchain_market_cap),
		onchain_tracking=VALUES(onchain_tracking),
		updated_at=NOW()
	`

	_, err := DB.Exec(
		q,
		chainID,
		strings.ToLower(t.Address),
		t.Symbol,
		holders, // ← завжди число
		priceUSD,
		priceETH,
		change24h,
		circulatingMarketCap,
		onchainMarketCap,
		1,
	)

	return err
}

/*
========================================================
MAIN IMPORT FUNCTION
========================================================
*/

func ImportAllTokens(path string) error {
	tokens, err := LoadAllTokens(path)

	if err != nil {
		return err
	}

	count := 0
	valid := 0
	invalid := 0
	failed := 0

	for _, t := range tokens {
		if !IsValidToken(t) {
			invalid++
			continue
		}
		valid++

		if err := DBUpsertTokenFromAllTokens(t); err != nil {
			failed++
			log.Printf("token 11 %s error: %v", t.Address, err)
			continue
		}
		count++
	}

	log.Printf("Imported %d tokens with onchain_tracking=true (valid=%d invalid=%d failed=%d)", count, valid, invalid, failed)
	if valid > 0 && count == 0 {
		return fmt.Errorf("import failed: inserted 0/%d valid rows", valid)
	}
	return nil
}
func normalizeMarketCap(v string) string {
	v = strings.TrimSpace(v)

	// беремо тільки перший рядок (до \n)
	if strings.Contains(v, "\n") {
		v = strings.Split(v, "\n")[0]
	}

	// прибираємо коми
	v = strings.ReplaceAll(v, ",", "")

	// якщо пусто — 0
	if v == "" || v == "--" {
		return "0"
	}

	return v
}

func normalizeNumericString(v string) string {
	v = strings.TrimSpace(v)
	if v == "" || v == "--" {
		return "0"
	}
	if strings.Contains(v, "\n") {
		v = strings.Split(v, "\n")[0]
	}

	v = strings.ReplaceAll(v, ",", "")
	v = strings.ReplaceAll(v, "$", "")
	v = strings.TrimSuffix(v, "%")

	parts := strings.Fields(v)
	if len(parts) > 0 {
		v = parts[0]
	}
	if v == "" {
		return "0"
	}

	if _, err := strconv.ParseFloat(v, 64); err != nil {
		return "0"
	}
	return v
}
