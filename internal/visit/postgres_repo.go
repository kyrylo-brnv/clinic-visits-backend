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
	"github.com/smithautotest/clinic-visits/internal/outbox"
	"github.com/smithautotest/clinic-visits/internal/uuid"
)

type PostgresRepository struct {
	queries  *sqlc.Queries
	database postgresDatabase
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return newPostgresRepository(pool)
}

type postgresDatabase interface {
	sqlc.DBTX
	Begin(ctx context.Context) (pgx.Tx, error)
}

func newPostgresRepository(database postgresDatabase) *PostgresRepository {
	return &PostgresRepository{
		queries:  sqlc.New(database),
		database: database,
	}
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

	transaction, err := r.database.Begin(ctx)
	if err != nil {
		return Visit{}, fmt.Errorf("begin visit creation transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	queries := r.queries.WithTx(transaction)
	row, err := queries.CreateVisit(ctx, sqlc.CreateVisitParams{
		DoctorID:       doctorID,
		PatientID:      patientID,
		ClinicID:       clinicID,
		VisitStartTime: pgtype.Timestamptz{Time: request.VisitStartTime, Valid: true},
		VisitEndTime:   pgtype.Timestamptz{Time: request.VisitEndTime, Valid: true},
	})
	if err != nil {
		return Visit{}, mapCreateVisitError(err)
	}

	createdVisit := Visit{
		ID:             row.ID,
		DoctorID:       row.DoctorID,
		PatientID:      row.PatientID,
		ClinicID:       row.ClinicID,
		Status:         row.Status,
		VisitStartTime: row.VisitStartTime.Time,
		VisitEndTime:   row.VisitEndTime.Time,
		CreatedAt:      row.CreatedAt.Time,
		UpdatedAt:      row.UpdatedAt.Time,
	}

	event, err := newCreatedEvent(createdVisit)
	if err != nil {
		return Visit{}, fmt.Errorf("create visit created outbox event: %w", err)
	}

	if err := insertVisitOutboxEvent(ctx, queries, event, "created"); err != nil {
		return Visit{}, err
	}

	if err := transaction.Commit(ctx); err != nil {
		return Visit{}, fmt.Errorf("commit visit creation transaction: %w", err)
	}

	return createdVisit, nil
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

	transaction, err := r.database.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin visit deletion transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	queries := r.queries.WithTx(transaction)
	row, err := queries.DeleteVisit(ctx, visitID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrVisitNotFound
		}

		return fmt.Errorf("failed to delete visit: %w", err)
	}
	deletedVisit := mapDeleteVisitRow(row)

	event, err := newDeletedEvent(deletedVisit)
	if err != nil {
		return fmt.Errorf("create visit deleted outbox event: %w", err)
	}
	if err := insertVisitOutboxEvent(ctx, queries, event, "deleted"); err != nil {
		return err
	}

	if err := transaction.Commit(ctx); err != nil {
		return fmt.Errorf("commit visit deletion transaction: %w", err)
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

	params := sqlc.UpdateVisitParams{
		VisitID:        visitID,
		DoctorID:       doctorID,
		PatientID:      patientID,
		ClinicID:       clinicID,
		VisitStartTime: optionalTimestamp(request.VisitStartTime),
		VisitEndTime:   optionalTimestamp(request.VisitEndTime),
		Status:         optionalText(request.Status),
	}
	transaction, err := r.database.Begin(ctx)
	if err != nil {
		return Visit{}, fmt.Errorf("begin visit update transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	queries := r.queries.WithTx(transaction)
	if request.Status != nil {
		currentStatus, err := queries.GetVisitStatusForUpdate(ctx, visitID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return Visit{}, ErrVisitNotFound
			}

			return Visit{}, fmt.Errorf("lock visit for status update: %w", err)
		}

		if !CanTransitionStatus(currentStatus, *request.Status) {
			return Visit{}, fmt.Errorf(
				"%w: %s -> %s",
				ErrInvalidStatusTransition,
				currentStatus,
				*request.Status,
			)
		}
	}

	row, err := queries.UpdateVisit(ctx, params)
	if err != nil {
		return Visit{}, mapUpdateVisitError(err)
	}
	updatedVisit := mapUpdateVisitRow(row)

	event, err := newUpdatedEvent(updatedVisit)
	if err != nil {
		return Visit{}, fmt.Errorf("create visit updated outbox event: %w", err)
	}
	if err := insertVisitOutboxEvent(ctx, queries, event, "updated"); err != nil {
		return Visit{}, err
	}

	if err := transaction.Commit(ctx); err != nil {
		return Visit{}, fmt.Errorf("commit visit update transaction: %w", err)
	}

	return updatedVisit, nil
}

func insertVisitOutboxEvent(
	ctx context.Context,
	queries *sqlc.Queries,
	event outbox.Event,
	action string,
) error {
	aggregateID, err := uuid.Parse(event.AggregateID)
	if err != nil {
		return fmt.Errorf("parse visit %s outbox aggregate ID: %w", action, err)
	}

	if err := queries.CreateOutboxEvent(ctx, sqlc.CreateOutboxEventParams{
		AggregateType: event.AggregateType,
		AggregateID:   aggregateID,
		EventType:     event.EventType,
		Payload:       event.Payload,
	}); err != nil {
		return fmt.Errorf("insert visit %s outbox event: %w", action, err)
	}

	return nil
}

func mapUpdateVisitRow(row sqlc.UpdateVisitRow) Visit {
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
	}
}

func mapDeleteVisitRow(row sqlc.DeleteVisitRow) Visit {
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
	}
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

func optionalText(value *string) pgtype.Text {
	if value == nil {
		return pgtype.Text{}
	}

	return pgtype.Text{String: *value, Valid: true}
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
