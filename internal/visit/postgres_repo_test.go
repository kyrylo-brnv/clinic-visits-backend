package visit

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

func TestMapCreateVisitErrorMapsMissingReferences(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		constraint string
		expected   error
	}{
		{
			name:       "doctor",
			constraint: "visits_doctor_id_fkey",
			expected:   ErrDoctorNotFound,
		},
		{
			name:       "patient",
			constraint: "visits_patient_id_fkey",
			expected:   ErrPatientNotFound,
		},
		{
			name:       "clinic",
			constraint: "visits_clinic_id_fkey",
			expected:   ErrClinicNotFound,
		},
		{
			name:       "doctor clinic mismatch",
			constraint: "visits_doctor_clinic_fkey",
			expected:   ErrDoctorClinicMismatch,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := mapCreateVisitError(&pgconn.PgError{
				Code:           "23503",
				ConstraintName: test.constraint,
			})

			if !errors.Is(err, test.expected) {
				t.Fatalf("expected %v, got %v", test.expected, err)
			}
		})
	}
}

func TestMapCreateVisitErrorWrapsUnknownError(t *testing.T) {
	t.Parallel()

	expected := errors.New("database unavailable")
	err := mapCreateVisitError(expected)

	if !errors.Is(err, expected) {
		t.Fatalf("expected wrapped error %v, got %v", expected, err)
	}
}

func TestMapCreateVisitErrorMapsInvalidTimeRange(t *testing.T) {
	t.Parallel()

	err := mapCreateVisitError(&pgconn.PgError{
		Code:           "23514",
		ConstraintName: "visits_valid_time_range",
	})

	if !errors.Is(err, ErrInvalidTimeRange) {
		t.Fatalf("expected %v, got %v", ErrInvalidTimeRange, err)
	}
}

func TestMapCreateVisitErrorDoesNotMapOtherCheckConstraints(t *testing.T) {
	t.Parallel()

	postgresError := &pgconn.PgError{
		Code:           "23514",
		ConstraintName: "another_check_constraint",
	}
	err := mapCreateVisitError(postgresError)

	if errors.Is(err, ErrInvalidTimeRange) {
		t.Fatalf("expected unrelated constraint to remain a database error")
	}
	if !errors.Is(err, postgresError) {
		t.Fatalf("expected PostgreSQL error to be wrapped, got %v", err)
	}
}
