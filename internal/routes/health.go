package routes

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/httpapi"
)

type healthResponse struct {
	Status string `json:"status"`
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		httpapi.WriteJSONError(w, r, http.StatusMethodNotAllowed, "Method Not Allowed")
		return
	}

	httpapi.WriteJSONResponse(w, r, http.StatusOK, healthResponse{Status: "ok"})
}
