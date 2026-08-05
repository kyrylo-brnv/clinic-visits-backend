package main

import (
	"context"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/joho/godotenv"
	"github.com/smithautotest/clinic-visits/internal/app"
	"github.com/smithautotest/clinic-visits/internal/config"
	"github.com/smithautotest/clinic-visits/internal/database"
	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
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

	elasticsearchConfig, err := config.LoadElasticsearchConfig()
	if err != nil {
		log.Fatal("Error loading Elasticsearch config:", err)
	}

	pool, err := database.NewPostgresPool(databaseConfig)
	if err != nil {
		log.Fatal("Error creating postgres pool:", err)
	}
	defer pool.Close()

	elasticsearchClient, err := elasticsearch.NewClient(elasticsearchConfig)
	if err != nil {
		log.Fatal("Error creating Elasticsearch client:", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if err := elasticsearchClient.Initialize(ctx); err != nil {
		log.Fatal("Error initializing Elasticsearch:", err)
	}
	if err := elasticsearch.Backfill(ctx, pool, elasticsearchClient); err != nil {
		log.Fatal("Error backfilling Elasticsearch:", err)
	}

	router := app.New(pool)
	address := ":" + strconv.Itoa(appServerConfig.HTTPPort)
	log.Fatal(http.ListenAndServe(address, router))
}
