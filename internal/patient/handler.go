package patient

import (
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/httpapi"
)

type Handler struct {
	repo Repository
}

func (h Handler) ListPatients(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodGet) {
		return
	}

	patients, err := h.repo.List(r.Context())
	if err != nil {
		http.Error(w, "failed to list patients", http.StatusInternalServerError)
		return
	}

	httpapi.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"data": patients,
	})
}

func NewHandler(repo Repository) Handler {
	return Handler{
		repo: repo,
	}
}
