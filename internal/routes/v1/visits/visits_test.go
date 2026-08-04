package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/smithautotest/clinic-visits/internal/visit"
)

type fakeRepository struct{}

func (fakeRepository) CreateVisit(
	context.Context,
	visit.CreateVisitRequest,
) (visit.Visit, error) {
	return visit.Visit{
		ID:             "44444444-4444-4444-8444-444444444444",
		DoctorID:       "11111111-1111-4111-8111-111111111111",
		PatientID:      "22222222-2222-4222-8222-222222222222",
		ClinicID:       "33333333-3333-4333-8333-333333333333",
		VisitStartTime: time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC),
		VisitEndTime:   time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
	}, nil
}

func (fakeRepository) ListVisits(
	context.Context,
	visit.ListVisitsRequest,
) ([]visit.Visit, error) {
	return []visit.Visit{}, nil
}

func TestRegisterCreateVisit(t *testing.T) {
	mux := http.NewServeMux()
	handler := visit.NewHandler(fakeRepository{})

	Register(mux, handler)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/visits/create",
		strings.NewReader(`{
			"doctor_id": "11111111-1111-4111-8111-111111111111",
			"patient_id": "22222222-2222-4222-8222-222222222222",
			"clinic_id": "33333333-3333-4333-8333-333333333333",
			"visit_start_time": "2026-08-05T09:00:00Z",
			"visit_end_time": "2026-08-05T10:00:00Z"
		}`),
	)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusCreated,
			response.Code,
		)
	}
}

func TestRegisterListVisits(t *testing.T) {
	mux := http.NewServeMux()
	handler := visit.NewHandler(fakeRepository{})

	Register(mux, handler)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/visits/list?page=2&per_page=10",
		nil,
	)
	response := httptest.NewRecorder()

	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}
}
