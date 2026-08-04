package app

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/doctor"
)

func newDoctorHandler(pool *pgxpool.Pool) doctor.Handler {
	repo := doctor.NewPostgresRepository(pool)
	return doctor.NewHandler(repo)
}
