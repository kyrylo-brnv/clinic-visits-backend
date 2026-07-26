package patient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type FakeRepository struct {
	Patients           []Patient
	LastSearch         PatientSearchRequest
	FilterError        error
	FindPatientsCalled bool
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

func (r *FakeRepository) FindPatients(
	ctx context.Context,
	request PatientSearchRequest,
) ([]Patient, error) {
	r.FindPatientsCalled = true
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

func TestSearchPatientsRejectsEmptyCriteria(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{
			name: "empty request",
			body: `{}`,
		},
		{
			name: "empty search",
			body: `{
						"search": {
								"first_name": "",
								"last_name": ""
						}
					}`,
		},
		{
			name: "empty filter",
			body: `{"filter": {}}`,
		},
		{
			name: "empty ID filter",
			body: `{"filter": {"id": {}}}`,
		}, {
			name: "empty equals ID",
			body: `{
                "filter": {
                        "id": {
                                "equals": ""
                        }
                }
        }`,
		},
		{
			name: "empty not-equals ID",
			body: `{
                "filter": {
                        "id": {
                                "not_equals": ""
                        }
                }
        }`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			repo := NewFakeRepository()
			handler := NewHandler(repo)

			request := httptest.NewRequest(
				http.MethodPost,
				"/patients/search",
				strings.NewReader(test.body),
			)
			response := httptest.NewRecorder()

			handler.SearchPatients(response, request)

			if response.Code != http.StatusBadRequest {
				t.Fatalf(
					"expected status %d, got %d",
					http.StatusBadRequest,
					response.Code,
				)
			}

			if repo.FindPatientsCalled {
				t.Fatal("expected repository not to be called")
			}
		})
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

func TestSearchPatientsWithIDEqualsFilter(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodPost,
		"/patients/search",
		strings.NewReader(`{
                        "filter": {
                                "id": {
                                        "equals": "fed95cc1-24c4-4076-af88-591828d6928a"
                                }
                        }
                }`),
	)
	response := httptest.NewRecorder()

	handler.SearchPatients(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	if repo.LastSearch.Filter == nil {
		t.Fatal("expected filter to be forwarded to repository")
	}

	if repo.LastSearch.Filter.Id == nil {
		t.Fatal("expected id filter to be forwarded to repository")
	}

	if repo.LastSearch.Filter.Id.Equals == nil {
		t.Fatal("expected equals filter to be forwarded to repository")
	}

	expectedID := "fed95cc1-24c4-4076-af88-591828d6928a"

	if *repo.LastSearch.Filter.Id.Equals != expectedID {
		t.Fatalf(
			"expected id %q, got %q",
			expectedID,
			*repo.LastSearch.Filter.Id.Equals,
		)
	}
}

func TestSearchPatientsWithSearchAndFilter(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(
		http.MethodPost,
		"/patients/search",
		strings.NewReader(`{
                        "search": {
                                "first_name": "Ann"
                        },
                        "filter": {
                                "id": {
                                        "not_equals": "fed95cc1-24c4-4076-af88-591828d6928a"
                                }
                        }
                }`),
	)
	response := httptest.NewRecorder()

	handler.SearchPatients(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf(
			"expected status %d, got %d",
			http.StatusOK,
			response.Code,
		)
	}

	if !repo.FindPatientsCalled {
		t.Fatal("expected repository to be called")
	}

	if repo.LastSearch.Search == nil {
		t.Fatal("expected search to be forwarded to repository")
	}

	if repo.LastSearch.Search.FirstName != "Ann" {
		t.Fatalf(
			"expected first name %q, got %q",
			"Ann",
			repo.LastSearch.Search.FirstName,
		)
	}

	if repo.LastSearch.Filter == nil ||
		repo.LastSearch.Filter.Id == nil ||
		repo.LastSearch.Filter.Id.NotEquals == nil {
		t.Fatal("expected not_equals filter to be forwarded to repository")
	}

	expectedID := "fed95cc1-24c4-4076-af88-591828d6928a"

	if *repo.LastSearch.Filter.Id.NotEquals != expectedID {
		t.Fatalf(
			"expected ID %q, got %q",
			expectedID,
			*repo.LastSearch.Filter.Id.NotEquals,
		)
	}
}
