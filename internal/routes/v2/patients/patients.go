package v2

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/patient"
)

func Register(mux *http.ServeMux, handler patient.Handler) {
	mux.HandleFunc(
		"/v2/patients/search",
		handler.SearchPatients,
	)
}
