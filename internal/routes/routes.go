package routes

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/apidocs"
	"github.com/smithautotest/clinic-visits/internal/doctor"
	"github.com/smithautotest/clinic-visits/internal/httpapi"
	"github.com/smithautotest/clinic-visits/internal/patient"
	doctors "github.com/smithautotest/clinic-visits/internal/routes/v1/doctors"
	patients "github.com/smithautotest/clinic-visits/internal/routes/v1/patients"
	visits "github.com/smithautotest/clinic-visits/internal/routes/v1/visits"
	v2doctors "github.com/smithautotest/clinic-visits/internal/routes/v2/doctors"
	v2patients "github.com/smithautotest/clinic-visits/internal/routes/v2/patients"
	v2visits "github.com/smithautotest/clinic-visits/internal/routes/v2/visits"
	"github.com/smithautotest/clinic-visits/internal/visit"
)

type Dependencies struct {
	Patients   patient.Handler
	V2Patients patient.Handler
	Doctors    doctor.Handler
	V2Doctors  doctor.Handler
	Visits     visit.Handler
	V2Visits   visit.ListHandler
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("/health", healthHandler)
	apidocs.Register(mux)
	patients.Register(mux, deps.Patients)
	v2patients.Register(mux, deps.V2Patients)
	doctors.Register(mux, deps.Doctors)
	v2doctors.Register(mux, deps.V2Doctors)
	visits.Register(mux, deps.Visits)
	v2visits.Register(mux, deps.Visits, deps.V2Visits)

	return httpapi.WithRequestID(mux)
}
