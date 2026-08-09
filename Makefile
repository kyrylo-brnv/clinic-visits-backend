include .env

MIGRATION_DB_URL := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: dev backfill migrate-create migrate-up migrate-down migrate-version run

dev:
	@echo "==> Dev stage: starting and waiting for PostgreSQL and Elasticsearch"
	@docker compose up -d --wait postgres elasticsearch
	@echo "==> Dev stage: applying database migrations"
	@$(MAKE) --no-print-directory migrate-up
	@echo "==> Dev stage: starting Clinic Visits API"
	@go run ./cmd/api

backfill: migrate-up
	@go run ./cmd/backfill

migrate-create:
	@test -n "$(name)" || (echo "Usage: make migrate-create name=create_doctors"; exit 1)
	@migrate create -ext sql -dir ./migrations -seq "$(name)"

migrate-up:
	@migrate -path ./migrations -database "$(MIGRATION_DB_URL)" up

migrate-down:
	@migrate -path ./migrations -database "$(MIGRATION_DB_URL)" down 1

migrate-version:
	@migrate -path ./migrations -database "$(MIGRATION_DB_URL)" version

run: migrate-up
	@go run ./cmd/api
