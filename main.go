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
	"gorm.io/gorm"
)

func appendLog(w http.ResponseWriter, r *http.Request) {
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

	payload, err := json.Marshal(&input.Payload)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to encode payload"})
		return
	}

	// Insert log
	db := r.Context().Value("db").(*gorm.DB)
	err = db.Exec(`
	INSERT INTO game_logs(server_timestamp, remote_addr, game_name, payload) VALUES (?, ?, ?, ?)
	`, time.Now().UTC(), remoteAddr, input.GameName, string(payload)).Error
	if err != nil {
		log.Printf("Failed to insert log: %v", err)
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]interface{}{"message": "Failed to insert log"})
		return
	}
}

func run(host string, port int, db *gorm.DB) error {
	router := mux.NewRouter().StrictSlash(true)

	router.HandleFunc("/log", func(w http.ResponseWriter, r *http.Request) {
		ctx := context.WithValue(r.Context(), "db", db)
		r = r.WithContext(ctx)
		appendLog(w, r)
	}).Methods(http.MethodPost)

	handler := cors.AllowAll().Handler(router)

	addr := fmt.Sprintf("%s:%d", host, port)
	err := http.ListenAndServe(addr, handler)
	if err != nil {
		return xerrors.Errorf("Failed to start api: %w", err)
	}

	return nil
}

func main() {
	// Set up db
	dbUser := os.Getenv("DB_USER")
	dbPassword := os.Getenv("DB_PASSWORD")
	dbPort, _ := strconv.Atoi(os.Getenv("DB_PORT"))
	dbHost := os.Getenv("DB_HOST")
	dbName := os.Getenv("DB_NAME")
	dbConfig := &DBConfig{
		User:     dbUser,
		Password: dbPassword,
		Host:     dbHost,
		Port:     dbPort,
		DBName:   dbName,
	}
	migrationsDir := os.Getenv("MIGRATIONS_DIR")
	if migrationsDir == "" {
		migrationsDir = "migrations"
	}
	err := SetupDB(dbConfig, migrationsDir)
	if err != nil {
		log.Fatalf("Failed to set up db: %+v", err)
	}
	db, err := OpenDB(dbConfig)
	if err != nil {
		log.Fatalf("Failed to open db: %+v", err)
	}

	host := os.Getenv("HOST")
	port, err := strconv.Atoi(os.Getenv("PORT"))
	if err != nil {
		port = 8000
	}

	err = run(host, port, db)
	if err != nil {
		log.Fatalf("Failed to run api: %+v", err)
	}
}
