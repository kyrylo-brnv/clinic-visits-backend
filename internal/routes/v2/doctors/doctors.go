package v2

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/doctor"
)

func Register(mux *http.ServeMux, handler doctor.Handler) {
	mux.HandleFunc(
		"POST /v2/doctors/search",
		handler.SearchDoctors,
	)
}
