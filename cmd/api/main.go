package main

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/smithautotest/clinic-visits/internal/app"
	"github.com/smithautotest/clinic-visits/internal/config"
	"github.com/smithautotest/clinic-visits/internal/database"
	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
	"github.com/smithautotest/clinic-visits/internal/outbox"
)

const apiShutdownTimeout = 10 * time.Second

func main() {
	err := godotenv.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
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

	appContext, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	initializationContext, cancelInitialization := context.WithTimeout(appContext, 5*time.Minute)
	if err := elasticsearchClient.Initialize(initializationContext); err != nil {
		cancelInitialization()
		log.Fatal("Error initializing Elasticsearch:", err)
	}
	cancelInitialization()

	router := app.New(pool, elasticsearchClient)
	address := ":" + strconv.Itoa(appServerConfig.HTTPPort)
	listener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("Error binding HTTP server on %s: %v", address, err)
	}

	processor := outbox.NewProcessor(
		pool,
		elasticsearch.NewOutboxEventConsumer(pool, elasticsearchClient),
	)
	workerDone := make(chan struct{})
	go func() {
		defer close(workerDone)
		outbox.RunWorker(appContext, processor, func(err error) {
			log.Printf("Error synchronizing outbox events; will retry: %v", err)
		})
	}()

	server := &http.Server{Handler: router}
	serverErrors := make(chan error, 1)
	go func() {
		serverErrors <- server.Serve(listener)
	}()

	log.Printf("Clinic Visits API is ready at http://localhost:%d/health", appServerConfig.HTTPPort)
	select {
	case err := <-serverErrors:
		stop()
		<-workerDone
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("Error serving Clinic Visits API:", err)
		}
	case <-appContext.Done():
		shutdownContext, cancelShutdown := context.WithTimeout(context.Background(), apiShutdownTimeout)
		shutdownErr := server.Shutdown(shutdownContext)
		cancelShutdown()
		if shutdownErr != nil {
			_ = server.Close()
		}

		serveErr := <-serverErrors
		<-workerDone
		if shutdownErr != nil {
			log.Fatal("Error shutting down Clinic Visits API:", shutdownErr)
		}
		if serveErr != nil && !errors.Is(serveErr, http.ErrServerClosed) {
			log.Fatal("Error serving Clinic Visits API:", serveErr)
		}
	}
}
