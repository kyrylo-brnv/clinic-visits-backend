package v2

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/smithautotest/clinic-visits/internal/doctor"
)

type fakeRepository struct {
	request doctor.DoctorSearchRequest
	doctors []doctor.Doctor
}

func (r *fakeRepository) FindDoctors(_ context.Context, request doctor.DoctorSearchRequest) ([]doctor.Doctor, error) {
	r.request = request
	return r.doctors, nil
}

func testDoctorWithVisit() doctor.Doctor {
	zone := time.FixedZone("EEST", 3*60*60)
	createdAt := time.Date(2026, time.August, 5, 14, 30, 45, 123456789, zone)
	return doctor.Doctor{
		ID:          "doctor-1",
		SpecialtyID: "specialty-1",
		ClinicID:    "clinic-1",
		FullName:    "Jane Doe",
		Visits: []doctor.VisitSummary{{
			ID: "visit-1", DoctorID: "doctor-1", PatientID: "patient-1", PatientFullName: "Ada Lovelace",
			ClinicID: "clinic-1", Status: "SCHEDULED",
			VisitStartTime: createdAt.Add(24 * time.Hour),
			VisitEndTime:   createdAt.Add(25 * time.Hour),
			CreatedAt:      createdAt,
			UpdatedAt:      createdAt.Add(time.Hour),
		}},
	}
}

func TestRegisterDoctorsSearch(t *testing.T) {
	mux := http.NewServeMux()
	repository := &fakeRepository{doctors: []doctor.Doctor{testDoctorWithVisit()}}
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

	wantBody := `{"data":[{"id":"doctor-1","specialty_id":"specialty-1","clinic_id":"clinic-1","full_name":"Jane Doe","visits":[{"id":"visit-1","doctor_id":"doctor-1","patient_id":"patient-1","patient_full_name":"Ada Lovelace","clinic_id":"clinic-1","status":"SCHEDULED","visit_start_time":"2026-08-06T14:30:45.123456+03:00","visit_end_time":"2026-08-06T15:30:45.123456+03:00","created_at":"2026-08-05T14:30:45.123456+03:00","updated_at":"2026-08-05T15:30:45.123456+03:00"}]}]}`
	if response.Body.String() != wantBody {
		t.Fatalf("POST /v2/doctors/search body = %s, want %s", response.Body.String(), wantBody)
	}
}

func TestRegisterDoctorsSearchIncludesEmptyVisits(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux, doctor.NewHandler(&fakeRepository{doctors: []doctor.Doctor{{
		ID: "doctor-1", SpecialtyID: "specialty-1", ClinicID: "clinic-1", FullName: "Jane Doe",
	}}}))

	response := httptest.NewRecorder()
	mux.ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v2/doctors/search", nil))

	if response.Code != http.StatusOK {
		t.Fatalf("POST /v2/doctors/search status = %d, want %d", response.Code, http.StatusOK)
	}
	wantBody := `{"data":[{"id":"doctor-1","specialty_id":"specialty-1","clinic_id":"clinic-1","full_name":"Jane Doe","visits":[]}]}`
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
	var body struct {
		Code      string `json:"code"`
		Message   string `json:"message"`
		RequestID string `json:"request_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode invalid UUID response: %v", err)
	}
	if body.Code != "BAD_REQUEST" || body.Message != "invalid UUID filter" || body.RequestID == "" {
		t.Fatalf("invalid UUID response = %#v", body)
	}
}
