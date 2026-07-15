package patient

import (
	"encoding/json"
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/httpapi"
)

type Handler struct {
	repo Repository
}

func (h Handler) SearchPatients(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, "QUERY") {
		return
	}

	var request PatientSearchRequest

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
	}

	patients, err := h.repo.Search(r.Context(), request)
	if err != nil {
		http.Error(w, "failed to search patients", http.StatusInternalServerError)
		return
	}

	httpapi.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"data": patients})
}

func NewHandler(repo Repository) Handler {
	return Handler{
		repo: repo,
	}
}
