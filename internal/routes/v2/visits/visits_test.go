package v2

import (
	"context"
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

func (*fakeRepository) DeleteVisit(
	context.Context,
	visit.DeleteVisitRequest,
) error {
	return nil
}

func (*fakeRepository) UpdateVisit(
	context.Context,
	visit.UpdateVisitRequest,
) (visit.Visit, error) {
	return visit.Visit{}, nil
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
	if response.Body.String() != `{"error":"invalid UUID"}` {
		t.Fatalf("invalid v2 create request body = %s", response.Body.String())
	}
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
	if response.Body.String() != `{"error":"doctor already has a visit during this time"}` {
		t.Fatalf("v2 visit time conflict body = %s", response.Body.String())
	}
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
	if response.Code != http.StatusBadRequest || response.Body.String() != `{"error":"page must be a positive integer"}` {
		t.Fatalf("invalid pagination response = %d %s", response.Code, response.Body.String())
	}
	if repository.listCalled {
		t.Fatal("ListVisits called for invalid pagination")
	}

	request = httptest.NewRequest(http.MethodPost, "/v2/visits/list", nil)
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusInternalServerError || response.Body.String() != `{"error":"failed to list visits"}` {
		t.Fatalf("repository error response = %d %s", response.Code, response.Body.String())
	}
}
