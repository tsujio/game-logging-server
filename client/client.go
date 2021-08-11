package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const (
	endpoint = "https://game-logging-server.tsujio.org/log"
)

func Log(gameName string, payload interface{}) error {
	b, err := json.Marshal(map[string]interface{}{
		"game_name": gameName,
		"payload":   payload,
	})
	if err != nil {
		return err
	}
	body := bytes.NewReader(b)

	resp, err := http.Post(endpoint, "application/json", body)
	if err != nil {
		return err
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("Response from game logging server: %s", resp.Status)
	}

	return nil
}

func LogAsync(gameName string, payload interface{}) {
	go Log(gameName, payload)
}
