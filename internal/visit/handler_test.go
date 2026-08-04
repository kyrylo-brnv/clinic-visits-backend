package visit

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const validCreateVisitBody = `{
	"doctor_id": "11111111-1111-4111-8111-111111111111",
	"patient_id": "22222222-2222-4222-8222-222222222222",
	"clinic_id": "33333333-3333-4333-8333-333333333333",
	"visit_start_time": "2026-08-05T09:00:00Z",
	"visit_end_time": "2026-08-05T10:00:00Z"
}`

type fakeRepository struct {
	createdVisit Visit
	err          error
	lastRequest  CreateVisitRequest
	called       bool
}

func (r *fakeRepository) CreateVisit(
	_ context.Context,
	request CreateVisitRequest,
) (Visit, error) {
	r.called = true
	r.lastRequest = request
	return r.createdVisit, r.err
}

func TestCreateVisitReturnsCreatedVisit(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.August, 4, 12, 0, 0, 0, time.UTC)
	repo := &fakeRepository{createdVisit: Visit{
		ID:             "44444444-4444-4444-8444-444444444444",
		DoctorID:       "11111111-1111-4111-8111-111111111111",
		PatientID:      "22222222-2222-4222-8222-222222222222",
		ClinicID:       "33333333-3333-4333-8333-333333333333",
		VisitStartTime: time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC),
		VisitEndTime:   time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}}
	handler := NewHandler(repo)
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/visits",
		strings.NewReader(validCreateVisitBody),
	)
	response := httptest.NewRecorder()

	handler.CreateVisit(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, response.Code)
	}
	if !repo.called {
		t.Fatal("expected repository to be called")
	}
	if repo.lastRequest.DoctorID != repo.createdVisit.DoctorID {
		t.Fatalf(
			"expected doctor ID %q, got %q",
			repo.createdVisit.DoctorID,
			repo.lastRequest.DoctorID,
		)
	}

	var body struct {
		Data Visit `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.ID != repo.createdVisit.ID {
		t.Fatalf("expected visit ID %q, got %q", repo.createdVisit.ID, body.Data.ID)
	}
}

func TestCreateVisitRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{`},
		{name: "unknown field", body: `{"unknown": true}`},
		{name: "multiple objects", body: validCreateVisitBody + `{}`},
		{name: "invalid UUID", body: strings.Replace(validCreateVisitBody, "11111111-1111-4111-8111-111111111111", "invalid", 1)},
		{name: "invalid time range", body: strings.Replace(validCreateVisitBody, "2026-08-05T10:00:00Z", "2026-08-05T08:00:00Z", 1)},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			handler := NewHandler(repo)
			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/visits",
				strings.NewReader(test.body),
			)
			response := httptest.NewRecorder()

			handler.CreateVisit(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusBadRequest,
					response.Code,
				)
			}
			if repo.called {
				t.Fatal("expected repository not to be called")
			}
		})
	}
}

func TestCreateVisitMapsRepositoryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "doctor not found", err: ErrDoctorNotFound, status: http.StatusNotFound},
		{name: "patient not found", err: ErrPatientNotFound, status: http.StatusNotFound},
		{name: "clinic not found", err: ErrClinicNotFound, status: http.StatusNotFound},
		{name: "doctor clinic mismatch", err: ErrDoctorClinicMismatch, status: http.StatusBadRequest},
		{name: "invalid time range", err: ErrInvalidTimeRange, status: http.StatusBadRequest},
		{name: "database error", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{err: test.err}
			handler := NewHandler(repo)
			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/visits",
				strings.NewReader(validCreateVisitBody),
			)
			response := httptest.NewRecorder()

			handler.CreateVisit(response, request)

			if response.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, response.Code)
			}
		})
	}
}

func TestCreateVisitRejectsWrongMethod(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	handler := NewHandler(repo)
	request := httptest.NewRequest(http.MethodGet, "/v1/visits", nil)
	response := httptest.NewRecorder()

	handler.CreateVisit(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			response.Code,
		)
	}
	if repo.called {
		t.Fatal("expected repository not to be called")
	}
}
