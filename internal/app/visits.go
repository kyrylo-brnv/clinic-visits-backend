package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
	"github.com/smithautotest/clinic-visits/internal/visit"
)

func newVisitHandler(pool *pgxpool.Pool) visit.Handler {
	repo := visit.NewPostgresRepository(pool)
	return visit.NewHandler(repo)
}

func newElasticsearchVisitListHandler(client *elasticsearch.Client) visit.ListHandler {
	repo := visit.NewElasticsearchRepository(client)
	return visit.NewListHandler(repo)
}
