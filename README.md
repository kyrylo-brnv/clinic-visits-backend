# Clinic Visits API

## Local development

Copy `.env.example` to `.env`. PostgreSQL is an external prerequisite and must
be available at the configured `DB_HOST` and `DB_PORT`.

```sh
make dev
```

`make dev` starts the pinned single-node Elasticsearch service, applies
migrations, and runs the Go API in the foreground. Starting the API checks
Elasticsearch health and idempotently creates the `clinic-visits-bootstrap-v1`
index. Wait for `Clinic Visits API is ready at http://localhost:8080/health`
before starting integration or Playwright tests. After the API has initialized,
verify the index with:

```sh
curl --head http://localhost:9200/clinic-visits-bootstrap-v1
```

Stop only Elasticsearch with `docker compose stop elasticsearch`.
