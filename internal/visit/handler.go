package visit

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

func (h Handler) CreateVisit(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	var request CreateVisitRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&request); err != nil {
		httpapi.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		httpapi.WriteJSONError(
			w,
			http.StatusBadRequest,
			"request body must contain only one JSON object",
		)
		return
	}

	if !uuid.IsValid(request.DoctorID) ||
		!uuid.IsValid(request.PatientID) ||
		!uuid.IsValid(request.ClinicID) {
		httpapi.WriteJSONError(w, http.StatusBadRequest, "invalid UUID")
		return
	}

	if request.VisitStartTime.IsZero() ||
		request.VisitEndTime.IsZero() ||
		!request.VisitEndTime.After(request.VisitStartTime) {
		httpapi.WriteJSONError(w, http.StatusBadRequest, ErrInvalidTimeRange.Error())
		return
	}

	createdVisit, err := h.repo.CreateVisit(r.Context(), request)
	if err != nil {
		writeCreateVisitError(w, err)
		return
	}

	httpapi.WriteJSONResponse(w, http.StatusCreated, map[string]any{
		"data": createdVisit,
	})
}

func writeCreateVisitError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrDoctorNotFound),
		errors.Is(err, ErrPatientNotFound),
		errors.Is(err, ErrClinicNotFound):
		httpapi.WriteJSONError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrDoctorClinicMismatch),
		errors.Is(err, ErrInvalidTimeRange):
		httpapi.WriteJSONError(w, http.StatusBadRequest, err.Error())
	default:
		httpapi.WriteJSONError(w, http.StatusInternalServerError, "failed to create visit")
	}
}
