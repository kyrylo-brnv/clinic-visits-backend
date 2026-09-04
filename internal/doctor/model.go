package doctor

import (
	"time"

	"github.com/smithautotest/clinic-visits/internal/apitime"
)

type Doctor struct {
	ID          string         `json:"id"`
	SpecialtyID string         `json:"specialty_id"`
	ClinicID    string         `json:"clinic_id"`
	FullName    string         `json:"full_name"`
	Visits      []VisitSummary `json:"visits,omitempty"`
}

// VisitSummary identifies an appointment embedded in an Elasticsearch doctor result.
type VisitSummary struct {
	ID              string    `json:"id"`
	DoctorID        string    `json:"doctor_id"`
	PatientID       string    `json:"patient_id"`
	PatientFullName string    `json:"patient_full_name"`
	ClinicID        string    `json:"clinic_id"`
	Status          string    `json:"status"`
	VisitStartTime  time.Time `json:"visit_start_time"`
	VisitEndTime    time.Time `json:"visit_end_time"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

type doctorV1Response struct {
	ID          string `json:"id"`
	SpecialtyID string `json:"specialty_id"`
	ClinicID    string `json:"clinic_id"`
	FullName    string `json:"full_name"`
}

type doctorV2Response struct {
	ID          string                 `json:"id"`
	SpecialtyID string                 `json:"specialty_id"`
	ClinicID    string                 `json:"clinic_id"`
	FullName    string                 `json:"full_name"`
	Visits      []doctorV2VisitSummary `json:"visits"`
}

type doctorV2VisitSummary struct {
	ID              string `json:"id"`
	DoctorID        string `json:"doctor_id"`
	PatientID       string `json:"patient_id"`
	PatientFullName string `json:"patient_full_name"`
	ClinicID        string `json:"clinic_id"`
	Status          string `json:"status"`
	VisitStartTime  string `json:"visit_start_time"`
	VisitEndTime    string `json:"visit_end_time"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

func newDoctorV1Responses(doctors []Doctor) []doctorV1Response {
	responses := make([]doctorV1Response, 0, len(doctors))
	for _, doctor := range doctors {
		responses = append(responses, doctorV1Response{
			ID:          doctor.ID,
			SpecialtyID: doctor.SpecialtyID,
			ClinicID:    doctor.ClinicID,
			FullName:    doctor.FullName,
		})
	}
	return responses
}

func newDoctorV2Responses(doctors []Doctor) []doctorV2Response {
	responses := make([]doctorV2Response, 0, len(doctors))
	for _, doctor := range doctors {
		visits := make([]doctorV2VisitSummary, 0, len(doctor.Visits))
		for _, visit := range doctor.Visits {
			visits = append(visits, doctorV2VisitSummary{
				ID:              visit.ID,
				DoctorID:        visit.DoctorID,
				PatientID:       visit.PatientID,
				PatientFullName: visit.PatientFullName,
				ClinicID:        visit.ClinicID,
				Status:          visit.Status,
				VisitStartTime:  apitime.FormatJSONTime(visit.VisitStartTime),
				VisitEndTime:    apitime.FormatJSONTime(visit.VisitEndTime),
				CreatedAt:       apitime.FormatJSONTime(visit.CreatedAt),
				UpdatedAt:       apitime.FormatJSONTime(visit.UpdatedAt),
			})
		}
		responses = append(responses, doctorV2Response{
			ID:          doctor.ID,
			SpecialtyID: doctor.SpecialtyID,
			ClinicID:    doctor.ClinicID,
			FullName:    doctor.FullName,
			Visits:      visits,
		})
	}
	return responses
}
