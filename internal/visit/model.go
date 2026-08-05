package visit

import (
	"encoding/json"
	"time"
)

const visitJSONTimeLayout = "2006-01-02T15:04:05.000000-07:00"

const (
	StatusScheduled  = "SCHEDULED"
	StatusInProgress = "IN_PROGRESS"
	StatusClosed     = "CLOSED"
	StatusCanceled   = "CANCELED"
)

func IsValidStatus(status string) bool {
	switch status {
	case StatusScheduled, StatusInProgress, StatusClosed, StatusCanceled:
		return true
	default:
		return false
	}
}

func CanTransitionStatus(currentStatus, nextStatus string) bool {
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

func (visit Visit) MarshalJSON() ([]byte, error) {
	type response struct {
		ID             string `json:"id"`
		DoctorID       string `json:"doctor_id"`
		PatientID      string `json:"patient_id"`
		ClinicID       string `json:"clinic_id"`
		Status         string `json:"status"`
		VisitStartTime string `json:"visit_start_time"`
		VisitEndTime   string `json:"visit_end_time"`
		CreatedAt      string `json:"created_at"`
		UpdatedAt      string `json:"updated_at"`
	}

	return json.Marshal(response{
		ID:             visit.ID,
		DoctorID:       visit.DoctorID,
		PatientID:      visit.PatientID,
		ClinicID:       visit.ClinicID,
		Status:         visit.Status,
		VisitStartTime: formatVisitJSONTime(visit.VisitStartTime),
		VisitEndTime:   formatVisitJSONTime(visit.VisitEndTime),
		CreatedAt:      formatVisitJSONTime(visit.CreatedAt),
		UpdatedAt:      formatVisitJSONTime(visit.UpdatedAt),
	})
}

func formatVisitJSONTime(value time.Time) string {
	return value.Truncate(time.Microsecond).Format(visitJSONTimeLayout)
}
