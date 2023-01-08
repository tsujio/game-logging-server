package main

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"cloud.google.com/go/compute/metadata"
	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"golang.org/x/xerrors"

	"github.com/tsujio/game-logging-server/storages"
)

func verifySignature(gameName string, body []byte, r *http.Request, storage storages.Storage) error {
	signatureStr := r.Header.Get("Authorization")
	signatureStr = strings.TrimPrefix(signatureStr, "Bearer ")
	signature, err := hex.DecodeString(signatureStr)
	if err != nil {
		return err
	}

	secret, err := storage.GetGameSecret(context.Background(), gameName)
	if err != nil {
		return err
	}

	h := hmac.New(sha256.New, []byte(secret))
	if _, err := h.Write(body); err != nil {
		return err
	}
	sig := h.Sum(nil)

	if !bytes.Equal(signature, sig) {
		return fmt.Errorf("Invalid signature")
	}

	return nil
}

type AppendLogInput struct {
	GameName string                 `json:"game_name"`
	Payload  map[string]interface{} `json:"payload"`
}

func appendLog(w http.ResponseWriter, r *http.Request, storage storages.Storage) {
	var input AppendLogInput
	err := json.NewDecoder(r.Body).Decode(&input)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to decode request body"})
		return
	}

	// Validation
	if input.GameName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Missing game_name"})
		return
	}

	remoteAddr := r.Header.Get("X-Forwarded-For")

	payloadJSON, err := json.Marshal(input.Payload)
	if err != nil {
		log.Printf("Failed to encode payload as json: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to encode log payload"})
		return
	}

	content := struct {
		ServerTimestamp time.Time `json:"serverTimestamp"`
		RemoteAddr      string    `json:"remoteAddr"`
		GameName        string    `json:"gameName"`
		PayloadJSON     string    `json:"payloadJson"`
	}{
		ServerTimestamp: time.Now().UTC(),
		RemoteAddr:      remoteAddr,
		GameName:        input.GameName,
		PayloadJSON:     string(payloadJSON),
	}

	// Insert log
	if err := storage.InsertLog(context.Background(), content.GameName, content.ServerTimestamp, content); err != nil {
		log.Printf("Failed to insert log: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to insert log"})
		return
	}
}

type RegisterScoreInput struct {
	GameName string `json:"game_name"`
	PlayerID string `json:"player_id"`
	Score    int    `json:"score"`
}

func registerScore(w http.ResponseWriter, r *http.Request, storage storages.Storage) {
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to read request body"})
		return
	}

	var input RegisterScoreInput
	if err := json.Unmarshal(body, &input); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to decode request body"})
		return
	}

	// Validation
	if input.GameName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Missing game_name"})
		return
	}

	if err := verifySignature(input.GameName, body, r, storage); err != nil {
		log.Printf("Failed to verify signature: %v", err)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Invalid signature"})
		return
	}

	// Register
	score := &storages.GameScore{
		GameName:  input.GameName,
		Timestamp: time.Now().UTC(),
		PlayerID:  input.PlayerID,
		Score:     input.Score,
	}
	if err := storage.RegisterScore(context.Background(), score); err != nil {
		log.Printf("Failed to register score: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to register score"})
		return
	}
}

type Score struct {
	GameName  string    `json:"game_name"`
	Timestamp time.Time `json:"timestamp"`
	PlayerID  string    `json:"player_id"`
	Score     int       `json:"score"`
}

type GetScoreOutput struct {
	Scores []Score `json:"scores"`
}

func getScore(w http.ResponseWriter, r *http.Request, storage storages.Storage) {
	params := r.URL.Query()

	gameName := params.Get("game_name")
	if gameName == "" {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Missing game_name"})
		return
	}

	scores, err := storage.GetScoreList(context.Background(), gameName)
	if err != nil {
		log.Printf("Failed to get scores: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to get scores"})
		return
	}

	outputScores := []Score{}
	for _, s := range scores {
		outputScores = append(outputScores, Score{
			GameName:  s.GameName,
			Timestamp: s.Timestamp,
			PlayerID:  s.PlayerID,
			Score:     s.Score,
		})
	}
	output := GetScoreOutput{
		Scores: outputScores,
	}

	if err := json.NewEncoder(w).Encode(&output); err != nil {
		log.Printf("Failed to encode output: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to encode output"})
		return
	}
}

func run(host string, port int, storage storages.Storage) error {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		appendLog(w, r, storage)
	}).Methods(http.MethodPost)

	router.HandleFunc("/score", func(w http.ResponseWriter, r *http.Request) {
		registerScore(w, r, storage)
	}).Methods(http.MethodPost)

	router.HandleFunc("/score", func(w http.ResponseWriter, r *http.Request) {
		getScore(w, r, storage)
	}).Methods(http.MethodGet)

	handler := cors.AllowAll().Handler(router)

	addr := fmt.Sprintf("%s:%d", host, port)

	log.Printf("Running on %s\n", addr)

	err := http.ListenAndServe(addr, handler)
	if err != nil {
		return xerrors.Errorf("Failed to start api: %w", err)
	}

	return nil
}

func main() {
	projectID := os.Getenv("PROJECT_ID")
	if projectID == "" {
		if pid, err := metadata.ProjectID(); err == nil {
			projectID = pid
		}
	}

	bucket := os.Getenv("BUCKET")

	storage, err := storages.New(projectID, bucket)
	if err != nil {
		log.Fatalf("Failed to create storage client: %+v", err)
	}
	defer storage.Close()

	host := os.Getenv("HOST")
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		port = 8000
	}

	err = run(host, port, storage)
	if err != nil {
		log.Fatalf("Failed to run api: %+v", err)
	}
}
