package doctor

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/smithautotest/clinic-visits/internal/httpapi"
	"github.com/smithautotest/clinic-visits/internal/uuid"
)

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) Handler {
	return Handler{repo: repo}
}

func (h Handler) SearchDoctors(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var request *DoctorSearchRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	err := decoder.Decode(&request)
	switch {
	case errors.Is(err, io.EOF):
		request = &DoctorSearchRequest{}
	case err != nil || request == nil:
		httpapi.WriteJSONError(w, r, http.StatusBadRequest, "invalid request body")
		return
	}

	if err == nil {
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			httpapi.WriteJSONError(
				w,
				r,
				http.StatusBadRequest,
				"request body must contain only one JSON object",
			)
			return
		}
	}

	if hasInvalidUUIDFilter(request.Filter) {
		httpapi.WriteJSONError(w, r, http.StatusBadRequest, "invalid UUID filter")
		return
	}

	doctors, err := h.repo.FindDoctors(r.Context(), *request)
	if err != nil {
		httpapi.WriteInternalError(w, r, err)
		return
	}

	httpapi.WriteJSONResponse(w, r, http.StatusOK, map[string]any{
		"data": doctors,
	})
}
func hasInvalidUUIDFilter(filter *DoctorFilter) bool {
	if filter == nil {
		return false
	}

	return !uuid.IsValidOptional(filter.DoctorID) ||
		!uuid.IsValidOptional(filter.VisitID) ||
		!uuid.IsValidOptional(filter.ClinicID)
}
