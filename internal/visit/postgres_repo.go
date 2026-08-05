package visit

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
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
		Status:         row.Status,
		VisitStartTime: row.VisitStartTime.Time,
		VisitEndTime:   row.VisitEndTime.Time,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}, nil
}

func (r *PostgresRepository) ListVisits(
	ctx context.Context,
	request ListVisitsRequest,
) ([]Visit, error) {
	rows, err := r.queries.ListVisits(ctx, sqlc.ListVisitsParams{
		PageLimit:  request.Pagination.Limit(),
		PageOffset: request.Pagination.Offset(),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list visits: %w", err)
	}

	visits := make([]Visit, 0, len(rows))
	for _, row := range rows {
		visits = append(visits, Visit{
			ID:             row.ID,
			DoctorID:       row.DoctorID,
			PatientID:      row.PatientID,
			ClinicID:       row.ClinicID,
			Status:         row.Status,
			VisitStartTime: row.VisitStartTime.Time,
			VisitEndTime:   row.VisitEndTime.Time,
			CreatedAt:      row.CreatedAt.Time,
			UpdatedAt:      row.UpdatedAt.Time,
		})
	}

	return visits, nil
}

func (r *PostgresRepository) DeleteVisit(
	ctx context.Context,
	request DeleteVisitRequest,
) error {
	visitID, err := uuid.Parse(request.VisitID)
	if err != nil {
		return fmt.Errorf("invalid visit ID: %w", err)
	}

	if _, err := r.queries.DeleteVisit(ctx, visitID); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrVisitNotFound
		}

		return fmt.Errorf("failed to delete visit: %w", err)
	}

	return nil
}

func (r *PostgresRepository) UpdateVisit(
	ctx context.Context,
	request UpdateVisitRequest,
) (Visit, error) {
	visitID, err := uuid.Parse(request.VisitID)
	if err != nil {
		return Visit{}, fmt.Errorf("invalid visit ID: %w", err)
	}

	doctorID, err := parseOptionalUUID(request.DoctorID, "doctor")
	if err != nil {
		return Visit{}, err
	}

	patientID, err := parseOptionalUUID(request.PatientID, "patient")
	if err != nil {
		return Visit{}, err
	}

	clinicID, err := parseOptionalUUID(request.ClinicID, "clinic")
	if err != nil {
		return Visit{}, err
	}

	row, err := r.queries.UpdateVisit(ctx, sqlc.UpdateVisitParams{
		VisitID:        visitID,
		DoctorID:       doctorID,
		PatientID:      patientID,
		ClinicID:       clinicID,
		VisitStartTime: optionalTimestamp(request.VisitStartTime),
		VisitEndTime:   optionalTimestamp(request.VisitEndTime),
	})
	if err != nil {
		return Visit{}, mapUpdateVisitError(err)
	}

	return Visit{
		ID:             row.ID,
		DoctorID:       row.DoctorID,
		PatientID:      row.PatientID,
		ClinicID:       row.ClinicID,
		Status:         row.Status,
		VisitStartTime: row.VisitStartTime.Time,
		VisitEndTime:   row.VisitEndTime.Time,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}, nil
}

func parseOptionalUUID(value *string, fieldName string) (pgtype.UUID, error) {
	if value == nil {
		return pgtype.UUID{}, nil
	}

	id, err := uuid.Parse(*value)
	if err != nil {
		return pgtype.UUID{}, fmt.Errorf("invalid %s ID: %w", fieldName, err)
	}

	return id, nil
}

func optionalTimestamp(value *time.Time) pgtype.Timestamptz {
	if value == nil {
		return pgtype.Timestamptz{}
	}

	return pgtype.Timestamptz{Time: *value, Valid: true}
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

		if postgresError.Code == "23P01" &&
			postgresError.ConstraintName == "visits_doctor_time_exclusion" {
			return ErrVisitTimeConflict
		}

		if postgresError.Code == "23P01" &&
			postgresError.ConstraintName == "visits_patient_time_exclusion" {
			return ErrPatientTimeConflict
		}
	}

	return fmt.Errorf("failed to create visit: %w", err)
}

func mapUpdateVisitError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrVisitNotFound
	}

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

		if postgresError.Code == "23P01" &&
			postgresError.ConstraintName == "visits_doctor_time_exclusion" {
			return ErrVisitTimeConflict
		}

		if postgresError.Code == "23P01" &&
			postgresError.ConstraintName == "visits_patient_time_exclusion" {
			return ErrPatientTimeConflict
		}
	}

	return fmt.Errorf("failed to update visit: %w", err)
}
