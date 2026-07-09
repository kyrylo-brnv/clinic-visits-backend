package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/smithautotest/clinic-visits/internal/config"
	"github.com/smithautotest/clinic-visits/internal/database"
)

func main() {
	err := godotenv.Load()

	if err != nil {
		log.Fatal("Error loading environment variables:", err)
	}

	appServerConfig, err := config.LoadAppServerConfig()
	if err != nil {
		log.Fatal("Error loading config:", err)
	}

	dbConfig, err := config.LoadDatabaseConfig()
	if err != nil {
		log.Fatal("Error loading database config:", err)
	}

	pool, err := database.NewPostgresPool(dbConfig)
	if err != nil {
		log.Fatal("Error connecting to database: ", err)
	}
	defer pool.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("OK"))
	})

	log.Fatal(http.ListenAndServe(":"+strconv.Itoa(appServerConfig.HTTPPort), mux))
}
