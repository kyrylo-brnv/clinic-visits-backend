package doctor

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"github.com/smithautotest/clinic-visits/internal/httpapi"
)

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) Handler {
	return Handler{repo: repo}
}

func (h Handler) SearchDoctors(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.Header().Set("Allow", http.MethodPost)
		httpapi.WriteJSONError(w, http.StatusMethodNotAllowed, "method not allowed")
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
		httpapi.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err == nil {
		if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
			httpapi.WriteJSONError(
				w,
				http.StatusBadRequest,
				"request body must contain only one JSON object",
			)
			return
		}
	}

	if hasInvalidUUIDFilter(request.Filter) {
		httpapi.WriteJSONError(w, http.StatusBadRequest, "invalid UUID filter")
		return
	}

	doctors, err := h.repo.FindDoctors(r.Context(), *request)
	if err != nil {
		httpapi.WriteJSONError(
			w,
			http.StatusInternalServerError,
			"failed to search doctors",
		)
		return
	}

	httpapi.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"data": doctors,
	})
}
func hasInvalidUUIDFilter(filter *DoctorFilter) bool {
	if filter == nil {
		return false
	}

	return !isValidOptionalUUID(filter.DoctorID) ||
		!isValidOptionalUUID(filter.VisitID) ||
		!isValidOptionalUUID(filter.ClinicID)
}

func isValidOptionalUUID(value string) bool {
	if value == "" {
		return true
	}

	if len(value) != 36 ||
		value[8] != '-' ||
		value[13] != '-' ||
		value[18] != '-' ||
		value[23] != '-' {
		return false
	}

	_, err := hex.DecodeString(strings.ReplaceAll(value, "-", ""))
	return err == nil
}
