package client

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

const (
	host = "https://game-logging-server.tsujio.org"
)

var _secret string
var _disabled = false

var mu sync.Mutex

func Enable(secret string) {
	mu.Lock()
	defer mu.Unlock()

	_disabled = false
	_secret = secret
}

func Disable() {
	mu.Lock()
	defer mu.Unlock()

	_disabled = true
	_secret = ""
}

func makeSignature(data, key []byte) ([]byte, error) {
	h := hmac.New(sha256.New, key)
	if _, err := h.Write(data); err != nil {
		return nil, err
	}
	return h.Sum(nil), nil
}

func httpPost(path string, body interface{}) error {
	mu.Lock()
	disabled := _disabled
	secret := _secret
	mu.Unlock()

	if disabled {
		return nil
	}

	b, err := json.Marshal(body)
	if err != nil {
		return err
	}

	sig, err := makeSignature(b, []byte(secret))
	if err != nil {
		return err
	}

	r := bytes.NewReader(b)

	req, err := http.NewRequest(http.MethodPost, host+path, r)

	req.Header.Set("Authorization", "Bearer "+hex.EncodeToString(sig))
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Error response from game logging server: %s", resp.Status)
	}

	return nil
}

func Log(gameName string, payload interface{}) error {
	return httpPost("/log", map[string]interface{}{
		"game_name": gameName,
		"payload":   payload,
	})
}

func LogAsync(gameName string, payload interface{}) {
	go Log(gameName, payload)
}

func RegisterScore(gameName, playerID string, score int) error {
	return httpPost("/score", map[string]interface{}{
		"game_name": gameName,
		"player_id": playerID,
		"score":     score,
	})
}

type GameScore struct {
	GameName  string    `json:"game_name"`
	Timestamp time.Time `json:"timestamp"`
	PlayerID  string    `json:"player_id"`
	Score     int       `json:"score"`
}

func GetScoreList(gameName string) ([]GameScore, error) {
	resp, err := http.Get(host + "/score?game_name=" + gameName)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Error response from game logging server: %s", resp.Status)
	}

	var data struct {
		Scores []GameScore `json:"scores"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	return data.Scores, nil
}
