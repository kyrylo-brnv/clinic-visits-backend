package visit

import "time"

type Visit struct {
	ID             string    `json:"id"`
	DoctorID       string    `json:"doctor_id"`
	PatientID      string    `json:"patient_id"`
	ClinicID       string    `json:"clinic_id"`
	VisitStartTime time.Time `json:"visit_start_time"`
	VisitEndTime   time.Time `json:"visit_end_time"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}
