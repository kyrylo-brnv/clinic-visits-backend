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

const syncIndexTimeout = 5 * time.Minute

func main() {
	indexName, ids, err := syncIndexArguments(os.Args[1:])
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
	ctx, cancel := context.WithTimeout(signalContext, syncIndexTimeout)
	defer cancel()

	if err := elasticsearchClient.Initialize(ctx); err != nil {
		log.Fatal("Error initializing Elasticsearch:", err)
	}
	if err := elasticsearch.SyncIndex(ctx, pool, elasticsearchClient, indexName, ids); err != nil {
		log.Fatal("Error synchronizing Elasticsearch index:", err)
	}

	log.Printf("Elasticsearch index %s synchronization completed for %d IDs", indexName, len(ids))
}

func syncIndexArguments(arguments []string) (string, []string, error) {
	if len(arguments) == 0 {
		return "", nil, fmt.Errorf("expected an Elasticsearch index name followed by at least one PostgreSQL UUID ID")
	}
	ids := arguments[1:]
	if _, err := elasticsearch.ValidateSyncIndexRequest(arguments[0], ids); err != nil {
		return "", nil, err
	}

	return arguments[0], ids, nil
}
