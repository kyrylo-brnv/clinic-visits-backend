package visit

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
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

func TestMapCreateVisitErrorMapsVisitTimeConflict(t *testing.T) {
	t.Parallel()

	err := mapCreateVisitError(&pgconn.PgError{
		Code:           "23P01",
		ConstraintName: "visits_doctor_time_exclusion",
	})

	if !errors.Is(err, ErrVisitTimeConflict) {
		t.Fatalf("expected %v, got %v", ErrVisitTimeConflict, err)
	}
}

func TestMapCreateVisitErrorMapsPatientTimeConflict(t *testing.T) {
	t.Parallel()

	err := mapCreateVisitError(&pgconn.PgError{
		Code:           "23P01",
		ConstraintName: "visits_patient_time_exclusion",
	})

	if !errors.Is(err, ErrPatientTimeConflict) {
		t.Fatalf("expected %v, got %v", ErrPatientTimeConflict, err)
	}
}

func TestMapCreateVisitErrorRequiresPatientConstraintSQLState(t *testing.T) {
	t.Parallel()

	postgresError := &pgconn.PgError{
		Code:           "23514",
		ConstraintName: "visits_patient_time_exclusion",
	}
	err := mapCreateVisitError(postgresError)

	if errors.Is(err, ErrPatientTimeConflict) {
		t.Fatal("expected patient constraint with another SQLSTATE to remain a database error")
	}
	if !errors.Is(err, postgresError) {
		t.Fatalf("expected PostgreSQL error to be wrapped, got %v", err)
	}
}

func TestMapCreateVisitErrorDoesNotMapOtherExclusionConstraints(t *testing.T) {
	t.Parallel()

	postgresError := &pgconn.PgError{
		Code:           "23P01",
		ConstraintName: "another_exclusion_constraint",
	}
	err := mapCreateVisitError(postgresError)

	if errors.Is(err, ErrVisitTimeConflict) || errors.Is(err, ErrPatientTimeConflict) {
		t.Fatal("expected unrelated exclusion constraint to remain a database error")
	}
	if !errors.Is(err, postgresError) {
		t.Fatalf("expected PostgreSQL error to be wrapped, got %v", err)
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

func TestMapUpdateVisitErrorMapsMissingVisit(t *testing.T) {
	t.Parallel()

	if err := mapUpdateVisitError(pgx.ErrNoRows); !errors.Is(err, ErrVisitNotFound) {
		t.Fatalf("expected %v, got %v", ErrVisitNotFound, err)
	}
}

func TestMapUpdateVisitErrorMapsConstraints(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		code       string
		constraint string
		expected   error
	}{
		{name: "doctor", code: "23503", constraint: "visits_doctor_id_fkey", expected: ErrDoctorNotFound},
		{name: "patient", code: "23503", constraint: "visits_patient_id_fkey", expected: ErrPatientNotFound},
		{name: "clinic", code: "23503", constraint: "visits_clinic_id_fkey", expected: ErrClinicNotFound},
		{name: "doctor clinic mismatch", code: "23503", constraint: "visits_doctor_clinic_fkey", expected: ErrDoctorClinicMismatch},
		{name: "invalid time range", code: "23514", constraint: "visits_valid_time_range", expected: ErrInvalidTimeRange},
		{name: "visit time conflict", code: "23P01", constraint: "visits_doctor_time_exclusion", expected: ErrVisitTimeConflict},
		{name: "patient time conflict", code: "23P01", constraint: "visits_patient_time_exclusion", expected: ErrPatientTimeConflict},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := mapUpdateVisitError(&pgconn.PgError{
				Code:           test.code,
				ConstraintName: test.constraint,
			})
			if !errors.Is(err, test.expected) {
				t.Fatalf("expected %v, got %v", test.expected, err)
			}
		})
	}
}

func TestMapUpdateVisitErrorDoesNotMapOtherExclusionConstraints(t *testing.T) {
	t.Parallel()

	postgresError := &pgconn.PgError{
		Code:           "23P01",
		ConstraintName: "another_exclusion_constraint",
	}
	err := mapUpdateVisitError(postgresError)

	if errors.Is(err, ErrVisitTimeConflict) || errors.Is(err, ErrPatientTimeConflict) {
		t.Fatal("expected unrelated exclusion constraint to remain a database error")
	}
	if !errors.Is(err, postgresError) {
		t.Fatalf("expected PostgreSQL error to be wrapped, got %v", err)
	}
}

func TestMapUpdateVisitErrorRequiresPatientConstraintSQLState(t *testing.T) {
	t.Parallel()

	postgresError := &pgconn.PgError{
		Code:           "23514",
		ConstraintName: "visits_patient_time_exclusion",
	}
	err := mapUpdateVisitError(postgresError)

	if errors.Is(err, ErrPatientTimeConflict) {
		t.Fatal("expected patient constraint with another SQLSTATE to remain a database error")
	}
	if !errors.Is(err, postgresError) {
		t.Fatalf("expected PostgreSQL error to be wrapped, got %v", err)
	}
}

func TestMapUpdateVisitErrorWrapsUnknownError(t *testing.T) {
	t.Parallel()

	expected := errors.New("database unavailable")
	err := mapUpdateVisitError(expected)

	if !errors.Is(err, expected) {
		t.Fatalf("expected wrapped error %v, got %v", expected, err)
	}
}
