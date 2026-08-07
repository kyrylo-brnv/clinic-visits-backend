package patient

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/httpapi"
	"github.com/smithautotest/clinic-visits/internal/pagination"
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

	if err := decoder.Decode(&request); err != nil && !errors.Is(err, io.EOF) {
		httpapi.WriteJSONError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	paginationParams, err := pagination.Parse(r.URL.Query())
	if err != nil {
		httpapi.WriteJSONError(w, r, http.StatusBadRequest, err.Error())
		return
	}
	request.Pagination = paginationParams

	if !allowedSortingFields.IsValid(request.Sort) {
		httpapi.WriteJSONError(w, r, http.StatusBadRequest, "Invalid sort field or direction")
		return
	}

	if (request.Search != nil && request.Search.isEmpty()) ||
		(request.Filter != nil && request.Filter.isEmpty()) {
		httpapi.WriteJSONError(w, r, http.StatusBadRequest, "Provided search or filter must contain at least one criterion")
		return
	}
	patients, err := h.repo.FindPatients(r.Context(), request)
	if err != nil {
		httpapi.WriteInternalError(w, r, err)
		return
	}

	httpapi.WriteJSONResponse(w, r, http.StatusOK, map[string]any{
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
