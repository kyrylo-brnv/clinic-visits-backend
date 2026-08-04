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

const validDeleteVisitBody = `{
	"visit_id": "44444444-4444-4444-8444-444444444444"
}`

const validUpdateVisitBody = `{
	"visit_id": "44444444-4444-4444-8444-444444444444",
	"visit_end_time": "2026-08-05T11:00:00Z"
}`

type fakeRepository struct {
	createdVisit    Visit
	err             error
	lastRequest     CreateVisitRequest
	called          bool
	listedVisits    []Visit
	listErr         error
	lastListRequest ListVisitsRequest
	listCalled      bool
	deleteErr       error
	lastDelete      DeleteVisitRequest
	deleteCalled    bool
	updatedVisit    Visit
	updateErr       error
	lastUpdate      UpdateVisitRequest
	updateCalled    bool
}

func (r *fakeRepository) CreateVisit(
	_ context.Context,
	request CreateVisitRequest,
) (Visit, error) {
	r.called = true
	r.lastRequest = request
	return r.createdVisit, r.err
}

func (r *fakeRepository) ListVisits(
	_ context.Context,
	request ListVisitsRequest,
) ([]Visit, error) {
	r.listCalled = true
	r.lastListRequest = request
	return r.listedVisits, r.listErr
}

func (r *fakeRepository) DeleteVisit(
	_ context.Context,
	request DeleteVisitRequest,
) error {
	r.deleteCalled = true
	r.lastDelete = request
	return r.deleteErr
}

func (r *fakeRepository) UpdateVisit(
	_ context.Context,
	request UpdateVisitRequest,
) (Visit, error) {
	r.updateCalled = true
	r.lastUpdate = request
	return r.updatedVisit, r.updateErr
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
		"/v1/visits/create",
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
				"/v1/visits/create",
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
		{name: "visit time conflict", err: ErrVisitTimeConflict, status: http.StatusConflict},
		{name: "patient time conflict", err: ErrPatientTimeConflict, status: http.StatusConflict},
		{name: "database error", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{err: test.err}
			handler := NewHandler(repo)
			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/visits/create",
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
	request := httptest.NewRequest(http.MethodGet, "/v1/visits/create", nil)
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

func TestListVisitsReturnsVisitsAndForwardsPagination(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{listedVisits: []Visit{{
		ID:             "44444444-4444-4444-8444-444444444444",
		DoctorID:       "11111111-1111-4111-8111-111111111111",
		PatientID:      "22222222-2222-4222-8222-222222222222",
		ClinicID:       "33333333-3333-4333-8333-333333333333",
		VisitStartTime: time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC),
		VisitEndTime:   time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC),
	}}}
	handler := NewHandler(repo)
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/visits/list?page=3&per_page=50",
		nil,
	)
	response := httptest.NewRecorder()

	handler.ListVisits(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if !repo.listCalled {
		t.Fatal("expected repository to be called")
	}
	if repo.lastListRequest.Pagination.Limit() != 50 {
		t.Fatalf(
			"expected limit %d, got %d",
			50,
			repo.lastListRequest.Pagination.Limit(),
		)
	}
	if repo.lastListRequest.Pagination.Offset() != 100 {
		t.Fatalf(
			"expected offset %d, got %d",
			100,
			repo.lastListRequest.Pagination.Offset(),
		)
	}

	var body struct {
		Data []Visit `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(body.Data) != 1 || body.Data[0].ID != repo.listedVisits[0].ID {
		t.Fatalf("expected listed visit %q, got %+v", repo.listedVisits[0].ID, body.Data)
	}
}

func TestListVisitsUsesPaginationDefaults(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{listedVisits: []Visit{}}
	handler := NewHandler(repo)
	request := httptest.NewRequest(http.MethodPost, "/v1/visits/list", nil)
	response := httptest.NewRecorder()

	handler.ListVisits(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if repo.lastListRequest.Pagination.Page() != 1 {
		t.Fatalf(
			"expected default page %d, got %d",
			1,
			repo.lastListRequest.Pagination.Page(),
		)
	}
	if repo.lastListRequest.Pagination.PerPage() != 20 {
		t.Fatalf(
			"expected default per_page %d, got %d",
			20,
			repo.lastListRequest.Pagination.PerPage(),
		)
	}
}

func TestListVisitsAcceptsMaxPageSize(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{listedVisits: []Visit{}}
	handler := NewHandler(repo)
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/visits/list?per_page=200",
		nil,
	)
	response := httptest.NewRecorder()

	handler.ListVisits(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if repo.lastListRequest.Pagination.PerPage() != 200 {
		t.Fatalf(
			"expected per_page %d, got %d",
			200,
			repo.lastListRequest.Pagination.PerPage(),
		)
	}
}

func TestListVisitsRejectsInvalidPagination(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		query string
	}{
		{name: "zero page", query: "page=0"},
		{name: "non-numeric page", query: "page=abc"},
		{name: "page size above maximum", query: "per_page=201"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			handler := NewHandler(repo)
			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/visits/list?"+test.query,
				nil,
			)
			response := httptest.NewRecorder()

			handler.ListVisits(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusBadRequest,
					response.Code,
				)
			}
			if repo.listCalled {
				t.Fatal("expected repository not to be called")
			}
		})
	}
}

func TestListVisitsHandlesRepositoryError(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{listErr: errors.New("database unavailable")}
	handler := NewHandler(repo)
	request := httptest.NewRequest(http.MethodPost, "/v1/visits/list", nil)
	response := httptest.NewRecorder()

	handler.ListVisits(response, request)

	if response.Code != http.StatusInternalServerError {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusInternalServerError,
			response.Code,
		)
	}
}

func TestListVisitsRejectsWrongMethod(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	handler := NewHandler(repo)
	request := httptest.NewRequest(http.MethodGet, "/v1/visits/list", nil)
	response := httptest.NewRecorder()

	handler.ListVisits(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusMethodNotAllowed,
			response.Code,
		)
	}
	if repo.listCalled {
		t.Fatal("expected repository not to be called")
	}
}

func TestDeleteVisitReturnsNoContent(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	handler := NewHandler(repo)
	request := httptest.NewRequest(
		http.MethodDelete,
		"/v1/visits/delete",
		strings.NewReader(validDeleteVisitBody),
	)
	response := httptest.NewRecorder()

	handler.DeleteVisit(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
	if !repo.deleteCalled || repo.lastDelete.VisitID != "44444444-4444-4444-8444-444444444444" {
		t.Fatalf("expected delete request to be forwarded, got %+v", repo.lastDelete)
	}
	if response.Body.Len() != 0 {
		t.Fatalf("expected an empty response body, got %q", response.Body.String())
	}
}

func TestDeleteVisitRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{`},
		{name: "unknown field", body: `{"visit_id":"44444444-4444-4444-8444-444444444444","unknown":true}`},
		{name: "multiple objects", body: validDeleteVisitBody + `{}`},
		{name: "missing visit ID", body: `{}`},
		{name: "invalid visit ID", body: `{"visit_id":"invalid"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			handler := NewHandler(repo)
			request := httptest.NewRequest(http.MethodDelete, "/v1/visits/delete", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			handler.DeleteVisit(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
			}
			if repo.deleteCalled {
				t.Fatal("expected repository not to be called")
			}
		})
	}
}

func TestDeleteVisitMapsRepositoryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "visit not found", err: ErrVisitNotFound, status: http.StatusNotFound},
		{name: "database error", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{deleteErr: test.err}
			handler := NewHandler(repo)
			request := httptest.NewRequest(http.MethodDelete, "/v1/visits/delete", strings.NewReader(validDeleteVisitBody))
			response := httptest.NewRecorder()

			handler.DeleteVisit(response, request)

			if response.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, response.Code)
			}
		})
	}
}

func TestUpdateVisitReturnsUpdatedVisitAndPreservesOmittedFields(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{updatedVisit: Visit{
		ID:           "44444444-4444-4444-8444-444444444444",
		DoctorID:     "11111111-1111-4111-8111-111111111111",
		PatientID:    "22222222-2222-4222-8222-222222222222",
		ClinicID:     "33333333-3333-4333-8333-333333333333",
		VisitEndTime: time.Date(2026, time.August, 5, 11, 0, 0, 0, time.UTC),
	}}
	handler := NewHandler(repo)
	request := httptest.NewRequest(http.MethodPatch, "/v1/visits/update", strings.NewReader(validUpdateVisitBody))
	response := httptest.NewRecorder()

	handler.UpdateVisit(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if !repo.updateCalled {
		t.Fatal("expected repository to be called")
	}
	if repo.lastUpdate.VisitEndTime == nil || !repo.lastUpdate.VisitEndTime.Equal(repo.updatedVisit.VisitEndTime) {
		t.Fatalf("expected visit end time to be forwarded, got %+v", repo.lastUpdate.VisitEndTime)
	}
	if repo.lastUpdate.DoctorID != nil || repo.lastUpdate.PatientID != nil || repo.lastUpdate.ClinicID != nil || repo.lastUpdate.VisitStartTime != nil {
		t.Fatalf("expected omitted fields to stay nil, got %+v", repo.lastUpdate)
	}

	var body struct {
		Data Visit `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Data.ID != repo.updatedVisit.ID {
		t.Fatalf("expected visit ID %q, got %q", repo.updatedVisit.ID, body.Data.ID)
	}
}

func TestUpdateVisitRejectsInvalidRequest(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		body string
	}{
		{name: "invalid JSON", body: `{`},
		{name: "unknown field", body: `{"visit_id":"44444444-4444-4444-8444-444444444444","unknown":true}`},
		{name: "multiple objects", body: validUpdateVisitBody + `{}`},
		{name: "missing visit ID", body: `{"doctor_id":"11111111-1111-4111-8111-111111111111"}`},
		{name: "invalid visit ID", body: `{"visit_id":"invalid","doctor_id":"11111111-1111-4111-8111-111111111111"}`},
		{name: "invalid related ID", body: `{"visit_id":"44444444-4444-4444-8444-444444444444","doctor_id":"invalid"}`},
		{name: "null mutable field", body: `{"visit_id":"44444444-4444-4444-8444-444444444444","doctor_id":null,"patient_id":"22222222-2222-4222-8222-222222222222"}`},
		{name: "no changes", body: `{"visit_id":"44444444-4444-4444-8444-444444444444"}`},
		{name: "invalid time range", body: `{"visit_id":"44444444-4444-4444-8444-444444444444","visit_start_time":"2026-08-05T11:00:00Z","visit_end_time":"2026-08-05T10:00:00Z"}`},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{}
			handler := NewHandler(repo)
			request := httptest.NewRequest(http.MethodPatch, "/v1/visits/update", strings.NewReader(test.body))
			response := httptest.NewRecorder()

			handler.UpdateVisit(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf("expected status %d, got %d", http.StatusBadRequest, response.Code)
			}
			if repo.updateCalled {
				t.Fatal("expected repository not to be called")
			}
		})
	}
}

func TestUpdateVisitMapsRepositoryErrors(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		err    error
		status int
	}{
		{name: "visit not found", err: ErrVisitNotFound, status: http.StatusNotFound},
		{name: "doctor not found", err: ErrDoctorNotFound, status: http.StatusNotFound},
		{name: "patient not found", err: ErrPatientNotFound, status: http.StatusNotFound},
		{name: "clinic not found", err: ErrClinicNotFound, status: http.StatusNotFound},
		{name: "doctor clinic mismatch", err: ErrDoctorClinicMismatch, status: http.StatusBadRequest},
		{name: "invalid final time range", err: ErrInvalidTimeRange, status: http.StatusBadRequest},
		{name: "visit time conflict", err: ErrVisitTimeConflict, status: http.StatusConflict},
		{name: "patient time conflict", err: ErrPatientTimeConflict, status: http.StatusConflict},
		{name: "database error", err: errors.New("database unavailable"), status: http.StatusInternalServerError},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{updateErr: test.err}
			handler := NewHandler(repo)
			request := httptest.NewRequest(http.MethodPatch, "/v1/visits/update", strings.NewReader(validUpdateVisitBody))
			response := httptest.NewRecorder()

			handler.UpdateVisit(response, request)

			if response.Code != test.status {
				t.Fatalf("expected status %d, got %d", test.status, response.Code)
			}
		})
	}
}

func TestPatientTimeConflictUsesSpecificMessage(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{
			name:   "create",
			method: http.MethodPost,
			path:   "/v1/visits/create",
			body:   validCreateVisitBody,
		},
		{
			name:   "update",
			method: http.MethodPatch,
			path:   "/v1/visits/update",
			body:   validUpdateVisitBody,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			repo := &fakeRepository{
				err:       ErrPatientTimeConflict,
				updateErr: ErrPatientTimeConflict,
			}
			handler := NewHandler(repo)
			request := httptest.NewRequest(test.method, test.path, strings.NewReader(test.body))
			response := httptest.NewRecorder()

			if test.method == http.MethodPost {
				handler.CreateVisit(response, request)
			} else {
				handler.UpdateVisit(response, request)
			}

			if response.Code != http.StatusConflict {
				t.Fatalf("expected status %d, got %d", http.StatusConflict, response.Code)
			}
			var responseBody struct {
				Error string `json:"error"`
			}
			if err := json.NewDecoder(response.Body).Decode(&responseBody); err != nil {
				t.Fatalf("decode response: %v", err)
			}
			if responseBody.Error != ErrPatientTimeConflict.Error() {
				t.Fatalf("expected message %q, got %q", ErrPatientTimeConflict, responseBody.Error)
			}
		})
	}
}
