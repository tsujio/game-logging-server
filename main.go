package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/rs/cors"
	"golang.org/x/xerrors"

	"github.com/tsujio/game-logging-server/storages"
)

func appendLog(w http.ResponseWriter, r *http.Request, storage storages.Storage) {
	type Input struct {
		GameName string                 `json:"game_name"`
		Payload  map[string]interface{} `json:"payload"`
	}

	var input Input
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

	content := struct {
		ServerTimestamp time.Time   `json:"serverTimestamp"`
		RemoteAddr      string      `json:"remoteAddr"`
		GameName        string      `json:"gameName"`
		Payload         interface{} `json:"payload"`
	}{
		ServerTimestamp: time.Now().UTC(),
		RemoteAddr:      remoteAddr,
		GameName:        input.GameName,
		Payload:         input.Payload,
	}

	// Insert log
	if err := storage.InsertLog(context.Background(), content.GameName, content.ServerTimestamp, content); err != nil {
		log.Printf("Failed to insert log: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to insert log"})
		return
	}
}

func run(host string, port int, storage storages.Storage) error {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		appendLog(w, r, storage)
	}).Methods(http.MethodPost)

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
	storage, err := storages.New(os.Getenv("BUCKET"))
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
