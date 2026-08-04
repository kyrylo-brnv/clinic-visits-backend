package app

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/routes"
)

func New(pool *pgxpool.Pool) http.Handler {
	deps := routes.Dependencies{
		Patients: newPatientHandler(pool),
		Doctors:  newDoctorHandler(pool),
	}

	return routes.NewRouter(deps)
}
