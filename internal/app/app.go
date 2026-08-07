package app

import (
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/elasticsearch"
	"github.com/smithautotest/clinic-visits/internal/routes"
)

func New(pool *pgxpool.Pool, elasticsearchClient *elasticsearch.Client) http.Handler {
	deps := routes.Dependencies{
		Patients:   newPatientHandler(pool),
		V2Patients: newElasticsearchPatientHandler(elasticsearchClient),
		Doctors:    newDoctorHandler(pool),
		V2Doctors:  newElasticsearchDoctorHandler(elasticsearchClient),
		Visits:     newVisitHandler(pool),
	}

	return routes.NewRouter(deps)
}
