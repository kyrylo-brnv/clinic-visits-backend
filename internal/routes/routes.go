package routes

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/doctor"
	"github.com/smithautotest/clinic-visits/internal/patient"
	doctors "github.com/smithautotest/clinic-visits/internal/routes/v1/doctors"
	patients "github.com/smithautotest/clinic-visits/internal/routes/v1/patients"
	visits "github.com/smithautotest/clinic-visits/internal/routes/v1/visits"
	v2patients "github.com/smithautotest/clinic-visits/internal/routes/v2/patients"
	"github.com/smithautotest/clinic-visits/internal/visit"
)

type Dependencies struct {
	Patients   patient.Handler
	V2Patients patient.Handler
	Doctors    doctor.Handler
	Visits     visit.Handler
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	patients.Register(mux, deps.Patients)
	v2patients.Register(mux, deps.V2Patients)
	doctors.Register(mux, deps.Doctors)
	visits.Register(mux, deps.Visits)

	return mux
}
