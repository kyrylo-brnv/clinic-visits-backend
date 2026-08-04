package doctor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRepository struct {
	doctors   []Doctor
	err       error
	request   DoctorSearchRequest
	wasCalled bool
}

func (r *fakeRepository) FindDoctors(
	_ context.Context,
	request DoctorSearchRequest,
) ([]Doctor, error) {
	r.wasCalled = true
	r.request = request

	return r.doctors, r.err
}

func TestSearchDoctorsReturnsDoctorsAndForwardsFilter(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{
		doctors: []Doctor{
			{
				ID:          "doctor-1",
				SpecialtyID: "specialty-1",
				ClinicID:    "clinic-1",
				FullName:    "Jane Doe",
			},
		},
	}
	handler := NewHandler(repo)
	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/doctors/search",
		strings.NewReader(`{
			"filter": {
				"doctor_id": "11111111-1111-1111-1111-111111111111",
				"visit_id": "22222222-2222-2222-2222-222222222222",
				"clinic_id": "33333333-3333-3333-3333-333333333333"
			}
		}`),
	)
	response := httptest.NewRecorder()

	handler.SearchDoctors(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	if repo.request.Filter == nil {
		t.Fatal("expected filter to be forwarded to repository")
	}

	if repo.request.Filter.DoctorID != "11111111-1111-1111-1111-111111111111" ||
		repo.request.Filter.VisitID != "22222222-2222-2222-2222-222222222222" ||
		repo.request.Filter.ClinicID != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("unexpected forwarded filter: %+v", *repo.request.Filter)
	}

	expectedBody := `{"data":[{"id":"doctor-1","specialty_id":"specialty-1","clinic_id":"clinic-1","full_name":"Jane Doe"}]}`
	if response.Body.String() != expectedBody {
		t.Fatalf("expected body %s, got %s", expectedBody, response.Body.String())
	}
}

func TestSearchDoctorsAllowsEmptyBody(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	handler := NewHandler(repo)
	request := httptest.NewRequest(http.MethodPost, "/v1/doctors/search", nil)
	response := httptest.NewRecorder()

	handler.SearchDoctors(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	if !repo.wasCalled {
		t.Fatal("expected repository to be called")
	}
}

func TestSearchDoctorsRejectsInvalidBody(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		body          string
		expectedError string
	}{
		{
			name:          "invalid JSON",
			body:          `invalid json`,
			expectedError: "invalid request body",
		},
		{
			name:          "unknown field",
			body:          `{"unknown": true}`,
			expectedError: "invalid request body",
		},
		{
			name:          "multiple JSON values",
			body:          `{} {}`,
			expectedError: "request body must contain only one JSON object",
		},
		{
			name:          "null",
			body:          `null`,
			expectedError: "invalid request body",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeRepository{}
			handler := NewHandler(repo)
			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/doctors/search",
				strings.NewReader(test.body),
			)
			response := httptest.NewRecorder()

			handler.SearchDoctors(response, request)

			assertJSONError(t, response, http.StatusBadRequest, test.expectedError)

			if repo.wasCalled {
				t.Fatal("expected repository not to be called")
			}
		})
	}
}

func TestSearchDoctorsRejectsInvalidUUIDFilters(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		field string
	}{
		{name: "doctor ID", field: "doctor_id"},
		{name: "visit ID", field: "visit_id"},
		{name: "clinic ID", field: "clinic_id"},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := &fakeRepository{}
			handler := NewHandler(repo)
			body := `{"filter":{"` + test.field + `":"abc"}}`
			request := httptest.NewRequest(
				http.MethodPost,
				"/v1/doctors/search",
				strings.NewReader(body),
			)
			response := httptest.NewRecorder()

			handler.SearchDoctors(response, request)

			assertJSONError(t, response, http.StatusBadRequest, "invalid UUID filter")

			if repo.wasCalled {
				t.Fatal("expected repository not to be called")
			}
		})
	}
}

func TestSearchDoctorsRejectsWrongMethod(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{}
	handler := NewHandler(repo)
	request := httptest.NewRequest(http.MethodGet, "/v1/doctors/search", nil)
	response := httptest.NewRecorder()

	handler.SearchDoctors(response, request)

	assertJSONError(t, response, http.StatusMethodNotAllowed, "method not allowed")

	if response.Header().Get("Allow") != http.MethodPost {
		t.Fatalf("expected Allow header %q", http.MethodPost)
	}

	if repo.wasCalled {
		t.Fatal("expected repository not to be called")
	}
}

func TestSearchDoctorsHandlesRepositoryError(t *testing.T) {
	t.Parallel()

	repo := &fakeRepository{err: errors.New("database unavailable")}
	handler := NewHandler(repo)
	request := httptest.NewRequest(http.MethodPost, "/v1/doctors/search", nil)
	response := httptest.NewRecorder()

	handler.SearchDoctors(response, request)

	assertJSONError(
		t,
		response,
		http.StatusInternalServerError,
		"failed to search doctors",
	)
}

func assertJSONError(
	t *testing.T,
	response *httptest.ResponseRecorder,
	statusCode int,
	message string,
) {
	t.Helper()

	if response.Code != statusCode {
		t.Fatalf("expected status %d, got %d", statusCode, response.Code)
	}

	if response.Header().Get("Content-Type") != "application/json" {
		t.Fatalf(
			"expected Content-Type %q, got %q",
			"application/json",
			response.Header().Get("Content-Type"),
		)
	}

	expectedBody := `{"error":"` + message + `"}`
	if response.Body.String() != expectedBody {
		t.Fatalf("expected body %s, got %s", expectedBody, response.Body.String())
	}
}
