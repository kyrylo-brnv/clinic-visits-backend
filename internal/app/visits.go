package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/visit"
)

func newVisitHandler(pool *pgxpool.Pool) visit.Handler {
	repo := visit.NewPostgresRepository(pool)
	return visit.NewHandler(repo)
}
