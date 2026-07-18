package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/patient"
)

type fakeRepository struct{}

func (fakeRepository) FindPatients(
	context.Context,
	patient.PatientSearchRequest,
) ([]patient.Patient, error) {
	return []patient.Patient{}, nil
}

func TestRegisterPatientsSearch(t *testing.T) {
	mux := http.NewServeMux()
	handler := patient.NewHandler(fakeRepository{})

	Register(mux, handler)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/patients/search",
		strings.NewReader(`{
                        "search": {
                                "first_name": "ann"
                        }
                }`),
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
