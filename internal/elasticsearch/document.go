package elasticsearch

import "time"

// DoctorDocument is the denormalized document stored in the doctors index.
type DoctorDocument struct {
	ID          string         `json:"id"`
	SpecialtyID string         `json:"specialty_id"`
	ClinicID    string         `json:"clinic_id"`
	FullName    string         `json:"full_name"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Visits      []VisitSummary `json:"visits"`
}

// PatientDocument is the denormalized document stored in the patients index.
type PatientDocument struct {
	ID          string         `json:"id"`
	FirstName   string         `json:"first_name"`
	LastName    string         `json:"last_name"`
	DateOfBirth time.Time      `json:"date_of_birth"`
	Gender      string         `json:"gender"`
	IsDeleted   bool           `json:"is_deleted"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Visits      []VisitSummary `json:"visits"`
}

// ClinicDocument is the denormalized document stored in the clinics index.
type ClinicDocument struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	Address   string         `json:"address"`
	TimeZone  string         `json:"time_zone"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	Visits    []VisitSummary `json:"visits"`
}

// VisitSummary is the nested visit representation embedded in entity documents.
type VisitSummary struct {
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

// VisitDocument is the searchable document stored in the visits index.
type VisitDocument struct {
	ID             string           `json:"id"`
	DoctorID       string           `json:"doctor_id"`
	PatientID      string           `json:"patient_id"`
	ClinicID       string           `json:"clinic_id"`
	Status         string           `json:"status"`
	VisitStartTime time.Time        `json:"visit_start_time"`
	VisitEndTime   time.Time        `json:"visit_end_time"`
	CreatedAt      time.Time        `json:"created_at"`
	UpdatedAt      time.Time        `json:"updated_at"`
	Doctor         VisitDoctorData  `json:"doctor"`
	Patient        VisitPatientData `json:"patient"`
	Clinic         VisitClinicData  `json:"clinic"`
}

// VisitDoctorData contains the doctor fields mapped within a visit document.
type VisitDoctorData struct {
	ID          string `json:"id"`
	SpecialtyID string `json:"specialty_id"`
	ClinicID    string `json:"clinic_id"`
	FullName    string `json:"full_name"`
}

// VisitPatientData contains the patient fields mapped within a visit document.
type VisitPatientData struct {
	ID          string    `json:"id"`
	FirstName   string    `json:"first_name"`
	LastName    string    `json:"last_name"`
	DateOfBirth time.Time `json:"date_of_birth"`
	Gender      string    `json:"gender"`
	IsDeleted   bool      `json:"is_deleted"`
}

// VisitClinicData contains the clinic fields mapped within a visit document.
type VisitClinicData struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Address  string `json:"address"`
	TimeZone string `json:"time_zone"`
}
