include .env

MIGRATION_DB_URL := postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

.PHONY: migrate-create migrate-up migrate-down migrate-version run

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
