package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/joho/godotenv"
	"github.com/smithautotest/clinic-visits/internal/app"
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

	databaseConfig, err := config.LoadDatabaseConfig()
	if err != nil {
		log.Fatal("Error loading database config:", err)
	}

	pool, err := database.NewPostgresPool(databaseConfig)
	if err != nil {
		log.Fatal("Error creating postgres pool:", err)
	}
	defer pool.Close()

	router := app.New(pool)
	address := ":" + strconv.Itoa(appServerConfig.HTTPPort)
	log.Fatal(http.ListenAndServe(address, router))
}
