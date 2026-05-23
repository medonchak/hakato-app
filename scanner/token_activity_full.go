// token_activity_full.go
//
// package main — повна реалізація “після CheckTxHashForKnownAddresses”:
//
// ✅ ЩО ЗМІНЕНО / ДОДАНО (без перейменувань існуючих функцій):
// 1) Аналіз активності робиться для ВСІХ tracked-токенів у tx (не тільки exchange-related).
//    Whitelist (ctx.Matches) тепер = додатковий контекст (ExchangeAddr/Name/Direction), але не gate.
// 2) Додано TokenProfile (читання з tokens_metadata) і USD/percent-метрики для правил.
// 3) EvaluateAlertRule тепер реально підтримує MinUSD/SpikeMult/DominancePct і в USD (якщо є price+decimals).
// 4) Додано daily 24h report як notification, якщо користувач увімкнув (daily_report_enabled) у portfolio_token_alert_settings.
//    Відправка 1 раз на добу на token+portfolio (через UNIQUE у БД).
//
// ❗ ВАЖЛИВО:
// - RPC тут НЕ використовується.
// - TokenProfile береться з БД.
// - Активність (hourly/daily) формується тільки з transfers.

package main

import (
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"sync"
	"time"

	"github.com/ethereum/go-ethereum/common"
)

/*
========================================================
ВХІДНІ ТИПИ (В ТЕБЕ ВЖЕ Є) — НЕ ПЕРЕЙМЕНОВУЮ
========================================================

Очікується що в проекті вже є:

type MatchDetail struct {
    Address string
    Name    string
    // ... інше
}

type TxAnalysisContext struct {
    ChainID int64
    TxHash common.Hash
    BlockTime time.Time
    Matches []MatchDetail
    TokenTransfers []TokenTransfer
}

type TokenTransfer struct {
    Token common.Address
    From common.Address
    To common.Address
    Amount *big.Int
}

Також глобальний DB *sql.DB існує у твоєму пакеті.
*/

/*
========================================================
ПОДІЇ ТА АНАЛІТИКА
========================================================
*/

type ExchangeDirection string

const (
	DirInToken   ExchangeDirection = "IN"   // на біржу (to = exchange)
	DirOutToken  ExchangeDirection = "OUT"  // з біржі (from = exchange)
	DirMixToken  ExchangeDirection = "MIX"  // якщо і from і to матчаться
	DirNoneToken ExchangeDirection = "NONE" // не exchange-related (але tracked токен)
)

type TokenOnchainEvent struct {
	ChainID   int64
	TxHash    common.Hash
	BlockTime time.Time
	HourTS    int64
	DayTS     int64

	Token  common.Address
	From   common.Address
	To     common.Address
	Amount *big.Int

	// Whitelist/exchange контекст (може бути порожній)
	ExchangeAddr string
	ExchangeName string
	Direction    ExchangeDirection
}

type TokenEventMetrics struct {
	AmountRaw *big.Int

	AmountUSD sql.NullFloat64 // якщо є price+decimals

	PctOfSupply sql.NullFloat64 // amount_raw / max_total_supply_raw
	PctOfMcap   sql.NullFloat64 // amount_usd / circulating_market_cap

	IsExchange bool
}

type TokenProfile struct {
	ChainID    int64
	TokenLower string

	PriceUSD sql.NullFloat64

	CirculatingMarketCap sql.NullFloat64
	OnchainMarketCap     sql.NullFloat64

	// ВАЖЛИВО: max_total_supply_raw в БД має бути в RAW (тобто як uint256, без decimals)
	// Якщо ти зберігаєш max_total_supply в human units — тоді %Supply буде неточним.
	MaxTotalSupplyRaw sql.NullString

	Decimals sql.NullInt64 // decimals потрібні тільки для USD-перерахунку

	Holders sql.NullInt64

	Top10Pct  sql.NullFloat64
	Top50Pct  sql.NullFloat64
	Top100Pct sql.NullFloat64
}

type HourlyTokenActivity struct {
	ChainID int64
	Token   common.Address
	HourTS  int64

	TransferCount int64

	TotalVolumeRaw *big.Int
	ExchangeInRaw  *big.Int
	ExchangeOutRaw *big.Int
	MaxTransferRaw *big.Int

	// опційно (якщо є price+decimals)
	TotalVolumeUSD sql.NullFloat64
	ExchangeInUSD  sql.NullFloat64
	ExchangeOutUSD sql.NullFloat64
	MaxTransferUSD sql.NullFloat64

	// контекст
	ExchangeTransferCount int64
}

func newHourly(chainID int64, token common.Address, hourTS int64) *HourlyTokenActivity {
	return &HourlyTokenActivity{
		ChainID:               chainID,
		Token:                 token,
		HourTS:                hourTS,
		TransferCount:         0,
		TotalVolumeRaw:        big.NewInt(0),
		ExchangeInRaw:         big.NewInt(0),
		ExchangeOutRaw:        big.NewInt(0),
		MaxTransferRaw:        big.NewInt(0),
		TotalVolumeUSD:        sql.NullFloat64{Valid: false},
		ExchangeInUSD:         sql.NullFloat64{Valid: false},
		ExchangeOutUSD:        sql.NullFloat64{Valid: false},
		MaxTransferUSD:        sql.NullFloat64{Valid: false},
		ExchangeTransferCount: 0,
	}
}

/*
========================================================
АЛЕРТ-ФІЛЬТРИ
========================================================
*/

type AlertRuleForToken struct {
	MinUSD float64
	MinRaw string // decimal string (RAW)

	// "", "IN", "OUT", "MIX", "NONE"
	Direction string

	SpikeMult    float64
	DominancePct float64
}

/*
========================================================
ПІДПИСКИ ПОРТФЕЛІВ
========================================================
*/

type PortfolioTokenSubscription struct {
	PortfolioID int64
	UserID      int64
	TokenLower  string
	AssetKey    string
	AssetSymbol string

	Rule AlertRuleForToken

	// NEW: 24h report
	DailyReportEnabled bool
}

type TokenFilterContext struct {
	// Профіль (read-only з БД)
	Profile *TokenProfile

	// totals for this hour
	HourTotalRaw *big.Int
	HourAvgRaw   *big.Int // avg raw over last 24h

	// totals for this hour in USD (якщо можна порахувати)
	HourTotalUSD sql.NullFloat64
	HourAvgUSD   sql.NullFloat64
}

type trackedTokensCacheEntry struct {
	tokens    map[string]bool
	expiresAt time.Time
}

var trackedTokensCache = struct {
	mu      sync.RWMutex
	byChain map[int64]trackedTokensCacheEntry
}{
	byChain: make(map[int64]trackedTokensCacheEntry),
}

const trackedTokensCacheTTL = 30 * time.Second

const (
	defaultAlertMinUSD       = 25000.0
	defaultAlertSpikeMult    = 3.0
	defaultAlertDominancePct = 0.15
	criticalAlertUSD         = 150000.0
	alertCooldownWindow      = 15 * time.Minute
	dailyReportMinTransfers  = 20
	dailyReportMinUSD        = 200000.0
)

var portfolioAlertGate = struct {
	mu         sync.Mutex
	lastByTx   map[string]time.Time
	lastBySlot map[string]time.Time
}{
	lastByTx:   make(map[string]time.Time),
	lastBySlot: make(map[string]time.Time),
}

/*
========================================================
ГОЛОВНА ФУНКЦІЯ (НЕ ПЕРЕЙМЕНОВУЮ)
========================================================
*/

func AnalyzeTokenActivityFromTx(ctx *TxAnalysisContext) error {
	if ctx == nil {
		return errors.New("ctx is nil")
	}
	if len(ctx.TokenTransfers) == 0 {
		return nil
	}

	// whitelist map (lower)
	exWL := make(map[string]MatchDetail, len(ctx.Matches))
	for _, m := range ctx.Matches {
		exWL[strings.ToLower(m.Address)] = m
	}

	// tracked tokens
	tracked, err := DB_LoadTrackedTokensCached(ctx.ChainID)
	if err != nil {
		return err
	}
	if len(tracked) == 0 {
		return nil
	}

	hourTS := ctx.BlockTime.UTC().Truncate(time.Hour).Unix()
	dayTS := ctx.BlockTime.UTC().Truncate(24 * time.Hour).Unix()

	// cache профілів per tokenLower
	profiles := make(map[string]*TokenProfile)

	// events (ВАЖЛИВО: беремо ВСІ transfers tracked-токена)
	events := make([]TokenOnchainEvent, 0, len(ctx.TokenTransfers))

	for _, tr := range ctx.TokenTransfers {
		tokenLower := strings.ToLower(tr.Token.Hex())
		if !tracked[tokenLower] {
			continue
		}

		fromL := strings.ToLower(tr.From.Hex())
		toL := strings.ToLower(tr.To.Hex())

		var dir ExchangeDirection = DirNoneToken
		var exAddr, exName string

		_, fromIs := exWL[fromL]
		_, toIs := exWL[toL]

		switch {
		case fromIs && toIs:
			dir = DirMixToken
			exAddr = fromL
			exName = exWL[fromL].Name
		case fromIs:
			dir = DirOutToken
			exAddr = fromL
			exName = exWL[fromL].Name
		case toIs:
			dir = DirInToken
			exAddr = toL
			exName = exWL[toL].Name
		default:
			// tracked transfer, але не whitelist — лишаємо як NONE
		}

		amount := tr.Amount
		if amount == nil {
			amount = big.NewInt(0)
		}

		events = append(events, TokenOnchainEvent{
			ChainID:      ctx.ChainID,
			TxHash:       ctx.TxHash,
			BlockTime:    ctx.BlockTime.UTC(),
			HourTS:       hourTS,
			DayTS:        dayTS,
			Token:        tr.Token,
			From:         tr.From,
			To:           tr.To,
			Amount:       new(big.Int).Set(amount),
			ExchangeAddr: exAddr,
			ExchangeName: exName,
			Direction:    dir,
		})

		// підтягуємо профіль в кеш (щоб не робити N запитів нижче)
		if _, ok := profiles[tokenLower]; !ok {
			p, perr := DB_LoadTokenProfile(ctx.ChainID, tokenLower)
			if perr == nil {
				profiles[tokenLower] = p
			} else {
				// профілю може не бути — тоді USD/% будуть недоступні, але RAW-аналіз працює
				profiles[tokenLower] = nil
			}
		}
	}

	if len(events) > 0 {
		tokenEventChan <- events
	}

	// 3) HOURLY агрегація
	agg := make(map[string]*HourlyTokenActivity) // key: tokenLower|chain|hour
	for _, ev := range events {
		tokenLower := strings.ToLower(ev.Token.Hex())
		key := fmt.Sprintf("%s|%d|%d", tokenLower, ev.ChainID, ev.HourTS)

		h, ok := agg[key]
		if !ok {
			h = newHourly(ev.ChainID, ev.Token, ev.HourTS)
			agg[key] = h
		}

		h.TransferCount++
		h.TotalVolumeRaw.Add(h.TotalVolumeRaw, ev.Amount)

		if ev.Amount.Cmp(h.MaxTransferRaw) > 0 {
			h.MaxTransferRaw.Set(ev.Amount)
		}

		if ev.ExchangeAddr != "" {
			h.ExchangeTransferCount++
		}

		switch ev.Direction {
		case DirInToken:
			h.ExchangeInRaw.Add(h.ExchangeInRaw, ev.Amount)
		case DirOutToken:
			h.ExchangeOutRaw.Add(h.ExchangeOutRaw, ev.Amount)
		case DirMixToken:
			h.ExchangeInRaw.Add(h.ExchangeInRaw, ev.Amount)
			h.ExchangeOutRaw.Add(h.ExchangeOutRaw, ev.Amount)
		default:
			// NONE: тільки в TotalVolumeRaw
		}

		// USD агрегати, якщо можемо
		p := profiles[tokenLower]
		if p != nil {
			m := BuildTokenEventMetrics(ev, p)
			if m.AmountUSD.Valid {
				// total
				h.TotalVolumeUSD = addNullFloat(h.TotalVolumeUSD, m.AmountUSD)

				// exchange totals
				if ev.Direction == DirInToken || ev.Direction == DirMixToken {
					h.ExchangeInUSD = addNullFloat(h.ExchangeInUSD, m.AmountUSD)
				}
				if ev.Direction == DirOutToken || ev.Direction == DirMixToken {
					h.ExchangeOutUSD = addNullFloat(h.ExchangeOutUSD, m.AmountUSD)
				}

				// max usd
				if !h.MaxTransferUSD.Valid || m.AmountUSD.Float64 > h.MaxTransferUSD.Float64 {
					h.MaxTransferUSD = m.AmountUSD
				}
			}
		}
	}
	// 4) upsert hourly (розширена версія)
	if err := DB_UpsertTokenHourlyActivity(agg); err != nil {
		return err
	}

	// 5) portfolio alerts + daily report
	subsCache := make(map[string][]PortfolioTokenSubscription)
	subsLoaded := make(map[string]bool)
	refCache := make(map[string]TokenFilterContext)
	refLoaded := make(map[string]bool)
	ruleCache := make(map[string]*LoadedAnomalyRule)
	ruleLoaded := make(map[string]bool)

	for _, ev := range events {
		tokenLower := strings.ToLower(ev.Token.Hex())

		if !subsLoaded[tokenLower] {
			subs, err := DB_LoadPortfolioTokenSubscriptions(ev.ChainID, tokenLower)
			if err != nil {
				return err
			}
			subsCache[tokenLower] = subs
			subsLoaded[tokenLower] = true
		}
		subs := subsCache[tokenLower]
		if len(subs) == 0 {
			continue
		}

		if !refLoaded[tokenLower] {
			ref, err := DB_LoadTokenFilterContext(ev.ChainID, tokenLower, ev.HourTS)
			if err != nil {
				return err
			}
			ref.Profile = profiles[tokenLower]
			refCache[tokenLower] = ref
			refLoaded[tokenLower] = true
		}
		ref := refCache[tokenLower]

		if !ruleLoaded[tokenLower] {
			rule, rerr := DB_LoadTokenAnomalyRule(ev.ChainID, tokenLower)
			if rerr == nil && rule != nil {
				ruleCache[tokenLower] = rule
			}
			ruleLoaded[tokenLower] = true
		}
		rule := ruleCache[tokenLower]
		if rule != nil {
			var hm *TokenHourlyMetrics = nil
			hm = DB_TryLoadHourlyMetrics(ev.ChainID, tokenLower, ev.HourTS)

			res := EvaluateSystemAnomaly(ev, ref, hm, rule)
			if res.Matched {
				_ = DB_InsertTokenAnomalyEvent(ev, ref, res)

				txt := BuildSystemAnomalyText(ev, res)
				for _, sub := range subs {
					SendTelegramAlert(sub.UserID, txt)
				}
			}
		}

		for _, sub := range subs {
			if shouldNotifyPortfolioEvent(sub, ev, ref) {
				nid, ierr := DB_InsertPortfolioNotification(sub.PortfolioID, sub, ev, sub.Rule, ref)
				if ierr != nil {
					return ierr
				}
				_ = nid
				SendTelegramAlert(sub.UserID, BuildAlertText(sub, ev, ref))
			}
		}

		if shouldSendDailyReportNow(ev.BlockTime) {
			for _, sub := range subs {
				if !sub.DailyReportEnabled {
					continue
				}

				rep, rerr := DB_LoadToken24hReportForPortfolio(sub.PortfolioID, ev.ChainID, tokenLower, ev.BlockTime)
				if rerr != nil {
					return rerr
				}

				if !shouldSendDailyPortfolioReport(rep) {
					continue
				}

				rid, ierr := DB_InsertPortfolioDailyReportNotification(sub.PortfolioID, ev.ChainID, tokenLower, ev.DayTS, rep)
				if ierr != nil {
					return ierr
				}
				_ = rid

				SendTelegramAlert(sub.UserID, rep.Text)
			}
		}
	}

	return nil
}

/*
========================================================
METRICS
========================================================
*/

func BuildTokenEventMetrics(ev TokenOnchainEvent, profile *TokenProfile) TokenEventMetrics {
	m := TokenEventMetrics{
		AmountRaw:  ev.Amount,
		IsExchange: ev.ExchangeAddr != "",
	}

	// amount_usd = (amount_raw / 10^decimals) * price_usd
	if profile.PriceUSD.Valid && profile.Decimals.Valid && profile.Decimals.Int64 >= 0 {
		usd, ok := rawToUSD(ev.Amount, profile.Decimals.Int64, profile.PriceUSD.Float64)
		if ok {
			m.AmountUSD = sql.NullFloat64{Float64: usd, Valid: true}
		}
	}

	// % of supply (RAW / MaxTotalSupplyRaw)
	if profile.MaxTotalSupplyRaw.Valid {
		supRaw, ok := new(big.Int).SetString(strings.TrimSpace(profile.MaxTotalSupplyRaw.String), 10)
		if ok && supRaw.Sign() > 0 {
			pct := new(big.Rat).SetFrac(ev.Amount, supRaw)
			f, _ := pct.Float64()
			m.PctOfSupply = sql.NullFloat64{Float64: f, Valid: true}
		}
	}

	// % of market cap (USD / circulating_market_cap)
	if m.AmountUSD.Valid && profile.CirculatingMarketCap.Valid && profile.CirculatingMarketCap.Float64 > 0 {
		m.PctOfMcap = sql.NullFloat64{
			Float64: m.AmountUSD.Float64 / profile.CirculatingMarketCap.Float64,
			Valid:   true,
		}
	}

	return m
}

func rawToUSD(amountRaw *big.Int, decimals int64, priceUSD float64) (float64, bool) {
	if amountRaw == nil {
		return 0, false
	}
	if decimals < 0 {
		return 0, false
	}
	// amount_human = amountRaw / 10^decimals
	den := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil)
	if den.Sign() == 0 {
		return 0, false
	}

	amtRat := new(big.Rat).SetInt(amountRaw)
	denRat := new(big.Rat).SetInt(den)
	human := new(big.Rat).Quo(amtRat, denRat)

	// human * priceUSD
	priceRat := new(big.Rat).SetFloat64(priceUSD)
	usdRat := new(big.Rat).Mul(human, priceRat)

	f, ok := usdRat.Float64()
	if !ok {
		// навіть якщо ok=false, f може бути значенням — але нехай буде safe
		return f, true
	}
	return f, true
}

func addNullFloat(a sql.NullFloat64, b sql.NullFloat64) sql.NullFloat64 {
	if !b.Valid {
		return a
	}
	if !a.Valid {
		return b
	}
	return sql.NullFloat64{Float64: a.Float64 + b.Float64, Valid: true}
}

func shouldSendDailyReportNow(t time.Time) bool {
	utc := t.UTC()
	// перші 10 хвилин після 00:00 UTC
	return utc.Hour() == 0 && utc.Minute() < 10
}

/*
========================================================
RULE EVAL (реально працює і для USD, якщо є)
========================================================
*/

func EvaluateAlertRule(ev TokenOnchainEvent, ref TokenFilterContext, rule AlertRuleForToken) bool {
	// 1) direction filter
	if rule.Direction != "" {
		if string(ev.Direction) != rule.Direction {
			return false
		}
	}

	// 2) min raw
	if rule.MinRaw != "" {
		minRaw, ok := new(big.Int).SetString(rule.MinRaw, 10)
		if ok && ev.Amount.Cmp(minRaw) < 0 {
			return false
		}
	}

	// 3) min usd (якщо можемо порахувати amountUSD)
	if rule.MinUSD > 0 {
		amountUSD, ok := eventAmountUSD(ev, ref.Profile)
		if ok {
			if amountUSD < rule.MinUSD {
				return false
			}
		}
		// якщо не можемо порахувати — НЕ блокуємо (як ти й хотів: USD опційно)
	}

	// 4) spike (USD якщо є, інакше RAW)
	if rule.SpikeMult > 0 {
		amountUSD, okUSD := eventAmountUSD(ev, ref.Profile)
		if okUSD && ref.HourAvgUSD.Valid && ref.HourAvgUSD.Float64 > 0 {
			if amountUSD <= ref.HourAvgUSD.Float64*rule.SpikeMult {
				return false
			}
		} else {
			// RAW spike
			if ref.HourAvgRaw != nil && ref.HourAvgRaw.Sign() > 0 {
				avgMul := new(big.Rat).SetInt(ref.HourAvgRaw)
				avgMul.Mul(avgMul, new(big.Rat).SetFloat64(rule.SpikeMult))
				thr := new(big.Int).Quo(avgMul.Num(), avgMul.Denom())
				if ev.Amount.Cmp(thr) <= 0 {
					return false
				}
			}
		}
	}

	// 5) dominance (USD якщо є, інакше RAW)
	if rule.DominancePct > 0 && rule.DominancePct <= 1 {
		amountUSD, okUSD := eventAmountUSD(ev, ref.Profile)
		if okUSD && ref.HourTotalUSD.Valid && ref.HourTotalUSD.Float64 > 0 {
			if amountUSD <= ref.HourTotalUSD.Float64*rule.DominancePct {
				return false
			}
		} else {
			if ref.HourTotalRaw != nil && ref.HourTotalRaw.Sign() > 0 {
				totalRat := new(big.Rat).SetInt(ref.HourTotalRaw)
				totalRat.Mul(totalRat, new(big.Rat).SetFloat64(rule.DominancePct))
				thr := new(big.Int).Quo(totalRat.Num(), totalRat.Denom())
				if ev.Amount.Cmp(thr) <= 0 {
					return false
				}
			}
		}
	}

	return true
}

func eventAmountUSD(ev TokenOnchainEvent, profile *TokenProfile) (float64, bool) {
	if profile == nil {
		return 0, false
	}
	if !profile.PriceUSD.Valid || !profile.Decimals.Valid {
		return 0, false
	}
	return rawToUSD(ev.Amount, profile.Decimals.Int64, profile.PriceUSD.Float64)
}

/*
========================================================
HASH RULE
========================================================
*/

func hashRule(r AlertRuleForToken) string {
	s := fmt.Sprintf("minusd=%.8f|minraw=%s|dir=%s|spike=%.4f|dom=%.4f",
		r.MinUSD, r.MinRaw, r.Direction, r.SpikeMult, r.DominancePct,
	)
	return hex.EncodeToString([]byte(s))
}

/*
========================================================
ALERT TEXT + SEND (заглушка)
========================================================
*/

func BuildAlertText(sub PortfolioTokenSubscription, ev TokenOnchainEvent, ref TokenFilterContext) string {
	effective := effectivePortfolioRule(sub.Rule)
	amountUSD, hasUSD := eventAmountUSD(ev, ref.Profile)
	severity := "INFO"
	if hasUSD && amountUSD >= criticalAlertUSD {
		severity = "CRITICAL"
	} else if hasUSD && amountUSD >= effective.MinUSD {
		severity = "WARNING"
	}

	direction := string(ev.Direction)
	if direction == "" {
		direction = "N/A"
	}

	displayToken := sub.AssetSymbol
	if displayToken == "" {
		displayToken = ev.Token.Hex()
	}

	exchangeName := ev.ExchangeName
	if exchangeName == "" {
		exchangeName = "exchange"
	}

	usdText := "n/a"
	if hasUSD {
		usdText = fmt.Sprintf("%.2f", amountUSD)
	}

	amountText := ev.Amount.String()
	if ref.Profile != nil && ref.Profile.Decimals.Valid && ref.Profile.Decimals.Int64 >= 0 {
		amountText = formatTokenAmountHuman(ev.Amount, ref.Profile.Decimals.Int64)
	}

	reason := fmt.Sprintf("minUSD>=%.0f spike>=x%.1f dominance>=%.0f%%", effective.MinUSD, effective.SpikeMult, effective.DominancePct*100)

	return fmt.Sprintf(
		"[Onchain %s] %s / %s %s amount=%s amountUSD=%s reason=%s tx=%s",
		severity,
		displayToken,
		ChainName(int(ev.ChainID)),
		exchangeName+" "+direction,
		amountText,
		usdText,
		reason,
		ev.TxHash.Hex(),
	)
}

func formatTokenAmountHuman(amount *big.Int, decimals int64) string {
	if amount == nil {
		return "0"
	}
	if decimals <= 0 {
		return amount.String()
	}

	den := new(big.Int).Exp(big.NewInt(10), big.NewInt(decimals), nil)
	if den.Sign() == 0 {
		return amount.String()
	}

	human := new(big.Rat).SetFrac(amount, den)
	text := human.FloatString(6)
	text = strings.TrimRight(text, "0")
	text = strings.TrimRight(text, ".")
	if text == "" {
		return "0"
	}
	return text
}

func SendTelegramAlert(userID int64, text string) {
	chatID, err := DB_GetTelegramIDByUserID(userID)
	if err != nil {
		fmt.Printf("[TG][ERR] resolve user_id=%d: %v\n", userID, err)
		return
	}
	if chatID == 0 {
		fmt.Printf("[TG][WARN] empty chat id for user_id=%d\n", userID)
		return
	}
	if err := sendTelegramMessage(chatID, text); err != nil {
		fmt.Printf("[TG][ERR] send chat_id=%d user_id=%d: %v\n", chatID, userID, err)
	}
}

func effectivePortfolioRule(rule AlertRuleForToken) AlertRuleForToken {
	out := rule
	if out.MinUSD < defaultAlertMinUSD {
		out.MinUSD = defaultAlertMinUSD
	}
	if out.SpikeMult < defaultAlertSpikeMult {
		out.SpikeMult = defaultAlertSpikeMult
	}
	if out.DominancePct <= 0 || out.DominancePct > 1 {
		out.DominancePct = defaultAlertDominancePct
	} else if out.DominancePct < defaultAlertDominancePct {
		out.DominancePct = defaultAlertDominancePct
	}
	return out
}

func shouldNotifyPortfolioEvent(sub PortfolioTokenSubscription, ev TokenOnchainEvent, ref TokenFilterContext) bool {
	if ev.Direction == DirNoneToken {
		return false
	}

	amountUSD, hasUSD := eventAmountUSD(ev, ref.Profile)
	critical := hasUSD && amountUSD >= criticalAlertUSD

	effective := effectivePortfolioRule(sub.Rule)
	if !critical && !EvaluateAlertRule(ev, ref, effective) {
		return false
	}

	now := time.Now()
	txKey := fmt.Sprintf("%d:%d:%s", sub.PortfolioID, ev.ChainID, strings.ToLower(ev.TxHash.Hex()))
	slotKey := fmt.Sprintf("%d:%d:%s", sub.PortfolioID, ev.ChainID, strings.ToLower(ev.Token.Hex()))

	portfolioAlertGate.mu.Lock()
	defer portfolioAlertGate.mu.Unlock()

	for k, ts := range portfolioAlertGate.lastByTx {
		if now.Sub(ts) > 2*24*time.Hour {
			delete(portfolioAlertGate.lastByTx, k)
		}
	}
	for k, ts := range portfolioAlertGate.lastBySlot {
		if now.Sub(ts) > 2*24*time.Hour {
			delete(portfolioAlertGate.lastBySlot, k)
		}
	}

	if _, exists := portfolioAlertGate.lastByTx[txKey]; exists {
		return false
	}
	if !critical {
		if last, exists := portfolioAlertGate.lastBySlot[slotKey]; exists && now.Sub(last) < alertCooldownWindow {
			return false
		}
	}

	portfolioAlertGate.lastByTx[txKey] = now
	portfolioAlertGate.lastBySlot[slotKey] = now
	return true
}

func shouldSendDailyPortfolioReport(rep Token24hReport) bool {
	if rep.Transfers >= dailyReportMinTransfers {
		return true
	}
	return rep.VolUSD.Valid && rep.VolUSD.Float64 >= dailyReportMinUSD
}

/*
========================================================
DB — MATCH RULE (як у тебе)
========================================================
*/

func DB_MatchRule(name string) (string, float64, string, error) {
	row := DB.QueryRow(`
		SELECT class, confidence,
		       CONCAT(match_type, ':', match_value)
		FROM classification_rules
		WHERE enabled = TRUE
		  AND (
		    (match_type = 'CONTAINS' AND ? LIKE CONCAT('%', match_value, '%'))
		 OR (match_type = 'PREFIX'   AND ? LIKE CONCAT(match_value, '%'))
		 OR (match_type = 'EXACT'    AND ? = match_value)
		 OR (match_type = 'REGEX'    AND ? REGEXP match_value)
		  )
		ORDER BY priority ASC
		LIMIT 1
	`, name, name, name, name)

	var class string
	var conf float64
	var rule string

	if err := row.Scan(&class, &conf, &rule); err != nil {
		if err == sql.ErrNoRows {
			return "UNKNOWN", 0.50, "NONE", nil
		}
		return "", 0, "", err
	}

	return class, conf, rule, nil
}

/*
========================================================
DB — TOKENS (tracked)
========================================================
*/

func DB_LoadTrackedTokens(chainID int64) (map[string]bool, error) {
	q := `
SELECT LOWER(contract)
FROM tokens_metadata
WHERE chain_id = ? AND onchain_tracking = 1
`
	rows, err := DB.Query(q, chainID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make(map[string]bool)
	for rows.Next() {
		var c string
		if err := rows.Scan(&c); err != nil {
			return nil, err
		}
		out[c] = true
	}
	return out, rows.Err()
}

func DB_LoadTrackedTokensCached(chainID int64) (map[string]bool, error) {
	now := time.Now()

	trackedTokensCache.mu.RLock()
	entry, ok := trackedTokensCache.byChain[chainID]
	if ok && now.Before(entry.expiresAt) {
		trackedTokensCache.mu.RUnlock()
		return entry.tokens, nil
	}
	trackedTokensCache.mu.RUnlock()

	tokens, err := DB_LoadTrackedTokens(chainID)
	if err != nil {
		return nil, err
	}

	trackedTokensCache.mu.Lock()
	trackedTokensCache.byChain[chainID] = trackedTokensCacheEntry{
		tokens:    tokens,
		expiresAt: now.Add(trackedTokensCacheTTL),
	}
	trackedTokensCache.mu.Unlock()

	return tokens, nil
}

/*
========================================================
DB — TOKEN PROFILE (read-only)
========================================================
*/

func DB_LoadTokenProfile(chainID int64, tokenLower string) (*TokenProfile, error) {
	var p TokenProfile

	q := `
SELECT
	chain_id,
	LOWER(contract),
	price_usd,
	circulating_market_cap,
	onchain_market_cap,
	max_total_supply_raw,
	decimals,
	holders,
	top10_pct,
	top50_pct,
	top100_pct
FROM tokens_metadata
WHERE chain_id = ? AND LOWER(contract) = ?
LIMIT 1
`
	err := DB.QueryRow(q, chainID, tokenLower).Scan(
		&p.ChainID,
		&p.TokenLower,
		&p.PriceUSD,
		&p.CirculatingMarketCap,
		&p.OnchainMarketCap,
		&p.MaxTotalSupplyRaw,
		&p.Decimals,
		&p.Holders,
		&p.Top10Pct,
		&p.Top50Pct,
		&p.Top100Pct,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

/*
========================================================
DB — HOURLY ANALYTICS UPSERT (РОЗШИРЕНО)
========================================================
*/

func DB_UpsertTokenHourlyActivity(agg map[string]*HourlyTokenActivity) error {
	// ВАЖЛИВО: потрібні нові колонки (дивись SQL нижче).
	q := `
INSERT INTO token_hourly_activity
(chain_id, token, hour_ts,
 transfer_count,
 total_volume_raw, exchange_in_raw, exchange_out_raw, max_transfer_raw,
 total_volume_usd, exchange_in_usd, exchange_out_usd, max_transfer_usd,
 exchange_transfer_count,
 updated_at)
VALUES
(?, ?, ?,
 ?, ?, ?, ?, ?,
 ?, ?, ?, ?,
 ?, NOW())
ON DUPLICATE KEY UPDATE
transfer_count = transfer_count + VALUES(transfer_count),
total_volume_raw = total_volume_raw + VALUES(total_volume_raw),
exchange_in_raw = exchange_in_raw + VALUES(exchange_in_raw),
exchange_out_raw = exchange_out_raw + VALUES(exchange_out_raw),
max_transfer_raw = GREATEST(max_transfer_raw, VALUES(max_transfer_raw)),

total_volume_usd = IFNULL(total_volume_usd,0) + IFNULL(VALUES(total_volume_usd),0),
exchange_in_usd = IFNULL(exchange_in_usd,0) + IFNULL(VALUES(exchange_in_usd),0),
exchange_out_usd = IFNULL(exchange_out_usd,0) + IFNULL(VALUES(exchange_out_usd),0),
max_transfer_usd = GREATEST(IFNULL(max_transfer_usd,0), IFNULL(VALUES(max_transfer_usd),0)),

exchange_transfer_count = exchange_transfer_count + VALUES(exchange_transfer_count),
updated_at = NOW()
`

	tx, err := DB.Begin()
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(q)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, h := range agg {
		var totalUSD interface{} = nil
		var inUSD interface{} = nil
		var outUSD interface{} = nil
		var maxUSD interface{} = nil

		if h.TotalVolumeUSD.Valid {
			totalUSD = h.TotalVolumeUSD.Float64
		}
		if h.ExchangeInUSD.Valid {
			inUSD = h.ExchangeInUSD.Float64
		}
		if h.ExchangeOutUSD.Valid {
			outUSD = h.ExchangeOutUSD.Float64
		}
		if h.MaxTransferUSD.Valid {
			maxUSD = h.MaxTransferUSD.Float64
		}

		_, err := stmt.Exec(
			h.ChainID,
			strings.ToLower(h.Token.Hex()),
			h.HourTS,

			h.TransferCount,
			h.TotalVolumeRaw.String(),
			h.ExchangeInRaw.String(),
			h.ExchangeOutRaw.String(),
			h.MaxTransferRaw.String(),

			totalUSD,
			inUSD,
			outUSD,
			maxUSD,

			h.ExchangeTransferCount,
		)
		if err != nil {
			return err
		}
	}

	return tx.Commit()
}

/*
========================================================
DB — ПІДПИСКИ ПОРТФЕЛІВ НА ТОКЕН (додано daily_report_enabled)
========================================================
*/

func DB_LoadPortfolioTokenSubscriptions(chainID int64, tokenLower string) ([]PortfolioTokenSubscription, error) {
	q := fmt.Sprintf(`
SELECT
	DISTINCT p.id,
	p.user_id,
	LOWER(tp.contract) AS token_lower,
	%s AS asset_key,
	%s AS asset_symbol,
	ats.min_usd,
	ats.min_raw,
	ats.direction,
	ats.spike_mult,
	ats.dominance_pct,
	ats.daily_report_enabled
FROM portfolios p
JOIN tokens t
  ON t.portfolio_id = p.id
JOIN token_prices tp
  ON tp.id = t.token_price_id
LEFT JOIN tokens_metadata tm
  ON tm.chain_id = tp.chain_id AND LOWER(tm.contract) = LOWER(tp.contract)
LEFT JOIN portfolio_asset_tracking_settings ats
  ON ats.portfolio_id = p.id
 AND ats.asset_key = %s
LEFT JOIN portfolio_asset_tracking_networks atn
  ON atn.portfolio_id = p.id
 AND atn.chain_id = tp.chain_id
 AND atn.token = LOWER(tp.contract)
WHERE
	tp.chain_id = ?
	AND LOWER(tp.contract) = ?
	AND p.onchain_alerts_enabled = 1
	AND COALESCE(ats.enabled, 1) = 1
	AND COALESCE(atn.enabled, 1) = 1
`, portfolioAssetKeyExprSQL("tp", "tm"), portfolioAssetSymbolExprSQL("tp", "tm"), portfolioAssetKeyExprSQL("tp", "tm"))
	rows, err := DB.Query(q, chainID, tokenLower)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []PortfolioTokenSubscription

	for rows.Next() {
		var sub PortfolioTokenSubscription

		var minUSD sql.NullFloat64
		var spike sql.NullFloat64
		var dom sql.NullFloat64
		var minRaw sql.NullString
		var direction sql.NullString
		var daily sql.NullInt64

		if err := rows.Scan(
			&sub.PortfolioID,
			&sub.UserID,
			&sub.TokenLower,
			&sub.AssetKey,
			&sub.AssetSymbol,
			&minUSD,
			&minRaw,
			&direction,
			&spike,
			&dom,
			&daily,
		); err != nil {
			return nil, err
		}

		r := AlertRuleForToken{}
		if minUSD.Valid {
			r.MinUSD = minUSD.Float64
		}
		if minRaw.Valid {
			r.MinRaw = minRaw.String
		}
		if direction.Valid {
			r.Direction = direction.String
		}
		if spike.Valid {
			r.SpikeMult = spike.Float64
		}
		if dom.Valid {
			r.DominancePct = dom.Float64
		}
		sub.Rule = r

		sub.DailyReportEnabled = daily.Valid && daily.Int64 == 1

		out = append(out, sub)
	}

	return out, rows.Err()
}

/*
========================================================
DB — ДАНІ ДЛЯ ФІЛЬТРІВ (hour total/avg + usd total/avg)
========================================================
*/

func DB_LoadTokenFilterContext(chainID int64, tokenLower string, hourTS int64) (TokenFilterContext, error) {
	var ref TokenFilterContext
	ref.HourTotalRaw = big.NewInt(0)
	ref.HourAvgRaw = big.NewInt(0)
	ref.HourTotalUSD = sql.NullFloat64{Valid: false}
	ref.HourAvgUSD = sql.NullFloat64{Valid: false}

	// 1) total volume за годину (+ usd, якщо є)
	{
		q := `
SELECT total_volume_raw, total_volume_usd
FROM token_hourly_activity
WHERE chain_id = ? AND token = ? AND hour_ts = ?
LIMIT 1
`
		var totalStr sql.NullString
		var totalUSD sql.NullFloat64
		err := DB.QueryRow(q, chainID, tokenLower, hourTS).Scan(&totalStr, &totalUSD)
		if err == nil {
			if totalStr.Valid {
				if v, ok := new(big.Int).SetString(totalStr.String, 10); ok {
					ref.HourTotalRaw = v
				}
			}
			ref.HourTotalUSD = totalUSD
		}
	}

	// 2) середній обʼєм за останні 24 години (raw + usd)
	{
		from := hourTS - 24*3600
		to := hourTS - 1
		q := `
SELECT AVG(total_volume_raw), AVG(total_volume_usd)
FROM token_hourly_activity
WHERE chain_id = ? AND token = ? AND hour_ts BETWEEN ? AND ?
`
		var avgRaw sql.NullString
		var avgUSD sql.NullFloat64
		err := DB.QueryRow(q, chainID, tokenLower, from, to).Scan(&avgRaw, &avgUSD)
		if err == nil {
			if avgRaw.Valid {
				s := avgRaw.String
				if i := strings.IndexByte(s, '.'); i >= 0 {
					s = s[:i]
				}
				if v, ok := new(big.Int).SetString(strings.TrimSpace(s), 10); ok {
					ref.HourAvgRaw = v
				}
			}
			ref.HourAvgUSD = avgUSD
		}
	}

	return ref, nil
}

/*
========================================================
DB — PORTFOLIO NOTIFICATIONS (EVENT)
========================================================
*/

func DB_InsertPortfolioNotification(
	portfolioID int64,
	sub PortfolioTokenSubscription,
	ev TokenOnchainEvent,
	rule AlertRuleForToken,
	ref TokenFilterContext,
) (int64, error) {
	ruleHash := hashRule(rule)
	payloadText := BuildAlertText(sub, ev, ref)

	q := `
	INSERT INTO portfolio_notifications
	(portfolio_id, notif_type, chain_id, token, tx_hash, block_time, direction, amount_raw, exchange_name, rule_hash, payload_text, created_at)
	VALUES
	(?, 'EVENT', ?, ?, ?, ?, ?, ?, ?, ?, ?, NOW())
	`
	res, err := DB.Exec(
		q,
		portfolioID,
		ev.ChainID,
		strings.ToLower(ev.Token.Hex()),
		strings.ToLower(ev.TxHash.Hex()),
		ev.BlockTime.UTC(),
		string(ev.Direction),
		ev.Amount.String(),
		ev.ExchangeName,
		ruleHash,
		payloadText,
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

/*
========================================================
DAILY 24h REPORT
========================================================
*/

type Token24hReport struct {
	DayTS int64
	Text  string

	Transfers int64
	VolUSD    sql.NullFloat64
	NetExUSD  sql.NullFloat64
	MaxTxUSD  sql.NullFloat64
}

func DB_LoadToken24hReportForPortfolio(portfolioID int64, chainID int64, tokenLower string, now time.Time) (Token24hReport, error) {
	// беремо попередні 24h по hourly таблиці
	end := now.UTC().Truncate(time.Hour).Unix()
	start := end - 24*3600 + 3600

	q := `
SELECT
	SUM(transfer_count) as transfers,
	SUM(total_volume_usd) as vol_usd,
	SUM(exchange_in_usd) as ex_in_usd,
	SUM(exchange_out_usd) as ex_out_usd,
	MAX(max_transfer_usd) as max_tx_usd
FROM token_hourly_activity
WHERE chain_id = ? AND token = ? AND hour_ts BETWEEN ? AND ?
`
	var transfers sql.NullInt64
	var vol sql.NullFloat64
	var exIn sql.NullFloat64
	var exOut sql.NullFloat64
	var maxTx sql.NullFloat64

	_ = DB.QueryRow(q, chainID, tokenLower, start, end).Scan(&transfers, &vol, &exIn, &exOut, &maxTx)

	net := sql.NullFloat64{Valid: false}
	if exIn.Valid && exOut.Valid {
		net = sql.NullFloat64{Float64: exIn.Float64 - exOut.Float64, Valid: true}
	}

	rep := Token24hReport{
		DayTS:     now.UTC().Truncate(24 * time.Hour).Unix(),
		Transfers: 0,
		VolUSD:    vol,
		NetExUSD:  net,
		MaxTxUSD:  maxTx,
	}
	if transfers.Valid {
		rep.Transfers = transfers.Int64
	}

	rep.Text = fmt.Sprintf(
		"[24h Token Report] token=%s transfers=%d volUSD=%v netExUSD=%v maxTxUSD=%v",
		tokenLower,
		rep.Transfers,
		nullFloatToStr(rep.VolUSD),
		nullFloatToStr(rep.NetExUSD),
		nullFloatToStr(rep.MaxTxUSD),
	)

	return rep, nil
}

func nullFloatToStr(v sql.NullFloat64) string {
	if !v.Valid {
		return "n/a"
	}
	return fmt.Sprintf("%.4f", v.Float64)
}

func DB_InsertPortfolioDailyReportNotification(portfolioID int64, chainID int64, tokenLower string, dayTS int64, rep Token24hReport) (int64, error) {
	// УНИКАЛЬНІСТЬ: (portfolio_id, chain_id, token, day_ts, notif_type) — щоб не спамити
	q := `
INSERT INTO portfolio_notifications
(portfolio_id, notif_type, chain_id, token, day_ts, payload_text, created_at)
VALUES
(?, 'DAILY_REPORT', ?, ?, ?, ?, NOW())
`
	res, err := DB.Exec(q, portfolioID, chainID, tokenLower, dayTS, rep.Text)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}
