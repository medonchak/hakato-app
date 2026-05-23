package main

// POST /tg/webapp/verify
// body: { "initData": "<raw initData string>" }
import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"time"
)

type webAppVerifyReq struct {
	InitData string `json:"initData"`
}

type webAppUser struct {
	ID        int64  `json:"id"`
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
	Username  string `json:"username"`
	Language  string `json:"language_code"`
}

type webAppVerifyResp struct {
	User webAppUser `json:"user"`
	Ts   int64      `json:"ts"`
}

var botToken = os.Getenv("TG_BOT_TOKEN")

func TgWebAppVerifyHandler(w http.ResponseWriter, r *http.Request) {
	var req webAppVerifyReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}
	if req.InitData == "" || botToken == "" {
		http.Error(w, "missing initData or bot token", http.StatusBadRequest)
		return
	}

	// 1) парсимо initData як query-string
	values, err := url.ParseQuery(req.InitData)
	if err != nil {
		http.Error(w, "bad initData", http.StatusBadRequest)
		return
	}

	// 2) дістаємо hash та будуємо data_check_string (без hash)
	hash := values.Get("hash")
	if hash == "" {
		http.Error(w, "no hash", http.StatusBadRequest)
		return
	}

	// зберігаємо пари key=value (для ключів, що присутні)
	pairs := make([]string, 0, len(values))
	for k, v := range values {
		if k == "hash" || len(v) == 0 {
			continue
		}
		// беремо перше значення
		pairs = append(pairs, k+"="+v[0])
	}
	sort.Strings(pairs)
	dataCheck := strings.Join(pairs, "\n")

	// 3) перевіряємо підпис: HMAC_SHA256(dataCheck, secret=SHA256(BOT_TOKEN))
	secret := sha256.Sum256([]byte(botToken))
	mac := hmac.New(sha256.New, secret[:])
	mac.Write([]byte(dataCheck))
	expected := hex.EncodeToString(mac.Sum(nil))

	if subtle.ConstantTimeCompare([]byte(strings.ToLower(expected)), []byte(strings.ToLower(hash))) != 1 {
		http.Error(w, "invalid signature", http.StatusUnauthorized)
		return
	}

	// 4) парсимо поле user (JSON-рядок усередині initData)
	var user webAppUser
	if uStr := values.Get("user"); uStr != "" {
		_ = json.Unmarshal([]byte(uStr), &user)
	}

	// TODO: тут можеш зберегти user.ID до своєї сесії/акаунта
	// map[sessionId]tgId або що тобі потрібно

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(webAppVerifyResp{
		User: user,
		Ts:   time.Now().Unix(),
	})
}
