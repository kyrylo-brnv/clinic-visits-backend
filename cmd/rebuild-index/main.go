package main

import (
	"context"
	"errors"
	"fmt"
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

const rebuildIndexTimeout = 30 * time.Minute

func main() {
	indexName, err := rebuildIndexName(os.Args[1:])
	if err != nil {
		log.Fatal(err)
	}

	err = godotenv.Load()
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
	ctx, cancel := context.WithTimeout(signalContext, rebuildIndexTimeout)
	defer cancel()

	if err := elasticsearchClient.RecreateIndex(ctx, indexName); err != nil {
		log.Fatal("Error recreating Elasticsearch index:", err)
	}
	if err := elasticsearch.BackfillIndex(ctx, pool, elasticsearchClient, indexName); err != nil {
		log.Fatal("Error backfilling Elasticsearch index:", err)
	}

	log.Printf("Elasticsearch index %s rebuild completed", indexName)
}

func rebuildIndexName(arguments []string) (string, error) {
	if len(arguments) != 1 {
		return "", fmt.Errorf("expected exactly one Elasticsearch index name: %s, %s, %s, or %s", elasticsearch.DoctorsIndexName, elasticsearch.PatientsIndexName, elasticsearch.ClinicsIndexName, elasticsearch.VisitsIndexName)
	}
	if err := elasticsearch.ValidateIndexName(arguments[0]); err != nil {
		return "", err
	}

	return arguments[0], nil
}
