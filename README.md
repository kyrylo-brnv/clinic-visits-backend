# Clinic Visits API

## Run the complete local stack

Requires Docker Compose. No `.env` file is required for the included local
defaults.

```sh
docker compose up --build
```

This starts PostgreSQL, applies the SQL migrations once, starts Elasticsearch,
and then starts the API. Wait for the API to report readiness, then use:

- Health: [http://localhost:8080/health](http://localhost:8080/health)
- Swagger UI: [http://localhost:8080/docs](http://localhost:8080/docs)
- OpenAPI JSON: [http://localhost:8080/openapi.json](http://localhost:8080/openapi.json)

Press `Ctrl-C` to stop the foreground stack. To remove the stopped containers
while preserving database and Elasticsearch data, run:

```sh
docker compose down
```

## Host API development

Requires Docker Compose, Go 1.26.4, and the
[golang-migrate CLI](https://github.com/golang-migrate/migrate/tree/master/cmd/migrate).

Copy `.env.example` to `.env`. The included local-only credentials match the
Docker Compose defaults; replace them only if you also update the corresponding
local database configuration.

```sh
cp .env.example .env
make dev
```

`make dev` starts and waits for local PostgreSQL and the pinned single-node
Elasticsearch service, applies migrations, and runs the Go API in the
foreground. Wait for `Clinic Visits API is ready at
http://localhost:8080/health` before starting integration or Playwright tests.
The API checks Elasticsearch health and idempotently creates the
`clinic-visits-bootstrap-v1` index. After initialization, verify the index
with:

```sh
curl --head http://localhost:9200/clinic-visits-bootstrap-v1
```

Stop the local dependencies while preserving their data with:

```sh
docker compose stop
```

## API documentation

With the API running, open the Swagger UI at
[http://localhost:8080/docs](http://localhost:8080/docs). The OpenAPI 3.0 JSON
document is available directly at
[http://localhost:8080/openapi.json](http://localhost:8080/openapi.json).
