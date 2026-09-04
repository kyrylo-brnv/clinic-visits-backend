package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
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
	backfillOptions, err := parseBackfillOptions(os.Args[1:])
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
	ctx, cancel := context.WithTimeout(signalContext, backfillTimeout)
	defer cancel()

	if err := elasticsearchClient.RecreateIndices(ctx); err != nil {
		log.Fatal("Error recreating Elasticsearch indices:", err)
	}
	if err := elasticsearch.BackfillWithOptions(ctx, pool, elasticsearchClient, backfillOptions); err != nil {
		log.Fatal("Error backfilling Elasticsearch:", err)
	}

	log.Print("Elasticsearch backfill completed")
}

func parseBackfillOptions(arguments []string) (elasticsearch.BackfillOptions, error) {
	options := elasticsearch.DefaultBackfillOptions()
	flags := flag.NewFlagSet("backfill", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	flags.IntVar(&options.BatchSize, "batch-size", options.BatchSize, "documents per worker batch")
	flags.IntVar(&options.Concurrency, "concurrency", options.Concurrency, "number of backfill workers")
	if err := flags.Parse(arguments); err != nil {
		return elasticsearch.BackfillOptions{}, fmt.Errorf("parse backfill options: %w", err)
	}
	if flags.NArg() != 0 {
		return elasticsearch.BackfillOptions{}, fmt.Errorf("unexpected backfill argument %q", flags.Arg(0))
	}
	if err := elasticsearch.ValidateBackfillOptions(options); err != nil {
		return elasticsearch.BackfillOptions{}, err
	}
	return options, nil
}
