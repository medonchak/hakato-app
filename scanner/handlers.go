package main

import (
	"encoding/json"
	"net/http"

	"github.com/ethereum/go-ethereum/common"
)

// === Типи даних ===
type Metric struct {
	Label           string   `json:"label"`
	Value           string   `json:"value"`
	AdressAlertInfo []TxJSON `json:"adressalertinfo"`
}
type ChartData struct {
	Title string        `json:"title"`
	Data  []interface{} `json:"data"`
}
type AddressEntry struct {
	Address string `json:"address"`
	TxCount int    `json:"txCount"`
}
type Transaction struct {
	Hash  string `json:"hash"`
	From  string `json:"from"`
	Value string `json:"value"`
	Gas   string `json:"gas"`
}
type AlertAddress struct {
	Address string `json:"address"`
	Trigger string `json:"trigger"`
	Value   string `json:"value"`
}
type UpdateAddresses struct {
	Type      string         `json:"type"`
	Addresses []AlertAddress `json:"addresses"`
}

func AddressAnalyticsHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Address string `json:"address"`
	}

	// 1. Зчитати адресу з POST body
	err := json.NewDecoder(r.Body).Decode(&req)
	if err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	// 2. Викликати твою функцію аналітики
	result := All_gas_Eth_EtherScan(common.HexToAddress(req.Address))

	// 3. Відповісти клієнту JSON-результатом
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// === WebSocket-сервер ===
// var upgrader = websocket.Upgrader{
// 	CheckOrigin: func(r *http.Request) bool {
// 		return true // дозволити всі джерела (тільки для dev)
// 	},
// }

// func WsHandler(w http.ResponseWriter, r *http.Request) {
// 	var alertAddresses []AlertAddress
// 	conn, err := upgrader.Upgrade(w, r, nil)
// 	if err != nil {
// 		http.Error(w, "WebSocket підключення не вдалося", http.StatusInternalServerError)
// 		return
// 	}
// 	defer conn.Close()

// 	log.Println("✅ WebSocket клієнт підключений")

// 	client := ClientOpen()

// 	// Читання у окремій горутині — слухає disconnect
// 	go func() {
// 		for {
// 			_, msg, err := conn.ReadMessage()
// 			if err != nil {
// 				log.Println("🔴 Клієнт закрив з'єднання:", err)
// 				conn.Close() // Закриваємо сокет
// 				return
// 			}
// 			var incoming UpdateAddresses
// 			if err := json.Unmarshal(msg, &incoming); err != nil {
// 				log.Println("❌ Помилка парсингу JSON:", err)
// 				continue
// 			}

// 			if incoming.Type == "update_addresses" {
// 				alertAddresses = incoming.Addresses
// 				log.Println("📬 Отримано адреси:", alertAddresses)
// 			}
// 		}
// 	}()

// 	for {
// 		stats := Scan_block(client)
// 		TotalGas := WeiToEth(stats.GasUsed)
// 		myactiveadress := FilterTransactionsByAddresses(stats.Address, alertAddresses)
// 		mock := []Metric{
// 			{"Tx Count", strconv.Itoa(stats.SummaryTx), nil},
// 			{"Gas Used", TotalGas, nil},
// 			{"AddressesMas", strconv.Itoa(len(myactiveadress)), myactiveadress},
// 		}

// 		err := conn.WriteJSON(mock)
// 		if err != nil {
// 			log.Println("❌ Відправка через WebSocket не вдалася:", err)
// 			break
// 		}

// 		time.Sleep(12 * time.Second)
// 	}
// }
