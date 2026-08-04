package doctor

import "context"

type DoctorFilter struct {
	DoctorID string `json:"doctor_id"`
	VisitID  string `json:"visit_id"`
	ClinicID string `json:"clinic_id"`
}

type DoctorSearchRequest struct {
	Filter *DoctorFilter `json:"filter"`
}

type Repository interface {
	FindDoctors(ctx context.Context, request DoctorSearchRequest) ([]Doctor, error)
}
