package v2

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/doctor"
)

type fakeRepository struct {
	request doctor.DoctorSearchRequest
}

func (r *fakeRepository) FindDoctors(_ context.Context, request doctor.DoctorSearchRequest) ([]doctor.Doctor, error) {
	r.request = request
	return []doctor.Doctor{{
		ID:          "doctor-1",
		SpecialtyID: "specialty-1",
		ClinicID:    "clinic-1",
		FullName:    "Jane Doe",
	}}, nil
}

func TestRegisterDoctorsSearch(t *testing.T) {
	mux := http.NewServeMux()
	repository := &fakeRepository{}
	Register(mux, doctor.NewHandler(repository))

	request := httptest.NewRequest(
		http.MethodPost,
		"/v2/doctors/search",
		strings.NewReader(`{"filter":{"clinic_id":"33333333-3333-3333-3333-333333333333"}}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("POST /v2/doctors/search status = %d, want %d", response.Code, http.StatusOK)
	}
	if repository.request.Filter == nil || repository.request.Filter.ClinicID != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("FindDoctors() filter = %#v", repository.request.Filter)
	}

	wantBody := `{"data":[{"id":"doctor-1","specialty_id":"specialty-1","clinic_id":"clinic-1","full_name":"Jane Doe"}]}`
	if response.Body.String() != wantBody {
		t.Fatalf("POST /v2/doctors/search body = %s, want %s", response.Body.String(), wantBody)
	}
}

func TestRegisterDoctorsSearchUsesV1Validation(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, doctor.NewHandler(&fakeRepository{}))

	request := httptest.NewRequest(
		http.MethodPost,
		"/v2/doctors/search",
		strings.NewReader(`{"filter":{"doctor_id":"invalid"}}`),
	)
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)

	if response.Code != http.StatusBadRequest {
		t.Fatalf("invalid UUID status = %d, want %d", response.Code, http.StatusBadRequest)
	}
	if response.Body.String() != `{"error":"invalid UUID filter"}` {
		t.Fatalf("invalid UUID body = %s", response.Body.String())
	}
}
