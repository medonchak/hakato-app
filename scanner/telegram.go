package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

/* ===================== TYPES ===================== */

type Update struct {
	UpdateID int      `json:"update_id"`
	Message  *Message `json:"message"`
}

type Message struct {
	Text string `json:"text"`
	Chat Chat   `json:"chat"`
}

type Chat struct {
	ID int64 `json:"id"`
}

/* ===================== STATE ===================== */

var offset int

/* ===================== PUBLIC ===================== */

// Викликається з main
func StartPolling() {
	token := os.Getenv("TG_BOT_TOKEN")
	if token == "" {
		log.Println("⚠️  TG_BOT_TOKEN not set — Telegram bot disabled")
		return
	}

	for {
		updates, err := getUpdates(token)
		if err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		for _, upd := range updates {
			offset = upd.UpdateID + 1

			if upd.Message == nil {
				continue
			}

			handleMessage(upd.Message)
		}
	}
}

/* ===================== INTERNAL ===================== */
func getUpdates(token string) ([]Update, error) {
	url := "https://api.telegram.org/bot" + token +
		"/getUpdates?timeout=30&offset=" + strconv.Itoa(offset)

	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var res struct {
		Result []Update `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, err
	}

	return res.Result, nil
}

func handleMessage(m *Message) {

	log.Println("➡️ handleMessage:", m.Text)

	text := strings.TrimSpace(m.Text)
	if !strings.HasPrefix(text, "/") {
		return
	}

	parts := strings.Fields(text)
	cmd := parts[0]

	switch cmd {
	case "/start":
		send(m.Chat.ID, fmt.Sprintf("👋 Бот запущений\n\nВаш ID: %d\n\nКоманди:\n/agent_status — позиція агента\n/best_strategy <TOKEN> — оптимальна стратегія", m.Chat.ID))

	case "/status":
		send(m.Chat.ID, "✅ OK")

	case "/agent_status":
		sendAgentStatus(m.Chat.ID)

	case "/best_strategy":
		tokenArg := "mETH"
		if len(parts) > 1 {
			tokenArg = parts[1]
		}
		sendBestStrategy(m.Chat.ID, tokenArg)

	default:
		send(m.Chat.ID, "❓ Невідома команда. Спробуй /agent_status або /best_strategy mETH")
	}
}

func sendAgentStatus(chatID int64) {
	rows, err := DB.Query(`SELECT token_symbol, size_usd, pnl_usd FROM agent_positions WHERE chain_id=5000`)
	if err != nil {
		send(chatID, "❌ Помилка читання позицій")
		return
	}
	defer rows.Close()

	msg := "📊 Позиції агента (Mantle):\n"
	found := false
	for rows.Next() {
		var sym string
		var sizeUSD, pnlUSD float64
		if rows.Scan(&sym, &sizeUSD, &pnlUSD) != nil {
			continue
		}
		found = true
		sign := "+"
		if pnlUSD < 0 {
			sign = ""
		}
		msg += fmt.Sprintf("• %s: $%.2f | PnL: %s$%.2f\n", sym, sizeUSD, sign, pnlUSD)
	}
	if !found {
		msg += "Активних позицій немає"
	}
	send(chatID, msg)
}

func sendBestStrategy(chatID int64, tokenSymbol string) {
	tokenAddr := strings.ToLower(tokenSymbol)
	switch tokenAddr {
	case "meth":
		tokenAddr = "0xcda86a272531e8640cd7f1a92c01839911b90bb0"
	case "mnt":
		tokenAddr = "native"
	case "usdy":
		tokenAddr = "0x5be26527e817998a7206475496fde1e68957c5a6"
	}

	strat, err := DB_LoadBestStrategy(5000, tokenAddr)
	if err != nil || strat == nil {
		send(chatID, fmt.Sprintf("❌ Стратегія для %s ще не розрахована", tokenSymbol))
		return
	}

	msg := fmt.Sprintf(
		"📈 Оптимальна стратегія для %s:\n"+
			"• VWAP period: %dh\n"+
			"• Купити при: %.1f%% нижче VWAP\n"+
			"• Продати при: +%.1f%% вище VWAP\n"+
			"• Cooldown: %dh\n"+
			"• Sharpe ratio: %.2f\n"+
			"• Win rate: %.0f%%\n"+
			"• Угод у backtест: %d",
		strings.ToUpper(tokenSymbol),
		strat.VWAPPeriod,
		-strat.BuyThresholdPct,
		strat.SellThresholdPct,
		strat.CooldownHours,
		strat.Sharpe,
		strat.WinRate*100,
		strat.TotalTrades,
	)
	send(chatID, msg)
}

func send(chatID int64, text string) {
	if err := sendTelegramMessage(chatID, text); err != nil {
		log.Printf("[TG][ERR] send chat=%d err=%v", chatID, err)
	}
}

// BroadcastTelegramMessage sends a message to all registered users.
// Falls back to a hardcoded admin chat if no users table exists.
func BroadcastTelegramMessage(text string) {
	rows, err := DB.Query(`SELECT telegram_id FROM users WHERE telegram_id IS NOT NULL LIMIT 100`)
	if err != nil {
		// table might not exist yet — try env fallback
		if adminID := adminChatID(); adminID != 0 {
			send(adminID, text)
		}
		return
	}
	defer rows.Close()

	sent := 0
	for rows.Next() {
		var tid int64
		if rows.Scan(&tid) == nil && tid != 0 {
			send(tid, text)
			sent++
		}
	}
	if sent == 0 {
		if adminID := adminChatID(); adminID != 0 {
			send(adminID, text)
		}
	}
}

func adminChatID() int64 {
	var id int64
	fmt.Sscanf(os.Getenv("TG_ADMIN_CHAT_ID"), "%d", &id)
	return id
}

func sendTelegramMessage(chatID int64, text string) error {
	token := os.Getenv("TG_BOT_TOKEN")
	if token == "" {
		return errors.New("TG_BOT_TOKEN not set")
	}

	resp, err := http.PostForm(
		"https://api.telegram.org/bot"+token+"/sendMessage",
		map[string][]string{
			"chat_id": {strconv.FormatInt(chatID, 10)},
			"text":    {text},
		},
	)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("telegram send status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}
