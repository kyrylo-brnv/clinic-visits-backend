package v2

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/smithautotest/clinic-visits/internal/visit"
)

const validCreateVisitBody = `{
	"doctor_id": "11111111-1111-4111-8111-111111111111",
	"patient_id": "22222222-2222-4222-8222-222222222222",
	"clinic_id": "33333333-3333-4333-8333-333333333333",
	"visit_start_time": "2026-08-05T09:00:00Z",
	"visit_end_time": "2026-08-05T10:00:00Z"
}`

type fakeRepository struct {
	createdVisit visit.Visit
	createErr    error
	createCalled bool
	listedVisits []visit.Visit
	listErr      error
	listCalled   bool
	listRequest  visit.ListVisitsRequest
	deleteErr    error
	deleteCalled bool
	lastDelete   visit.DeleteVisitRequest
	updatedVisit visit.Visit
	updateErr    error
	updateCalled bool
	lastUpdate   visit.UpdateVisitRequest
}

func (r *fakeRepository) CreateVisit(
	_ context.Context,
	_ visit.CreateVisitRequest,
) (visit.Visit, error) {
	r.createCalled = true
	return r.createdVisit, r.createErr
}

func (r *fakeRepository) ListVisits(
	_ context.Context,
	request visit.ListVisitsRequest,
) ([]visit.Visit, error) {
	r.listCalled = true
	r.listRequest = request
	return r.listedVisits, r.listErr
}

func (r *fakeRepository) DeleteVisit(
	_ context.Context,
	request visit.DeleteVisitRequest,
) error {
	r.deleteCalled = true
	r.lastDelete = request
	return r.deleteErr
}

func (r *fakeRepository) UpdateVisit(
	_ context.Context,
	request visit.UpdateVisitRequest,
) (visit.Visit, error) {
	r.updateCalled = true
	r.lastUpdate = request
	return r.updatedVisit, r.updateErr
}

func TestRegisterVisitsCreateMatchesV1Contract(t *testing.T) {
	createdAt := time.Date(2026, time.August, 4, 12, 0, 0, 123456000, time.FixedZone("UTC+3", 3*60*60))
	repository := &fakeRepository{createdVisit: visit.Visit{
		ID:             "44444444-4444-4444-8444-444444444444",
		DoctorID:       "11111111-1111-4111-8111-111111111111",
		PatientID:      "22222222-2222-4222-8222-822222222222",
		ClinicID:       "33333333-3333-4333-8333-333333333333",
		Status:         visit.StatusScheduled,
		VisitStartTime: time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC),
		VisitEndTime:   time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
		CreatedAt:      createdAt,
		UpdatedAt:      createdAt,
	}}
	mux := http.NewServeMux()
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

	request := httptest.NewRequest(http.MethodPost, "/v2/visits/create", strings.NewReader(validCreateVisitBody))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusCreated {
		t.Fatalf("POST /v2/visits/create status = %d, want %d", response.Code, http.StatusCreated)
	}
	if !repository.createCalled {
		t.Fatal("expected CreateVisit to be called")
	}

	wantBody := `{"data":{"id":"44444444-4444-4444-8444-444444444444","doctor_id":"11111111-1111-4111-8111-111111111111","patient_id":"22222222-2222-4222-8222-822222222222","clinic_id":"33333333-3333-4333-8333-333333333333","status":"SCHEDULED","visit_start_time":"2026-08-05T09:00:00.000000+00:00","visit_end_time":"2026-08-05T10:00:00.000000+00:00","created_at":"2026-08-04T12:00:00.123456+03:00","updated_at":"2026-08-04T12:00:00.123456+03:00"}}`
	if response.Body.String() != wantBody {
		t.Fatalf("POST /v2/visits/create body = %s, want %s", response.Body.String(), wantBody)
	}
}

func TestRegisterVisitsCreateUsesV1Validation(t *testing.T) {
	repository := &fakeRepository{}
	mux := http.NewServeMux()
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

	request := httptest.NewRequest(
		http.MethodPost,
		"/v2/visits/create",
		strings.NewReader(`{"doctor_id":"invalid"}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid v2 create request status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	assertJSONError(t, response, "BAD_REQUEST", "invalid UUID")
	if repository.createCalled {
		t.Fatal("CreateVisit called for invalid request")
	}
}

func TestRegisterVisitsCreateUsesV1ErrorResponse(t *testing.T) {
	repository := &fakeRepository{createErr: visit.ErrVisitTimeConflict}
	mux := http.NewServeMux()
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

	request := httptest.NewRequest(
		http.MethodPost,
		"/v2/visits/create",
		strings.NewReader(validCreateVisitBody),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("v2 visit time conflict status = %d, want %d", response.Code, http.StatusConflict)
	}
	assertJSONError(t, response, "CONFLICT", "doctor already has a visit during this time")
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
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

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
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

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

func TestRegisterVisitsDeleteUsesV1Handler(t *testing.T) {
	repository := &fakeRepository{}
	mux := http.NewServeMux()
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

	request := httptest.NewRequest(
		http.MethodDelete,
		"/v2/visits/delete",
		strings.NewReader(`{"visit_id":"44444444-4444-4444-8444-444444444444"}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("DELETE /v2/visits/delete status = %d, want %d", response.Code, http.StatusNoContent)
	}
	if !repository.deleteCalled || repository.lastDelete.VisitID != "44444444-4444-4444-8444-444444444444" {
		t.Fatalf("DeleteVisit() request = %#v", repository.lastDelete)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("DELETE /v2/visits/delete body = %q, want empty", response.Body.String())
	}
}

func TestRegisterVisitsDeleteUsesV1Validation(t *testing.T) {
	tests := []struct {
		name        string
		body        string
		wantMessage string
	}{
		{
			name:        "unknown field",
			body:        `{"visit_id":"44444444-4444-4444-8444-444444444444","unknown":true}`,
			wantMessage: "invalid request body",
		},
		{
			name:        "multiple JSON objects",
			body:        `{"visit_id":"44444444-4444-4444-8444-444444444444"}{}`,
			wantMessage: "request body must contain only one JSON object",
		},
		{
			name:        "invalid UUID",
			body:        `{"visit_id":"invalid"}`,
			wantMessage: "invalid UUID",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repository := &fakeRepository{}
			mux := http.NewServeMux()
			Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

			request := httptest.NewRequest(http.MethodDelete, "/v2/visits/delete", strings.NewReader(test.body))
			response := httptest.NewRecorder()
			mux.ServeHTTP(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("DELETE /v2/visits/delete status = %d, want %d", response.Code, http.StatusBadRequest)
			}
			assertJSONError(t, response, "BAD_REQUEST", test.wantMessage)
			if repository.deleteCalled {
				t.Fatal("DeleteVisit called for invalid request")
			}
		})
	}
}

func TestRegisterVisitsDeleteUsesV1NotFoundResponse(t *testing.T) {
	repository := &fakeRepository{deleteErr: visit.ErrVisitNotFound}
	mux := http.NewServeMux()
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

	request := httptest.NewRequest(
		http.MethodDelete,
		"/v2/visits/delete",
		strings.NewReader(`{"visit_id":"44444444-4444-4444-8444-444444444444"}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusNotFound {
		t.Fatalf("DELETE /v2/visits/delete status = %d, want %d", response.Code, http.StatusNotFound)
	}
	assertJSONError(t, response, "NOT_FOUND", "visit not found")
}

func TestRegisterVisitsUpdateUsesV1Handler(t *testing.T) {
	repository := &fakeRepository{updatedVisit: visit.Visit{
		ID:     "44444444-4444-4444-8444-444444444444",
		Status: visit.StatusInProgress,
	}}
	mux := http.NewServeMux()
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

	request := httptest.NewRequest(
		http.MethodPatch,
		"/v2/visits/update",
		strings.NewReader(`{"visit_id":"44444444-4444-4444-8444-444444444444","status":"IN_PROGRESS"}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("PATCH /v2/visits/update status = %d, want %d", response.Code, http.StatusOK)
	}
	if !repository.updateCalled || repository.lastUpdate.Status == nil || *repository.lastUpdate.Status != visit.StatusInProgress {
		t.Fatalf("UpdateVisit() request = %#v", repository.lastUpdate)
	}
	if !strings.Contains(response.Body.String(), `"status":"IN_PROGRESS"`) {
		t.Fatalf("PATCH /v2/visits/update body = %s", response.Body.String())
	}
}

func TestRegisterVisitsUpdateUsesV1Validation(t *testing.T) {
	repository := &fakeRepository{}
	mux := http.NewServeMux()
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

	request := httptest.NewRequest(
		http.MethodPatch,
		"/v2/visits/update",
		strings.NewReader(`{"visit_id":"44444444-4444-4444-8444-444444444444","status":"UNKNOWN"}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid v2 update request status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	assertJSONError(t, response, "BAD_REQUEST", "invalid visit status")
	if repository.updateCalled {
		t.Fatal("UpdateVisit called for invalid request")
	}
}

func TestRegisterVisitsUpdateUsesV1ErrorResponse(t *testing.T) {
	repository := &fakeRepository{updateErr: fmt.Errorf("%w: SCHEDULED -> CLOSED", visit.ErrInvalidStatusTransition)}
	mux := http.NewServeMux()
	Register(mux, visit.NewHandler(repository), visit.NewListHandler(repository))

	request := httptest.NewRequest(
		http.MethodPatch,
		"/v2/visits/update",
		strings.NewReader(`{"visit_id":"44444444-4444-4444-8444-444444444444","status":"CLOSED"}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid v2 status transition status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	var body struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode invalid v2 status transition response: %v", err)
	}
	if body.Message != "invalid visit status transition: SCHEDULED -> CLOSED" {
		t.Fatalf("invalid v2 status transition body = %q", body.Message)
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
