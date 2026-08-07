package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/doctor"
	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
)

func newDoctorHandler(pool *pgxpool.Pool) doctor.Handler {
	repo := doctor.NewPostgresRepository(pool)
	return doctor.NewHandler(repo)
}

func newElasticsearchDoctorHandler(client *elasticsearch.Client) doctor.Handler {
	repo := doctor.NewElasticsearchRepository(client)
	return doctor.NewHandler(repo)
}
