package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/smithautotest/clinic-visits/internal/config"
)

type healthResponse struct {
	Status string `json:"status"`
}

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading environment variables:", err)
	}

	appServerConfig, err := config.LoadAppServerConfig()
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(appServerConfig.HTTPPort), mux))
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	payload, err := json.Marshal(healthResponse{Status: "ok"})
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(payload)
}
