package patient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

type FakeRepository struct {
	Patients []Patient
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
	return r.Patients, nil
}

func TestSearchPatients(t *testing.T) {
	repo := NewFakeRepository()
	handler := NewHandler(repo)

	request := httptest.NewRequest(http.MethodGet, "/patients", nil)
	response := httptest.NewRecorder()

	handler.SearchPatients(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("Expected status code %d, got %d", http.StatusOK, response.Code)
	}

	expectedBody := `{"data":[{"id":"abc1234-5678-90ab-cdef-1234567890ab","first_name":"John","last_name":"Doe","date_of_birth":"1990-01-01","gender":"Male","is_deleted":false}]}`
	if response.Body.String() != expectedBody {
		t.Fatalf("Expected body %s, got %s", expectedBody, response.Body.String())
	}
}
