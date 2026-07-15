package patient

import "context"

type PatientSearch struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

type PatientFilter struct {
	FirstName   string `json:"first_name"`
	LastName    string `json:"last_name"`
	DateOfBirth string `json:"date_of_birth"`
	Gender      string `json:"gender"`
}

type PatientSort struct {
	Field     string `json:"field"`
	Direction string `json:"direction"`
}

type PatientSearchRequest struct {
	Search     PatientSearch `json:"search"`
	Filter     PatientFilter `json:"filter"`
	Sort       PatientSort   `json:"sort"`
	Pagination PatientPagination
}

type PatientPagination struct {
	Limit  int
	Offset int
}

type Repository interface {
	Search(ctx context.Context, request PatientSearchRequest) ([]Patient, error)
}
