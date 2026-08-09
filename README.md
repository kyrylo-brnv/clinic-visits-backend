# Clinic Visits API

## Run the local stack

Requires Docker Compose. No `.env` file is required for the included local
defaults.

For a first bootstrap, apply migrations and populate the persistent
Elasticsearch indices before starting the API:

```sh
docker compose run --build --rm backfill
docker compose up --build
```

For later starts, only `docker compose up --build` is needed. Normal API
startup validates Elasticsearch and idempotently creates any missing mappings;
it never performs a full backfill. The `backfill` service is an explicit tool
profile and does not run during `docker compose up` or API restarts.

Wait for the API to report readiness, then use:

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
```

For the first bootstrap, start the dependencies and run the one-off backfill
before starting the API:

```sh
docker compose up -d --wait postgres elasticsearch
make backfill
make dev
```

On later starts, use `make dev` by itself. It starts and waits for local
PostgreSQL and the pinned single-node Elasticsearch service, applies migrations,
and runs the Go API in the foreground. Wait for `Clinic Visits API is ready at
http://localhost:8080/health` before starting integration or Playwright tests.
After initialization, verify an index with:

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

Run migrations as a one-off deployment job before starting either the API or a
backfill. Run `clinic-visits-backfill` once for the first bootstrap, or
explicitly during repair while API writes are paused. The command checks
Elasticsearch health, deletes and recreates the four versioned indices
(`doctors-v1`, `visits-v1`, `patients-v1`, and `clinics-v1`) with their mappings,
and loads the current PostgreSQL state. Recreating the indices removes documents
that no longer exist in PostgreSQL. The command exits nonzero if any step fails.

To repair one read-model index without rebuilding the other three, run the
explicit command with exactly one index name:

```sh
go run ./cmd/rebuild-index doctors-v1
```

Supported names are `doctors-v1`, `patients-v1`, `clinics-v1`, and `visits-v1`.
The command recreates and reloads only the named index from the current
PostgreSQL snapshot. Run it only while API writes and outbox processing are
paused: an outbox update or delete processed during the rebuild can otherwise
be overwritten by the older snapshot document.

Normal API startup only checks Elasticsearch health and creates missing indices.
While the API runs, a background worker continuously processes transactional
visit outbox events in bounded batches, keeping visit documents and related
nested visit arrays current. Retryable database or Elasticsearch failures are
logged and retried without stopping the API.

Use persistent storage for both PostgreSQL and Elasticsearch. PostgreSQL is the
source of truth for writes and durable data; Elasticsearch is a persistent,
denormalized read model for search and v2 reads. Restarting the API does not
repopulate it.

`GET /health` and `HEAD /health` return HTTP 200 with application status `ok`
once the HTTP server is accepting requests. The endpoint does not probe
PostgreSQL or Elasticsearch; dependency availability is validated separately
by migrations, API startup, and the Compose health checks.

## API documentation

With the API running, the Swagger UI is served at `/docs` and the OpenAPI 3.0
JSON document at `/openapi.json`. Locally, open
[http://localhost:8080/docs](http://localhost:8080/docs) or
[http://localhost:8080/openapi.json](http://localhost:8080/openapi.json).
