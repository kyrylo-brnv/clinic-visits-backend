package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/smithautotest/clinic-visits/internal/config"
)

func main() {
	godotenv.Load()
	config, err := config.LoadConfig()
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(config.HTTPPort), mux))
}
