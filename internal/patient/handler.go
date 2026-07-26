package patient

import (
	"encoding/json"
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/httpapi"
	"github.com/smithautotest/clinic-visits/internal/sorting"
)

type Handler struct {
	repo Repository
}

var allowedSortingFields = sorting.NewAllowedFields(
	"first_name",
	"last_name",
	"date_of_birth",
	"created_at",
)

func (h Handler) SearchPatients(w http.ResponseWriter, r *http.Request) {
	if !isPostRequest(w, r) {
		return
	}

	var request PatientSearchRequest

	decoder := decodeRequestBody(r)

	if err := decoder.Decode(&request); err != nil {
		http.Error(
			w,
			"invalid request body",
			http.StatusBadRequest)
		return
	}

	if !allowedSortingFields.IsValid(request.Sort) {
		httpapi.WriteJSONResponse(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "Invalid sort field or direction",
			},
		)
		return
	}

	if request.Search.isEmpty() && request.Filter.isEmpty() {
		httpapi.WriteJSONResponse(
			w,
			http.StatusBadRequest,
			map[string]string{
				"error": "At least 1 searchable field is required",
			},
		)
		return
	}

	patients, err := h.repo.FindPatients(r.Context(), request)
	if err != nil {
		http.Error(
			w,
			"failed to search patients",
			http.StatusInternalServerError)
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

func decodeRequestBody(r *http.Request) *json.Decoder {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	return decoder
}

func isPostRequest(w http.ResponseWriter, r *http.Request) bool {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return false
	}
	return true
}
