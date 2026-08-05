# Clinic Visits API

## Local Elasticsearch

Copy `.env.example` to `.env`, then start the pinned single-node Elasticsearch service:

```sh
docker compose up -d --wait elasticsearch
curl http://localhost:9200
```

Starting the Go API checks Elasticsearch health and idempotently creates the
`clinic-visits-bootstrap-v1` index. After the API has initialized it, verify the
index with:

```sh
curl --head http://localhost:9200/clinic-visits-bootstrap-v1
```

Stop only Elasticsearch with `docker compose stop elasticsearch`.
