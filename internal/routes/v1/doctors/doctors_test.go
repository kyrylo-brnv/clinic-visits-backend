package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/smithautotest/clinic-visits/internal/doctor"
)

type fakeRepository struct{}

func (fakeRepository) FindDoctors(
	context.Context,
	doctor.DoctorSearchRequest,
) ([]doctor.Doctor, error) {
	return []doctor.Doctor{}, nil
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
