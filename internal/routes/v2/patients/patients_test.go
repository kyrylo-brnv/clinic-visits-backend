package v2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/patient"
)

type fakeRepository struct{}

func (fakeRepository) FindPatients(context.Context, patient.PatientSearchRequest) ([]patient.Patient, error) {
	return []patient.Patient{}, nil
}

func TestRegisterPatientsSearch(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, patient.NewHandler(fakeRepository{}))

	request := httptest.NewRequest(http.MethodPost, "/v2/patients/search", strings.NewReader(`{"search":{"first_name":"ann"}}`))
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("POST /v2/patients/search status = %d, want %d", response.Code, http.StatusOK)
	}

	request = httptest.NewRequest(http.MethodPost, "/v2/patients/search", strings.NewReader(`{"unknown":true}`))
	response = httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want %d", response.Code, http.StatusBadRequest)
	}
}
