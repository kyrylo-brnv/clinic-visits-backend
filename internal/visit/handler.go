package visit

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"github.com/smithautotest/clinic-visits/internal/httpapi"
	"github.com/smithautotest/clinic-visits/internal/pagination"
	"github.com/smithautotest/clinic-visits/internal/uuid"
)

type Handler struct {
	repo Repository
}

type updateVisitBody struct {
	VisitID        string                   `json:"visit_id"`
	DoctorID       optionalField[string]    `json:"doctor_id"`
	PatientID      optionalField[string]    `json:"patient_id"`
	ClinicID       optionalField[string]    `json:"clinic_id"`
	VisitStartTime optionalField[time.Time] `json:"visit_start_time"`
	VisitEndTime   optionalField[time.Time] `json:"visit_end_time"`
}

type optionalField[T any] struct {
	value T
	set   bool
}

func (f *optionalField[T]) UnmarshalJSON(data []byte) error {
	if bytes.Equal(data, []byte("null")) {
		return errors.New("field cannot be null")
	}

	if err := json.Unmarshal(data, &f.value); err != nil {
		return err
	}

	f.set = true
	return nil
}

func (f optionalField[T]) pointer() *T {
	if !f.set {
		return nil
	}

	return &f.value
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

func (h Handler) ListVisits(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPost) {
		return
	}

	paginationParams, err := pagination.Parse(r.URL.Query())
	if err != nil {
		httpapi.WriteJSONError(w, http.StatusBadRequest, err.Error())
		return
	}

	visits, err := h.repo.ListVisits(r.Context(), ListVisitsRequest{
		Pagination: paginationParams,
	})
	if err != nil {
		httpapi.WriteJSONError(w, http.StatusInternalServerError, "failed to list visits")
		return
	}

	httpapi.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"data": visits,
	})
}

func (h Handler) DeleteVisit(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodDelete) {
		return
	}

	var request DeleteVisitRequest
	if !decodeStrictRequest(w, r, &request) {
		return
	}

	if !uuid.IsValid(request.VisitID) {
		httpapi.WriteJSONError(w, http.StatusBadRequest, "invalid UUID")
		return
	}

	if err := h.repo.DeleteVisit(r.Context(), request); err != nil {
		if errors.Is(err, ErrVisitNotFound) {
			httpapi.WriteJSONError(w, http.StatusNotFound, err.Error())
			return
		}

		httpapi.WriteJSONError(w, http.StatusInternalServerError, "failed to delete visit")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func (h Handler) UpdateVisit(w http.ResponseWriter, r *http.Request) {
	if !httpapi.RequireMethod(w, r, http.MethodPatch) {
		return
	}

	var body updateVisitBody
	if !decodeStrictRequest(w, r, &body) {
		return
	}
	request := UpdateVisitRequest{
		VisitID:        body.VisitID,
		DoctorID:       body.DoctorID.pointer(),
		PatientID:      body.PatientID.pointer(),
		ClinicID:       body.ClinicID.pointer(),
		VisitStartTime: body.VisitStartTime.pointer(),
		VisitEndTime:   body.VisitEndTime.pointer(),
	}

	if !uuid.IsValid(request.VisitID) ||
		(request.DoctorID != nil && !uuid.IsValid(*request.DoctorID)) ||
		(request.PatientID != nil && !uuid.IsValid(*request.PatientID)) ||
		(request.ClinicID != nil && !uuid.IsValid(*request.ClinicID)) {
		httpapi.WriteJSONError(w, http.StatusBadRequest, "invalid UUID")
		return
	}

	if !request.HasChanges() {
		httpapi.WriteJSONError(w, http.StatusBadRequest, "at least one field must be updated")
		return
	}

	if (request.VisitStartTime != nil && request.VisitStartTime.IsZero()) ||
		(request.VisitEndTime != nil && request.VisitEndTime.IsZero()) ||
		(request.VisitStartTime != nil && request.VisitEndTime != nil &&
			!request.VisitEndTime.After(*request.VisitStartTime)) {
		httpapi.WriteJSONError(w, http.StatusBadRequest, ErrInvalidTimeRange.Error())
		return
	}

	updatedVisit, err := h.repo.UpdateVisit(r.Context(), request)
	if err != nil {
		writeUpdateVisitError(w, err)
		return
	}

	httpapi.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"data": updatedVisit,
	})
}

func decodeStrictRequest(w http.ResponseWriter, r *http.Request, destination any) bool {
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(destination); err != nil {
		httpapi.WriteJSONError(w, http.StatusBadRequest, "invalid request body")
		return false
	}

	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		httpapi.WriteJSONError(
			w,
			http.StatusBadRequest,
			"request body must contain only one JSON object",
		)
		return false
	}

	return true
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
	case errors.Is(err, ErrVisitTimeConflict),
		errors.Is(err, ErrPatientTimeConflict):
		httpapi.WriteJSONError(w, http.StatusConflict, err.Error())
	default:
		httpapi.WriteJSONError(w, http.StatusInternalServerError, "failed to create visit")
	}
}

func writeUpdateVisitError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrVisitNotFound),
		errors.Is(err, ErrDoctorNotFound),
		errors.Is(err, ErrPatientNotFound),
		errors.Is(err, ErrClinicNotFound):
		httpapi.WriteJSONError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrDoctorClinicMismatch),
		errors.Is(err, ErrInvalidTimeRange):
		httpapi.WriteJSONError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, ErrVisitTimeConflict),
		errors.Is(err, ErrPatientTimeConflict):
		httpapi.WriteJSONError(w, http.StatusConflict, err.Error())
	default:
		httpapi.WriteJSONError(w, http.StatusInternalServerError, "failed to update visit")
	}
}
