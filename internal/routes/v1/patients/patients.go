package v1

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/patient"
)

func Register(mux *http.ServeMux, handler patient.Handler) {
	mux.HandleFunc(
		"/v1/patients/search",
		handler.SearchPatients,
	)
}
