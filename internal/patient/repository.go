package patient

import (
	"context"

	"github.com/smithautotest/clinic-visits/internal/filter"
	"github.com/smithautotest/clinic-visits/internal/sorting"
)

type PatientSearch struct {
	FirstName string `json:"first_name"`
	LastName  string `json:"last_name"`
}

func (s *PatientSearch) isEmpty() bool {
	if s == nil {
		return true
	}

	return *s == (PatientSearch{})
}

type PatientFilter struct {
	Id *filter.StringFilter `json:"id"`
}

func (f *PatientFilter) isEmpty() bool {
	if f == nil {
		return true
	}

	return f.Id.IsEmpty()
}

type PatientSearchRequest struct {
	Search *PatientSearch `json:"search"`
	Filter *PatientFilter `json:"filter"`
	Sort   *sorting.Sort  `json:"sort"`
}

type Repository interface {
	FindPatients(
		ctx context.Context,
		request PatientSearchRequest,
	) ([]Patient, error)
}
