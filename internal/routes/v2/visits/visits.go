package v2

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/visit"
)

func Register(mux *http.ServeMux, listHandler visit.ListHandler) {
	mux.HandleFunc(
		"/v2/visits/list",
		listHandler.ListVisits,
	)
}
