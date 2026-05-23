package main

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

var ScanByChainID = map[int]string{
	1:     "https://etherscan.io",
	56:    "https://bscscan.com",
	137:   "https://polygonscan.com",
	42161: "https://arbiscan.io",
	10:    "https://optimistic.etherscan.io",
	8453:  "https://basescan.org",
	43114: "https://snowtrace.io",
	250:   "https://ftmscan.com",
}

type TokenData struct {
	ChainID              int
	Address              string
	Symbol               string
	MaxTotalSupply       string
	Holders              int
	HoldersChange        *float64
	TransfersTotal       string
	Transfers24h         string
	PriceUSD             *float64
	PriceETH             *float64
	PriceChange          *float64
	OnchainMarketCap     string
	CirculatingMarketCap string
	Decimals             int
}

func ParseTokenOverview(chainID int, address string) (TokenData, error) {
	baseURL, ok := ScanByChainID[chainID]
	if !ok {
		return TokenData{}, fmt.Errorf("unsupported chainID: %d", chainID)
	}

	doc, err := fetchDoc(baseURL + "/token/" + address)
	if err != nil {
		return TokenData{}, err
	}

	t := TokenData{
		ChainID: chainID,
		Address: strings.ToLower(address),
	}

	// SYMBOL
	t.Symbol = attr(doc, "#ContentPlaceHolder1_hdnSymbol")

	// MAX TOTAL SUPPLY
	t.MaxTotalSupply = normalizeNumber(attr(doc, "#ContentPlaceHolder1_hdnTotalSupply"))

	// HOLDERS
	rawHolders := doc.
		Find("#ContentPlaceHolder1_tr_tokenHolders > div > div").
		First().
		Clone().
		Children().
		Remove().
		End().
		Text()

	t.Holders = atoiSafe(normalizeNumber(rawHolders))

	// HOLDERS CHANGE %
	t.HoldersChange = parsePercent(
		doc.Find("#ContentPlaceHolder1_tr_tokenHolders span").First().Text(),
	)

	// TRANSFERS TOTAL
	rawTransfersTotal := strings.ReplaceAll(
		doc.Find("#totaltxns").Text(),
		"More than",
		"",
	)
	t.TransfersTotal = normalizeNumber(rawTransfersTotal)

	// TRANSFERS 24H
	rawTransfers24h := doc.
		Find("#transfer-24h-content").
		Clone().
		Children().
		Remove().
		End().
		Text()
	t.Transfers24h = normalizeNumber(rawTransfers24h)

	// PRICE BLOCK
	priceBlock := doc.Find("#ContentPlaceHolder1_tr_valuepertoken").Text()

	t.PriceUSD = parseNumber(extractBetween(priceBlock, "$", "@"))
	t.PriceETH = parseNumber(extractBetween(priceBlock, "@", "ETH"))
	t.PriceChange = parsePercent(priceBlock)

	// ONCHAIN MARKET CAP
	t.OnchainMarketCap = normalizeNumber(
		doc.Find("#ContentPlaceHolder1_tr_marketcap div").Last().Text(),
	)

	// CIRCULATING MARKET CAP
	t.CirculatingMarketCap = normalizeNumber(
		doc.Find("#ContentPlaceHolder1_tr_circulatingmarketcap div").Last().Text(),
	)

	// DECIMALS
	decStr := doc.Find("h4:contains('Decimals') b").First().Text()
	t.Decimals = atoiSafe(decStr)
	if t.Decimals == 0 {
		t.Decimals = 18
	}

	return t, nil
}

/* ================= HELPERS ================= */

func fetchDoc(url string) (*goquery.Document, error) {
	req, _ := http.NewRequest("GET", url, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0")

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return goquery.NewDocumentFromReader(resp.Body)
}

func attr(doc *goquery.Document, sel string) string {
	v, _ := doc.Find(sel).Attr("value")
	return strings.TrimSpace(v)
}

func extractBetween(s, a, b string) string {
	i := strings.Index(s, a)
	j := strings.Index(s, b)
	if i == -1 || j == -1 || j <= i {
		return ""
	}
	return strings.TrimSpace(s[i+len(a) : j])
}

func normalizeNumber(s string) string {
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "$", "")
	s = strings.TrimSpace(s)
	fields := strings.Fields(s)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func parseNumber(s string) *float64 {
	s = normalizeNumber(s)
	if s == "" {
		return nil
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func parsePercent(s string) *float64 {
	if !strings.Contains(s, "%") {
		return nil
	}
	s = strings.ReplaceAll(s, "%", "")
	s = strings.ReplaceAll(s, "(", "")
	s = strings.ReplaceAll(s, ")", "")
	s = strings.ReplaceAll(s, "+", "")
	s = strings.TrimSpace(s)
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return nil
	}
	return &v
}

func atoiSafe(s string) int {
	s = normalizeNumber(s)
	v, _ := strconv.Atoi(s)
	return v
}
