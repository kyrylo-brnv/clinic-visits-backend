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
The API checks Elasticsearch health, idempotently initializes and backfills
the `doctors-v1`, `visits-v1`, `patients-v1`, and `clinics-v1` indices from
PostgreSQL. After initialization, verify an index with:

```sh
curl --head http://localhost:9200/doctors-v1
```

Stop the local dependencies while preserving their data with:

```sh
docker compose stop
```

## Production and cloud deployment

Configure the API through runtime environment variables. Supply credentials
from the platform's secret manager; do not put production values in source
control.

| Variable | Purpose |
| --- | --- |
| `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE` | PostgreSQL connection |
| `ELASTICSEARCH_URL` | Absolute HTTP or HTTPS Elasticsearch URL |
| `HTTP_PORT` | API listen port |

Run migrations as a one-off deployment job before starting the API. The API
must start only after PostgreSQL migrations complete and Elasticsearch is
available. During startup it checks Elasticsearch health, initializes and
backfills the four versioned indices (`doctors-v1`, `visits-v1`,
`patients-v1`, and `clinics-v1`) before accepting traffic. Use `GET /health`
as the readiness endpoint.

Use persistent storage for both PostgreSQL and Elasticsearch. PostgreSQL is
the source of truth for writes and durable data; Elasticsearch is a
denormalized read model for search and v2 reads, and can be rebuilt from
PostgreSQL when needed.

## API documentation

With the API running, the Swagger UI is served at `/docs` and the OpenAPI 3.0
JSON document at `/openapi.json`. Locally, open
[http://localhost:8080/docs](http://localhost:8080/docs) or
[http://localhost:8080/openapi.json](http://localhost:8080/openapi.json).
