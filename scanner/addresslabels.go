package main

// import (
// 	"context"
// 	"strings"
// )

/*
===========================================================
ВХІДНИЙ ТИП ТРАНЗАКЦІЇ
===========================================================
*/

/*
===========================================================
DB (ініціалізується ззовні)
===========================================================
*/

/*
===========================================================
ТАБЛИЦЯ

CREATE TABLE address_labels (
    chain_id   INT NOT NULL,
    address    VARCHAR(42) NOT NULL,
    label      VARCHAR(128),
    entity     VARCHAR(64),
    source     VARCHAR(32),
    checked_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (chain_id, address)
);
===========================================================
*/

/*
===========================================================
ТОЧКА ВХОДУ
===========================================================
*/

// func ProcessTxAddressesLabels(
// 	ctx context.Context,
// 	chainID int,
// 	txs []TxJSON,
// ) {
// 	seen := make(map[string]struct{})

// 	for _, tx := range txs {

// 		if addr := normalizeAddress(tx.From); addr != "" {
// 			seen[addr] = struct{}{}
// 		}

// 		if addr := normalizeAddress(tx.To); addr != "" {
// 			seen[addr] = struct{}{}
// 		}
// 	}

// 	for addr := range seen {
// 		resolveAndStoreAddressLabel(ctx, chainID, addr)
// 	}
// }

/*
===========================================================
АДРЕСА → НОРМАЛІЗАЦІЯ
===========================================================
*/

// func normalizeAddress(addr string) string {
// 	addr = strings.ToLower(strings.TrimSpace(addr))
// 	if addr == "" {
// 		return ""
// 	}
// 	if !strings.HasPrefix(addr, "0x") || len(addr) != 42 {
// 		return ""
// 	}
// 	return addr
// }

/*
===========================================================
ГОЛОВНА ЛОГІКА
===========================================================
*/

// func resolveAndStoreAddressLabel(
// 	ctx context.Context,
// 	chainID int,
// 	address string,
// ) {

// 	if dbAddressExists(chainID, address) {
// 		return
// 	}

// 	label, entity, err := fetchAddressLabelV2(ctx, chainID, address)
// 	if err != nil {
// 		dbInsertAddressLabel(chainID, address, "", "Unknown", "etherscan_v2")
// 		return
// 	}

// 	dbInsertAddressLabel(chainID, address, label, entity, "etherscan_v2")
// }

/*
===========================================================
ETHERSCAN API V2
ОДИН ENDPOINT / ОДИН KEY / chainid
===========================================================
*/

// func fetchAddressLabelV2(
// 	ctx context.Context,
// 	chainID int,
// 	address string,
// ) (label string, entity string, err error) {

// 	baseURL := "https://api.etherscan.io/v2/api"

// 	reqURL := baseURL +
// 		"?chainid=" + itoa(chainID) +
// 		"&module=account" +
// 		"&action=balance" +
// 		"&address=" + address +
// 		"&apikey=" + getEtherscanV2Key()

// 	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
// 	client := &http.Client{Timeout: 5 * time.Second}

// 	resp, err := client.Do(req)
// 	if err != nil {
// 		return "", "", err
// 	}
// 	defer resp.Body.Close()

// 	body, _ := io.ReadAll(resp.Body)
// 	raw := strings.ToLower(string(body))

// 	// best-effort entity detection
// 	switch {
// 	case strings.Contains(raw, "binance"):
// 		return "Binance", "Binance", nil
// 	case strings.Contains(raw, "okx"):
// 		return "OKX", "OKX", nil
// 	case strings.Contains(raw, "kraken"):
// 		return "Kraken", "Kraken", nil
// 	case strings.Contains(raw, "coinbase"):
// 		return "Coinbase", "Coinbase", nil
// 	}

// 	return "", "Unknown", nil
// }

/*
===========================================================
DB HELPERS
===========================================================
*/

// func dbAddressExists(chainID int, address string) bool {
// 	var x int
// 	err := DB.QueryRow(`
// 		SELECT 1
// 		FROM address_labels
// 		WHERE chain_id = ? AND address = ?
// 		LIMIT 1
// 	`, chainID, address).Scan(&x)

// 	return err == nil
// }

// func dbInsertAddressLabel(
// 	chainID int,
// 	address string,
// 	label string,
// 	entity string,
// 	source string,
// ) {
// 	_, _ = DB.Exec(`
// 		INSERT INTO address_labels
// 		    (chain_id, address, label, entity, source)
// 		VALUES (?, ?, ?, ?, ?)
// 		ON DUPLICATE KEY UPDATE
// 		    label = VALUES(label),
// 		    entity = VALUES(entity),
// 		    source = VALUES(source),
// 		    checked_at = NOW()
// 	`, chainID, address, label, entity, source)
// }

/*
===========================================================
UTILS
===========================================================
*/

// func getEtherscanV2Key() string {
// 	return "JPRXBXFXFDHY5UG6US7JEUQ7EEHTNHQDGB"
// }

// func itoa(v int) string {
// 	if v == 0 {
// 		return "0"
// 	}
// 	out := ""
// 	for v > 0 {
// 		out = string('0'+(v%10)) + out
// 		v /= 10
// 	}
// 	return out
// }

/*
===========================================================
КІНЕЦЬ

✔ ETHERSCAN API V2
✔ ОДИН API KEY
✔ chainid для всіх EVM
✔ повний цикл
✔ кеш у БД
✔ без повернення значень
===========================================================
*/
