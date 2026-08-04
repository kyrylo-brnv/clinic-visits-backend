package visit

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
	"github.com/smithautotest/clinic-visits/internal/uuid"
)

type PostgresRepository struct {
	queries *sqlc.Queries
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{queries: sqlc.New(pool)}
}

func (r *PostgresRepository) CreateVisit(
	ctx context.Context,
	request CreateVisitRequest,
) (Visit, error) {
	doctorID, err := uuid.Parse(request.DoctorID)
	if err != nil {
		return Visit{}, fmt.Errorf("invalid doctor ID: %w", err)
	}

	patientID, err := uuid.Parse(request.PatientID)
	if err != nil {
		return Visit{}, fmt.Errorf("invalid patient ID: %w", err)
	}

	clinicID, err := uuid.Parse(request.ClinicID)
	if err != nil {
		return Visit{}, fmt.Errorf("invalid clinic ID: %w", err)
	}

	row, err := r.queries.CreateVisit(ctx, sqlc.CreateVisitParams{
		DoctorID:       doctorID,
		PatientID:      patientID,
		ClinicID:       clinicID,
		VisitStartTime: pgtype.Timestamptz{Time: request.VisitStartTime, Valid: true},
		VisitEndTime:   pgtype.Timestamptz{Time: request.VisitEndTime, Valid: true},
	})
	if err != nil {
		return Visit{}, mapCreateVisitError(err)
	}

	return Visit{
		ID:             row.ID,
		DoctorID:       row.DoctorID,
		PatientID:      row.PatientID,
		ClinicID:       row.ClinicID,
		VisitStartTime: row.VisitStartTime.Time,
		VisitEndTime:   row.VisitEndTime.Time,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}, nil
}

func mapCreateVisitError(err error) error {
	var postgresError *pgconn.PgError
	if errors.As(err, &postgresError) {
		if postgresError.Code == "23503" {
			switch postgresError.ConstraintName {
			case "visits_doctor_id_fkey":
				return ErrDoctorNotFound
			case "visits_patient_id_fkey":
				return ErrPatientNotFound
			case "visits_clinic_id_fkey":
				return ErrClinicNotFound
			case "visits_doctor_clinic_fkey":
				return ErrDoctorClinicMismatch
			}
		}

		if postgresError.Code == "23514" &&
			postgresError.ConstraintName == "visits_valid_time_range" {
			return ErrInvalidTimeRange
		}
	}

	return fmt.Errorf("failed to create visit: %w", err)
}
