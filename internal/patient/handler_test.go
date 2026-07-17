package patient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type FakeRepository struct {
	Patients    []Patient
	LastSearch  PatientSearchRequest
	LastFilter  PatientFilter
	FilterError error
}

func (r *FakeRepository) Filter(
	ctx context.Context,
	request PatientFilter,
) ([]Patient, error) {
	r.LastFilter = request

	if r.FilterError != nil {
		return nil, r.FilterError
	}

	return r.Patients, nil
}

func NewFakeRepository() *FakeRepository {
	return &FakeRepository{
		Patients: []Patient{
			{
				ID:          "abc1234-5678-90ab-cdef-1234567890ab",
				FirstName:   "John",
				LastName:    "Doe",
				DateOfBirth: "1990-01-01",
				Gender:      "Male",
				IsDeleted:   false,
			},
		},
	}
}

func (r *FakeRepository) Search(ctx context.Context, request PatientSearchRequest) ([]Patient, error) {
	r.LastSearch = request
	return r.Patients, nil
}

func TestSearchPatientsValidResponse(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodPost,
		"/patients",
		strings.NewReader(`
			{
				"search": {
					"first_name": "ann",
					"last_name": ""
				}
			}
		`))
	response := httptest.NewRecorder()

	handler.SearchPatients(response, request)

	if repo.LastSearch.Search.FirstName != "ann" {
		t.Fatalf(
			"expected first name search %q, got %q",
			"ann",
			repo.LastSearch.Search.FirstName,
		)
	}

	if repo.LastSearch.Search.LastName != "" {
		t.Fatalf(
			"expected empty last name search, got %q",
			repo.LastSearch.Search.LastName,
		)
	}

	if response.Code != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, response.Code)
	}
}

func TestSearchPatientsEmptySearch(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodPost,
		"/patients",
		strings.NewReader(`{
                        "search": {
                                "first_name": "",
                                "last_name": ""
                        }
                }`),
	)
	response := httptest.NewRecorder()

	handler.SearchPatients(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status code %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}
}

func TestSearchPatientsInvalidJSON(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodPost,
		"/patients",
		strings.NewReader(`invalid json`),
	)
	response := httptest.NewRecorder()

	handler.SearchPatients(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf(
			"expected status code %d, got %d",
			http.StatusBadRequest,
			response.Code,
		)
	}
}

func TestSearchPatientsWrongMethod(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodGet,
		"/patients",
		nil,
	)
	response := httptest.NewRecorder()

	handler.SearchPatients(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf(
			"expected status code %d, got %d",
			http.StatusMethodNotAllowed,
			response.Code,
		)
	}
}

func TestFilterPatientsValidResponse(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodPost,
		"/patients/filter",
		strings.NewReader(`{"id":"abc123"}`),
	)
	response := httptest.NewRecorder()

	handler.FilterPatients(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d",
			http.StatusOK, response.Code)
	}

	if repo.LastFilter.Id != "abc123" {
		t.Fatalf("expected id %q, got %q",
			"abc123", repo.LastFilter.Id)
	}
}

func TestFilterPatientsEmptyID(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodPost,
		"/patients/filter",
		strings.NewReader(`{"id":""}`),
	)
	response := httptest.NewRecorder()

	handler.FilterPatients(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d",
			http.StatusBadRequest, response.Code)
	}
}

func TestFilterPatientsInvalidJSON(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodPost,
		"/patients/filter",
		strings.NewReader(`invalid json`),
	)
	response := httptest.NewRecorder()

	handler.FilterPatients(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d",
			http.StatusBadRequest, response.Code)
	}
}

func TestFilterPatientsWrongMethod(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodGet,
		"/patients/filter",
		nil,
	)
	response := httptest.NewRecorder()

	handler.FilterPatients(response, request)

	if response.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected status %d, got %d",
			http.StatusMethodNotAllowed, response.Code)
	}
}
