package main

// SwapMove struct for swap details

// ws_unified.go — ЄДИНИЙ /ws + буфер алертів + TG

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ============= Глобальний список WS-клієнтів (єдиний /ws) =============
type WSClient struct {
	conn       *websocket.Conn
	addrFilter map[string]struct{} // фільтр адрес клієнта (lower 0x..), задається через "update_addresses"
	mu         sync.Mutex          // щоб не змішувались паралельні writes
}

var (
	wsClientsMu sync.RWMutex
	wsClients   = map[*WSClient]struct{}{}
)

func registerWS(c *WSClient) { wsClientsMu.Lock(); wsClients[c] = struct{}{}; wsClientsMu.Unlock() }
func unregisterWS(c *WSClient) {
	wsClientsMu.Lock()
	delete(wsClients, c)
	wsClientsMu.Unlock()
	_ = c.conn.Close()
}

func normHexLower(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return s
	}
	if !strings.HasPrefix(s, "0x") {
		s = "0x" + s
	}
	return s
}

var upgrader = websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}

// =================== Буфер алертів (щоб віддати беклог) ===================
type AlertEvent struct {
	Type     string `json:"type"` // "alert"
	Ts       int64  `json:"ts"`
	Block    uint64 `json:"block"`
	TxHash   string `json:"txHash"`
	From     string `json:"from"`
	To       string `json:"to"`
	ValueEth string `json:"valueEth"`
	GasUsed  uint64 `json:"gasUsed"`
	GasPrice string `json:"gasPrice"`
	Nonce    uint64 `json:"nonce"`

	RuleID  int64  `json:"ruleId"`
	RuleTag string `json:"ruleTag"`

	// 🔥 додаткові поля для токенів / swap
	Token  string `json:"token,omitempty"`
	Amount string `json:"amount,omitempty"`
}

var (
	alertBufMu sync.RWMutex
	alertBuf   []AlertEvent
	// alertBufCap = 1000
)

// alertBufAdd — кладе алерт у кільцевий буфер (викликається при кожному збігу правил)
// func alertBufAdd(evt AlertEvent) {
// 	alertBufMu.Lock()
// 	if len(alertBuf) >= alertBufCap {
// 		alertBuf = alertBuf[1:]
// 	}
// 	alertBuf = append(alertBuf, evt)
// 	alertBufMu.Unlock()
// }

// alertBufSnapshot — повертає зріз з урахуванням фільтра клієнта (from/to ∈ filter)
func alertBufSnapshot(filter map[string]struct{}) []AlertEvent {
	alertBufMu.RLock()
	defer alertBufMu.RUnlock()
	if len(filter) == 0 {
		out := make([]AlertEvent, len(alertBuf))
		copy(out, alertBuf)
		return out
	}
	out := make([]AlertEvent, 0, len(alertBuf))
	for _, e := range alertBuf {
		if _, ok := filter[e.From]; ok {
			out = append(out, e)
			continue
		}
		if _, ok := filter[e.To]; ok {
			out = append(out, e)
			continue
		}
	}
	return out
}

// ========================= ЄДИНИЙ WS-ХЕНДЛЕР =========================
// РЕЄСТР: router.HandleFunc("/ws", WsHandler)
func WsHandler(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		http.Error(w, "upgrade failed", 500)
		return
	}
	// НЕ роби окремий defer conn.Close(); закриємо в unregisterWS
	c := &WSClient{conn: conn, addrFilter: map[string]struct{}{}}
	registerWS(c)
	defer unregisterWS(c)

	log.Println("✅ WS connected")

	// ---- keepalive: дедлайни + ping/pong ----
	const (
		pongWait   = 60 * time.Second
		pingPeriod = 30 * time.Second
		writeWait  = 10 * time.Second
	)
	conn.SetReadLimit(1 << 20)
	conn.SetReadDeadline(time.Now().Add(pongWait))
	conn.SetPongHandler(func(string) error {
		conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})
	go func() { // пінги
		t := time.NewTicker(pingPeriod)
		defer t.Stop()
		for range t.C {
			c.mu.Lock()
			conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				c.mu.Unlock()
				return
			}
			c.mu.Unlock()
		}
	}()

	// ---- ЧИТАЙ ТУТ (без окремої горутини), щоб хендлер не вийшов ----
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			// не шуміти на нормальні закриття
			if websocket.IsCloseError(err, websocket.CloseNormalClosure, websocket.CloseGoingAway, websocket.CloseNoStatusReceived) ||
				errors.Is(err, net.ErrClosed) {
				log.Println("🔴 WS read closed:", err)
			} else {
				log.Println("🛑 WS read error:", err)
			}
			return
		}
		var msg struct {
			Type      string   `json:"type"`
			Addresses []string `json:"addresses"`
		}
		if json.Unmarshal(raw, &msg) == nil && msg.Type == "update_addresses" {
			m := make(map[string]struct{}, len(msg.Addresses))
			for _, a := range msg.Addresses {
				if low := normHexLower(a); low != "" {
					m[low] = struct{}{}
				}
			}
			// під замком, див. п.2
			c.mu.Lock()
			c.addrFilter = m
			c.mu.Unlock()

			items := alertBufSnapshot(m) // backlog
			if len(items) > 0 {
				resp := map[string]any{"type": "alert_batch", "items": items}
				c.mu.Lock()
				_ = c.conn.WriteJSON(resp)
				c.mu.Unlock()
			}
		}
	}
}

type AlertRecord struct {
	// ---- core ----
	RuleID  int64  `json:"ruleId"`
	UserID  int64  `json:"userId"`
	RuleTag string `json:"ruleTag"`

	TxHash string `json:"txHash"`
	Block  uint64 `json:"block"`
	Ts     int64  `json:"timestamp"`

	From string `json:"from"`
	To   string `json:"to"`

	// ---- ETH / gas ----
	ValueEth string `json:"valueEth"` // wei
	GasUsed  uint64 `json:"gasUsed"`
	GasPrice string `json:"gasPrice"` // wei
	GasCost  string `json:"gasCost"`  // wei
	Nonce    uint64 `json:"nonce"`
	Status   uint64 `json:"status"`

	// ---- ERC20 / swap ----
	Token         string `json:"token,omitempty"`
	TokenSymbol   string `json:"tokenSymbol,omitempty"`
	TokenDecimals uint8  `json:"tokenDecimals,omitempty"`
	NativToken    string `json:"nativToken"`

	AmountRaw   string `json:"amountRaw,omitempty"`
	AmountHuman string `json:"amountHuman,omitempty"`

	PriceUSD string `json:"priceUsd,omitempty"`
	ValueUSD string `json:"valueUsd,omitempty"`

	Direction      string `json:"direction,omitempty"`
	TransfersCount int    `json:"transfersCount,omitempty"`

	SwapIns   []SwapMove `json:"swapIns,omitempty"` // ← ДОДАСИ В СТРУКТУРУ
	SwapOuts  []SwapMove `json:"swapOuts,omitempty"`
	SwapRoute []string
}

// =================== РОЗСИЛКА АЛЕРТІВ УСІМ КЛІЄНТАМ + TG ===================
// Викликай там, де ти вже отримуєш matches у своєму споживачі блоків.
// Напр. у consumer після Scan_block:
//
//	_, matches := GetTransactionStatsAndMatchesJSON_Rules(a.Address, CurrentAlertRules())
//	for _, m := range matches { BroadcastAlertWS(a.Number_block, m) }
func BroadcastAlertWS(block uint64, m MatchedTxInfo) {
	// 1) Формуємо подію та кладемо у кільцевий буфер (для беклогу новим клієнтам)
	evt := AlertRecord{
		RuleID:  m.RuleID,
		UserID:  m.UserID,
		RuleTag: m.RuleTag,

		TxHash: m.TxHash,
		Block:  block,
		Ts:     time.Now().Unix(),

		From: m.From,
		To:   m.To,

		ValueEth: m.ValueEth,
		GasUsed:  m.GasUsed,
		GasPrice: m.GasPrice,
		GasCost:  m.GasCost,
		Nonce:    m.Nonce,
		Status:   m.Status,

		Token:         m.Token,
		TokenSymbol:   m.TokenSymbol,
		TokenDecimals: m.TokenDecimals,

		AmountRaw:   m.Amount,
		AmountHuman: m.AmountHuman,

		PriceUSD: m.PriceUSD,
		ValueUSD: m.ValueUSD,

		Direction:      m.Direction,
		TransfersCount: m.TransfersCount,
		NativToken:     m.NativToken,
		SwapIns:        m.SwapIns,
		SwapOuts:       m.SwapOuts,
		SwapRoute:      m.SwapRoute,
	}
	//alertBufAdd(evt)
	// 2) Готуємо повідомлення один раз
	b, err := json.Marshal(evt)
	if err != nil {
		log.Printf("WS marshal error: %v", err)
		return
	}

	b, _ = json.MarshalIndent(m, "", "  ")
	log.Printf("🚨 Alert:\n%s", b)

	detailsJSON, err := json.Marshal(evt)
	if err != nil {
		log.Printf("❌ marshal AlertRecord error: %v", err)
		return
	}

	res, err := DB.Exec(`
		INSERT INTO alerts
			(rule_id, telegram_id, tx_hash, short_message, details, created_at)
		VALUES
			(?,?,?,?,?, NOW())
	`,
		evt.RuleID,
		evt.UserID,
		evt.TxHash,
		fmt.Sprintf("%s → %s", evt.From, evt.To),
		detailsJSON,
	)

	if err != nil {
		log.Printf("❌ DB insert alert error: %v", err)
		return
	}

	rows, _ := res.RowsAffected()
	log.Printf("✅ alert inserted, rows=%d", rows)
	// 3) Розсилка клієнтам з урахуванням їхнього фільтра
	var toDrop []*WSClient // кого треба відписати після RUnlock
	wsClientsMu.RLock()
	for c := range wsClients {
		// --- безпечно читаємо поточний фільтр клієнта ---
		c.mu.Lock()
		filter := c.addrFilter // зчитуємо посилання на мапу (мапа не мутується, лише підміняється в іншому місці)
		c.mu.Unlock()

		// вирішуємо, чи слати подію цьому клієнту
		send := len(filter) == 0
		if !send && filter != nil {
			if _, ok := filter[evt.From]; ok {
				send = true
			} else if _, ok := filter[evt.To]; ok {
				send = true
			}
		}
		if !send {
			continue
		}

		// --- відправляємо (write під локом клієнта) ---
		c.mu.Lock()
		if err := c.conn.WriteMessage(websocket.TextMessage, b); err != nil {
			// Відкладено відпишемо клієнта після того, як відпустимо wsClientsMu.RLock()
			toDrop = append(toDrop, c)
		}
		c.mu.Unlock()
	}
	wsClientsMu.RUnlock()

	// 4) Відписуємо «померлі» конекшени поза RLock
	for _, c := range toDrop {
		unregisterWS(c)
	}

	// 5) TG — завжди, незалежно від WS
	sendTelegramAlert(m)
}

func sendTelegramAlert(m MatchedTxInfo) {
	token := os.Getenv("TG_BOT_TOKEN")
	if token == "" {
		return
	}

	chatID := strconv.FormatInt(m.UserID, 10)

	// ---------- TITLE ----------
	var title string
	switch m.RuleTag {
	case "swap":
		title = "🔄 Swap detected"

	case "SwapFinance":
		title = "💱 Value swap detected"

	case "eth_transfer":
		title = "💸 Native transfer"

	case "erc20_transfer":
		title = "🪙 Token transfer"

	default:
		title = "🚨 Alert triggered"
	}

	var b strings.Builder

	// ---------- HEADER ----------
	b.WriteString(fmt.Sprintf("<b>%s</b>\n", title))

	if m.RuleName != "" {
		b.WriteString(fmt.Sprintf(
			"<b>Rule:</b> %s\n",
			html.EscapeString(m.RuleName),
		))
	}

	// ---------- ADDRESSES ----------
	if m.From != "" {
		b.WriteString(fmt.Sprintf(
			"<b>From:</b> <code>%s</code>\n",
			m.From,
		))
	}

	if m.To != "" {
		b.WriteString(fmt.Sprintf(
			"<b>To:</b> <code>%s</code>\n",
			m.To,
		))
	}

	// ---------- SWAP FINANCE DETAILS ----------
	if m.RuleTag == "SwapFinance" {

		b.WriteString(fmt.Sprintf(
			"<b>Network:</b> %s\n",
			m.NativToken,
		))

		if len(m.SwapOuts) > 0 {
			b.WriteString("\n<b>OUT:</b>\n")
			for _, o := range m.SwapOuts {
				line := o.TokenSymbol
				if line == "" {
					line = o.Token
				}
				b.WriteString(fmt.Sprintf(
					"• %s %.6f ($%.2f)\n",
					line,
					o.Amount,
					o.USD,
				))
			}
		}

		if len(m.SwapIns) > 0 {
			b.WriteString("\n<b>IN:</b>\n")
			for _, i := range m.SwapIns {
				line := i.TokenSymbol
				if line == "" {
					line = i.Token
				}
				b.WriteString(fmt.Sprintf(
					"• %s %.6f ($%.2f)\n",
					line,
					i.Amount,
					i.USD,
				))
			}
		}

		if m.ValueUSD != "" {
			b.WriteString(fmt.Sprintf(
				"\n<b>Total value:</b> $%s\n",
				m.ValueUSD,
			))
		}
	}

	// ---------- SWAP TOKEN INFO (classic swap) ----------
	if m.RuleTag == "swap" && m.Token != "" {

		tokenLine := m.Token
		if m.TokenSymbol != "" {
			tokenLine = fmt.Sprintf(
				"%s (<code>%s</code>)",
				html.EscapeString(m.TokenSymbol),
				m.Token,
			)
		}

		b.WriteString(fmt.Sprintf(
			"<b>Token:</b> %s\n",
			tokenLine,
		))

		if m.AmountHuman != "" {
			b.WriteString(fmt.Sprintf(
				"<b>Amount:</b> %s\n",
				m.AmountHuman,
			))
		}

		if m.ValueUSD != "" {
			b.WriteString(fmt.Sprintf(
				"<b>Value:</b> $%s\n",
				m.ValueUSD,
			))
		}

		if m.Direction != "" {
			b.WriteString(fmt.Sprintf(
				"<b>Direction:</b> %s\n",
				m.Direction,
			))
		}
	}

	// ---------- NATIVE VALUE ----------
	if m.ValueEth != "" && m.ValueEth != "0" {
		b.WriteString(fmt.Sprintf(
			"<b>Native value:</b> %s wei\n",
			m.ValueEth,
		))
	}

	// ---------- TX LINK ----------
	if m.TxHash != "" {
		var txURL string

		switch m.NativToken {
		case "ETH":
			txURL = "https://etherscan.io/tx/" + m.TxHash

		case "BNB":
			txURL = "https://bscscan.com/tx/" + m.TxHash

		case "POL":
			txURL = "https://polygonscan.com/tx/" + m.TxHash

		case "SOL":
			txURL = "https://solscan.io/tx/" + m.TxHash

		default:
			txURL = m.TxHash // fallback без explorer
		}

		b.WriteString(fmt.Sprintf(
			"\n<a href=\"%s\">🔗 View transaction</a>",
			txURL,
		))
	}
	text := b.String()

	// ---------- SEND ----------
	_, err := http.PostForm(
		"https://api.telegram.org/bot"+token+"/sendMessage",
		url.Values{
			"chat_id":                  {chatID},
			"text":                     {text},
			"parse_mode":               {"HTML"},
			"disable_web_page_preview": {"true"},
		},
	)

	if err != nil {
		log.Printf("❌ Telegram send error: %v", err)
	}
}
