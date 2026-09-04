package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/doctor"
)

type fakeRepository struct {
	doctors []doctor.Doctor
}

func (r fakeRepository) FindDoctors(
	context.Context,
	doctor.DoctorSearchRequest,
) ([]doctor.Doctor, error) {
	return r.doctors, nil
}

func TestRegisterDoctorsSearchOmitsVisits(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, doctor.NewHandler(fakeRepository{doctors: []doctor.Doctor{{
		ID: "doctor-1", SpecialtyID: "specialty-1", ClinicID: "clinic-1", FullName: "Jane Doe",
		Visits: []doctor.VisitSummary{{ID: "visit-1", PatientID: "patient-1"}},
	}}}))

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/doctors/search", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("POST /v1/doctors/search status = %d, want %d", response.Code, http.StatusOK)
	}
	wantBody := `{"data":[{"id":"doctor-1","specialty_id":"specialty-1","clinic_id":"clinic-1","full_name":"Jane Doe"}]}`
	if response.Body.String() != wantBody {
		t.Fatalf("POST /v1/doctors/search body = %s, want %s", response.Body.String(), wantBody)
	}
}

func TestRegisterDoctorsSearch(t *testing.T) {
	mux := http.NewServeMux()
	handler := doctor.NewHandler(fakeRepository{})

	Register(mux, handler)

	request := httptest.NewRequest(
		http.MethodPost,
		"/v1/doctors/search",
		nil,
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
