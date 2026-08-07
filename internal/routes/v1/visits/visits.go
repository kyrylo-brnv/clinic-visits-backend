package v1

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/visit"
)

func Register(mux *http.ServeMux, handler visit.Handler) {
	mux.HandleFunc(
		"/v1/visits/create",
		handler.CreateVisit,
	)
	mux.HandleFunc(
		"/v1/visits/list",
		handler.ListVisits,
	)
	mux.HandleFunc(
		"/v1/visits/delete",
		handler.DeleteVisit,
	)
	mux.HandleFunc(
		"/v1/visits/update",
		handler.UpdateVisit,
	)
}
