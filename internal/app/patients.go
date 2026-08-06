package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
	"github.com/smithautotest/clinic-visits/internal/patient"
)

func newPatientHandler(pool *pgxpool.Pool) patient.Handler {
	repo := patient.NewPostgresRepository(pool)
	return patient.NewHandler(repo)
}

func newElasticsearchPatientHandler(client *elasticsearch.Client) patient.Handler {
	repo := patient.NewElasticsearchRepository(client)
	return patient.NewHandler(repo)
}
