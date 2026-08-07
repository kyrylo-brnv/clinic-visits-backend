package visit

import (
	"context"
	"errors"
	"time"

	"github.com/smithautotest/clinic-visits/internal/pagination"
)

var (
	ErrDoctorNotFound          = errors.New("doctor not found")
	ErrPatientNotFound         = errors.New("patient not found")
	ErrClinicNotFound          = errors.New("clinic not found")
	ErrVisitNotFound           = errors.New("visit not found")
	ErrVisitTimeConflict       = errors.New("doctor already has a visit during this time")
	ErrPatientTimeConflict     = errors.New("patient already has a visit during this time")
	ErrDoctorClinicMismatch    = errors.New("doctor does not belong to clinic")
	ErrInvalidTimeRange        = errors.New("visit end time must be after start time")
	ErrInvalidStatusTransition = errors.New("invalid visit status transition")
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

type ListRepository interface {
	ListVisits(ctx context.Context, request ListVisitsRequest) ([]Visit, error)
}

type DeleteVisitRequest struct {
	VisitID string `json:"visit_id"`
}

type UpdateVisitRequest struct {
	VisitID        string     `json:"visit_id"`
	DoctorID       *string    `json:"doctor_id"`
	PatientID      *string    `json:"patient_id"`
	ClinicID       *string    `json:"clinic_id"`
	VisitStartTime *time.Time `json:"visit_start_time"`
	VisitEndTime   *time.Time `json:"visit_end_time"`
	Status         *string    `json:"status"`
}

func (r UpdateVisitRequest) HasChanges() bool {
	return r.DoctorID != nil ||
		r.PatientID != nil ||
		r.ClinicID != nil ||
		r.VisitStartTime != nil ||
		r.VisitEndTime != nil ||
		r.Status != nil
}

type Repository interface {
	ListRepository
	CreateVisit(ctx context.Context, request CreateVisitRequest) (Visit, error)
	DeleteVisit(ctx context.Context, request DeleteVisitRequest) error
	UpdateVisit(ctx context.Context, request UpdateVisitRequest) (Visit, error)
}
