package routes

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/patient"
	patients "github.com/smithautotest/clinic-visits/internal/routes/v1/patients"
)

type Dependencies struct {
	Patients patient.Handler
}

func NewRouter(deps Dependencies) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", healthHandler)
	patients.Register(mux, deps.Patients)

	return mux
}
