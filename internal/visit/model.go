package visit

import (
	"encoding/json"
	"time"

	"github.com/smithautotest/clinic-visits/internal/apitime"
)

type VisitStatus string

const (
	StatusScheduled  VisitStatus = "SCHEDULED"
	StatusInProgress VisitStatus = "IN_PROGRESS"
	StatusClosed     VisitStatus = "CLOSED"
	StatusCanceled   VisitStatus = "CANCELED"
)

func IsValidStatus(status VisitStatus) bool {
	switch status {
	case StatusScheduled, StatusInProgress, StatusClosed, StatusCanceled:
		return true
	default:
		return false
	}
}

func CanTransitionStatus(currentStatus, nextStatus VisitStatus) bool {
	if !IsValidStatus(currentStatus) || !IsValidStatus(nextStatus) {
		return false
	}

	if currentStatus == nextStatus {
		return true
	}

	switch currentStatus {
	case StatusScheduled:
		return nextStatus == StatusInProgress || nextStatus == StatusCanceled
	case StatusInProgress:
		return nextStatus == StatusClosed || nextStatus == StatusCanceled
	default:
		return false
	}
}

type Visit struct {
	ID             string      `json:"id"`
	DoctorID       string      `json:"doctor_id"`
	PatientID      string      `json:"patient_id"`
	ClinicID       string      `json:"clinic_id"`
	Status         VisitStatus `json:"status"`
	VisitStartTime time.Time   `json:"visit_start_time"`
	VisitEndTime   time.Time   `json:"visit_end_time"`
	CreatedAt      time.Time   `json:"created_at"`
	UpdatedAt      time.Time   `json:"updated_at"`
}

type VisitResponse struct {
	ID             string      `json:"id"`
	DoctorID       string      `json:"doctor_id"`
	PatientID      string      `json:"patient_id"`
	ClinicID       string      `json:"clinic_id"`
	Status         VisitStatus `json:"status"`
	VisitStartTime string      `json:"visit_start_time"`
	VisitEndTime   string      `json:"visit_end_time"`
	CreatedAt      string      `json:"created_at"`
	UpdatedAt      string      `json:"updated_at"`
}

func newVisitResponse(visit Visit) VisitResponse {
	return VisitResponse{
		ID:             visit.ID,
		DoctorID:       visit.DoctorID,
		PatientID:      visit.PatientID,
		ClinicID:       visit.ClinicID,
		Status:         visit.Status,
		VisitStartTime: apitime.FormatJSONTime(visit.VisitStartTime),
		VisitEndTime:   apitime.FormatJSONTime(visit.VisitEndTime),
		CreatedAt:      apitime.FormatJSONTime(visit.CreatedAt),
		UpdatedAt:      apitime.FormatJSONTime(visit.UpdatedAt),
	}
}

func (visit Visit) MarshalJSON() ([]byte, error) {
	return json.Marshal(newVisitResponse(visit))
}
