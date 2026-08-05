package visit

import (
	"time"

	"github.com/smithautotest/clinic-visits/internal/outbox"
)

type createdEventPayload struct {
	ID             string    `json:"id"`
	DoctorID       string    `json:"doctor_id"`
	PatientID      string    `json:"patient_id"`
	ClinicID       string    `json:"clinic_id"`
	Status         string    `json:"status"`
	VisitStartTime time.Time `json:"visit_start_time"`
	VisitEndTime   time.Time `json:"visit_end_time"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type updatedEventPayload struct {
	ID             string    `json:"id"`
	DoctorID       string    `json:"doctor_id"`
	PatientID      string    `json:"patient_id"`
	ClinicID       string    `json:"clinic_id"`
	Status         string    `json:"status"`
	VisitStartTime time.Time `json:"visit_start_time"`
	VisitEndTime   time.Time `json:"visit_end_time"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type deletedEventPayload struct {
	ID             string    `json:"id"`
	DoctorID       string    `json:"doctor_id"`
	PatientID      string    `json:"patient_id"`
	ClinicID       string    `json:"clinic_id"`
	Status         string    `json:"status"`
	VisitStartTime time.Time `json:"visit_start_time"`
	VisitEndTime   time.Time `json:"visit_end_time"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func newCreatedEvent(visit Visit) (outbox.Event, error) {
	return outbox.NewEvent(
		outbox.AggregateTypeVisit,
		visit.ID,
		outbox.EventTypeVisitCreated,
		createdEventPayload{
			ID:             visit.ID,
			DoctorID:       visit.DoctorID,
			PatientID:      visit.PatientID,
			ClinicID:       visit.ClinicID,
			Status:         visit.Status,
			VisitStartTime: visit.VisitStartTime,
			VisitEndTime:   visit.VisitEndTime,
			CreatedAt:      visit.CreatedAt,
			UpdatedAt:      visit.UpdatedAt,
		},
	)
}

func newUpdatedEvent(visit Visit) (outbox.Event, error) {
	return outbox.NewEvent(
		outbox.AggregateTypeVisit,
		visit.ID,
		outbox.EventTypeVisitUpdated,
		updatedEventPayload{
			ID:             visit.ID,
			DoctorID:       visit.DoctorID,
			PatientID:      visit.PatientID,
			ClinicID:       visit.ClinicID,
			Status:         visit.Status,
			VisitStartTime: visit.VisitStartTime,
			VisitEndTime:   visit.VisitEndTime,
			CreatedAt:      visit.CreatedAt,
			UpdatedAt:      visit.UpdatedAt,
		},
	)
}

func newDeletedEvent(visit Visit) (outbox.Event, error) {
	return outbox.NewEvent(
		outbox.AggregateTypeVisit,
		visit.ID,
		outbox.EventTypeVisitDeleted,
		deletedEventPayload{
			ID:             visit.ID,
			DoctorID:       visit.DoctorID,
			PatientID:      visit.PatientID,
			ClinicID:       visit.ClinicID,
			Status:         visit.Status,
			VisitStartTime: visit.VisitStartTime,
			VisitEndTime:   visit.VisitEndTime,
			CreatedAt:      visit.CreatedAt,
			UpdatedAt:      visit.UpdatedAt,
		},
	)
}
