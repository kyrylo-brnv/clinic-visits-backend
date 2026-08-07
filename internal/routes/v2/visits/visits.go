package v2

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/visit"
)

func Register(mux *http.ServeMux, handler visit.Handler, listHandler visit.ListHandler) {
	mux.HandleFunc(
		"POST /v2/visits/create",
		handler.CreateVisit,
	)
	mux.HandleFunc(
		"POST /v2/visits/list",
		listHandler.ListVisits,
	)
	mux.HandleFunc(
		"DELETE /v2/visits/delete",
		handler.DeleteVisit,
	)
}
