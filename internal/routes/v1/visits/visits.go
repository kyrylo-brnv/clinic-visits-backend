package v1

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/visit"
)

func Register(mux *http.ServeMux, handler visit.Handler) {
	mux.HandleFunc(
		"POST /v1/visits/create",
		handler.CreateVisit,
	)
}
