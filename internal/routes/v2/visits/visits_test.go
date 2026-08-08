package v2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/smithautotest/clinic-visits/internal/visit"
)

type fakeRepository struct {
	listedVisits []visit.Visit
	listErr      error
	listCalled   bool
	listRequest  visit.ListVisitsRequest
}

func (r *fakeRepository) ListVisits(
	_ context.Context,
	request visit.ListVisitsRequest,
) ([]visit.Visit, error) {
	r.listCalled = true
	r.listRequest = request
	return r.listedVisits, r.listErr
}

func TestRegisterVisitsListUsesV1PaginationAndResponseContract(t *testing.T) {
	repository := &fakeRepository{listedVisits: []visit.Visit{{
		ID:             "44444444-4444-4444-8444-444444444444",
		DoctorID:       "11111111-1111-4111-8111-111111111111",
		PatientID:      "22222222-2222-4222-8222-822222222222",
		ClinicID:       "33333333-3333-4333-8333-333333333333",
		Status:         visit.StatusScheduled,
		VisitStartTime: time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC),
		VisitEndTime:   time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
		CreatedAt:      time.Date(2026, time.August, 4, 12, 0, 0, 123456000, time.UTC),
		UpdatedAt:      time.Date(2026, time.August, 4, 13, 0, 0, 654321000, time.UTC),
	}}}
	mux := http.NewServeMux()
	Register(mux, visit.NewListHandler(repository))

	request := httptest.NewRequest(http.MethodPost, "/v2/visits/list?page=2&per_page=10", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("POST /v2/visits/list status = %d, want %d", response.Code, http.StatusOK)
	}
	if !repository.listCalled || repository.listRequest.Pagination.Offset() != 10 || repository.listRequest.Pagination.Limit() != 10 {
		t.Fatalf("ListVisits() request = %#v", repository.listRequest)
	}
	wantBody := `{"data":[{"id":"44444444-4444-4444-8444-444444444444","doctor_id":"11111111-1111-4111-8111-111111111111","patient_id":"22222222-2222-4222-8222-822222222222","clinic_id":"33333333-3333-4333-8333-333333333333","status":"SCHEDULED","visit_start_time":"2026-08-05T09:00:00.000000+00:00","visit_end_time":"2026-08-05T10:00:00.000000+00:00","created_at":"2026-08-04T12:00:00.123456+00:00","updated_at":"2026-08-04T13:00:00.654321+00:00"}]}`
	if response.Body.String() != wantBody {
		t.Fatalf("POST /v2/visits/list body = %s, want %s", response.Body.String(), wantBody)
	}
}

func TestRegisterVisitsListUsesV1Errors(t *testing.T) {
	repository := &fakeRepository{listErr: context.DeadlineExceeded}
	mux := http.NewServeMux()
	Register(mux, visit.NewListHandler(repository))

	request := httptest.NewRequest(http.MethodPost, "/v2/visits/list?page=0", nil)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid pagination response = %d %s", response.Code, response.Body.String())
	}
	assertJSONError(t, response, "BAD_REQUEST", "page must be a positive integer")
	if repository.listCalled {
		t.Fatal("ListVisits called for invalid pagination")
	}

	request = httptest.NewRequest(http.MethodPost, "/v2/visits/list", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError {
		t.Fatalf("repository error response = %d %s", response.Code, response.Body.String())
	}
	assertJSONError(t, response, "INTERNAL_ERROR", "Something went wrong")
}

func TestRegisterVisitsWriteRoutesAreNotRegistered(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, visit.NewListHandler(&fakeRepository{}))

	tests := []struct {
		method string
		path   string
	}{
		{method: http.MethodPost, path: "/v2/visits/create"},
		{method: http.MethodPatch, path: "/v2/visits/update"},
		{method: http.MethodDelete, path: "/v2/visits/delete"},
	}
	for _, test := range tests {
		t.Run(test.method+" "+test.path, func(t *testing.T) {
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, httptest.NewRequest(test.method, test.path, nil))

			if response.Code != http.StatusNotFound {
				t.Fatalf("%s %s status = %d, want %d", test.method, test.path, response.Code, http.StatusNotFound)
			}
			if response.Body.String() != "404 page not found\n" {
				t.Fatalf("%s %s body = %q, want standard net/http 404", test.method, test.path, response.Body.String())
			}
		})
	}
}

func assertJSONError(t *testing.T, response *httptest.ResponseRecorder, code, message string) {
	t.Helper()

	var body struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if body.Code != code || body.Message != message || body.RequestID == "" {
		t.Fatalf("error response = %#v, want code=%q message=%q and a request ID", body, code, message)
	}
}
