package visit

import (
	"context"
	"errors"
	"time"

	"github.com/smithautotest/clinic-visits/internal/pagination"
)

var (
	ErrDoctorNotFound       = errors.New("doctor not found")
	ErrPatientNotFound      = errors.New("patient not found")
	ErrClinicNotFound       = errors.New("clinic not found")
	ErrDoctorClinicMismatch = errors.New("doctor does not belong to clinic")
	ErrInvalidTimeRange     = errors.New("visit end time must be after start time")
)

type CreateVisitRequest struct {
	DoctorID       string    `json:"doctor_id"`
	PatientID      string    `json:"patient_id"`
	ClinicID       string    `json:"clinic_id"`
	VisitStartTime time.Time `json:"visit_start_time"`
	VisitEndTime   time.Time `json:"visit_end_time"`
}

type ListVisitsRequest struct {
	Pagination pagination.Params
}

type Repository interface {
	CreateVisit(ctx context.Context, request CreateVisitRequest) (Visit, error)
	ListVisits(ctx context.Context, request ListVisitsRequest) ([]Visit, error)
}
