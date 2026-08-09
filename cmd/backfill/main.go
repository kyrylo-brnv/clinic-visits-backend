package main

import (
	"context"
	"errors"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/smithautotest/clinic-visits/internal/config"
	"github.com/smithautotest/clinic-visits/internal/database"
	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

const backfillTimeout = 30 * time.Minute

func main() {
	err := godotenv.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		log.Fatal("Error loading environment variables:", err)
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

	signalContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	ctx, cancel := context.WithTimeout(signalContext, backfillTimeout)
	defer cancel()

	if err := elasticsearchClient.RecreateIndices(ctx); err != nil {
		log.Fatal("Error recreating Elasticsearch indices:", err)
	}
	if err := elasticsearch.Backfill(ctx, pool, elasticsearchClient); err != nil {
		log.Fatal("Error backfilling Elasticsearch:", err)
	}

	log.Print("Elasticsearch backfill completed")
}
