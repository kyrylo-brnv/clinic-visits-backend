//go:build integration

package visit

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
)

func TestCreateVisitRejectsDoctorFromAnotherClinic(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := t.Context()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer connection.Close(ctx)

	transaction, err := connection.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer transaction.Rollback(ctx)

	var specialtyID pgtype.UUID
	err = transaction.QueryRow(
		ctx,
		"INSERT INTO specialties (name) VALUES ($1) RETURNING id",
		"Integration Test Specialty",
	).Scan(&specialtyID)
	if err != nil {
		t.Fatalf("create specialty fixture: %v", err)
	}

	doctorClinicID := createClinicFixture(t, transaction, "Doctor Clinic")
	visitClinicID := createClinicFixture(t, transaction, "Visit Clinic")

	var doctorID pgtype.UUID
	err = transaction.QueryRow(
		ctx,
		`INSERT INTO doctors (specialty_id, clinic_id, full_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		specialtyID,
		doctorClinicID,
		"Integration Test Doctor",
	).Scan(&doctorID)
	if err != nil {
		t.Fatalf("create doctor fixture: %v", err)
	}

	var patientID pgtype.UUID
	err = transaction.QueryRow(
		ctx,
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		"Integration",
		"Patient",
		"1990-01-01",
		"other",
	).Scan(&patientID)
	if err != nil {
		t.Fatalf("create patient fixture: %v", err)
	}

	startTime := time.Now().UTC().Add(time.Hour)
	_, err = sqlc.New(transaction).CreateVisit(ctx, sqlc.CreateVisitParams{
		DoctorID:       doctorID,
		PatientID:      patientID,
		ClinicID:       visitClinicID,
		VisitStartTime: pgtype.Timestamptz{Time: startTime, Valid: true},
		VisitEndTime:   pgtype.Timestamptz{Time: startTime.Add(time.Hour), Valid: true},
	})

	var postgresError *pgconn.PgError
	if !errors.As(err, &postgresError) {
		t.Fatalf("expected PostgreSQL constraint error, got %v", err)
	}
	if postgresError.ConstraintName != "visits_doctor_clinic_fkey" {
		t.Fatalf(
			"expected constraint %q, got %q",
			"visits_doctor_clinic_fkey",
			postgresError.ConstraintName,
		)
	}
}

func TestVisitStatusDatabaseValidation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := t.Context()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer connection.Close(ctx)

	transaction, err := connection.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer transaction.Rollback(ctx)

	doctorID, patientID, _, clinicID := createVisitScheduleFixtures(t, transaction)
	startTime := time.Now().UTC().Add(72 * time.Hour).Truncate(time.Microsecond)

	var defaultStatus string
	err = transaction.QueryRow(
		ctx,
		`INSERT INTO visits (
			doctor_id,
			patient_id,
			clinic_id,
			visit_start_time,
			visit_end_time
		 ) VALUES ($1, $2, $3, $4, $5)
		 RETURNING status`,
		doctorID,
		patientID,
		clinicID,
		startTime,
		startTime.Add(time.Hour),
	).Scan(&defaultStatus)
	if err != nil {
		t.Fatalf("insert visit with default status: %v", err)
	}
	if defaultStatus != "SCHEDULED" {
		t.Fatalf("expected default status %q, got %q", "SCHEDULED", defaultStatus)
	}

	validStatuses := []string{"SCHEDULED", "IN_PROGRESS", "CLOSED", "CANCELED"}
	for index, status := range validStatuses {
		t.Run("accepts "+status, func(t *testing.T) {
			visitStart := startTime.Add(time.Duration(index+1) * 2 * time.Hour)
			var persistedStatus string
			err := transaction.QueryRow(
				ctx,
				`INSERT INTO visits (
					doctor_id,
					patient_id,
					clinic_id,
					visit_start_time,
					visit_end_time,
					status
				 ) VALUES ($1, $2, $3, $4, $5, $6)
				 RETURNING status`,
				doctorID,
				patientID,
				clinicID,
				visitStart,
				visitStart.Add(time.Hour),
				status,
			).Scan(&persistedStatus)
			if err != nil {
				t.Fatalf("insert visit with status %q: %v", status, err)
			}
			if persistedStatus != status {
				t.Fatalf("expected persisted status %q, got %q", status, persistedStatus)
			}
		})
	}

	t.Run("rejects unsupported status", func(t *testing.T) {
		savepoint, err := transaction.Begin(ctx)
		if err != nil {
			t.Fatalf("create unsupported status savepoint: %v", err)
		}

		visitStart := startTime.Add(10 * time.Hour)
		_, insertErr := savepoint.Exec(
			ctx,
			`INSERT INTO visits (
				doctor_id,
				patient_id,
				clinic_id,
				visit_start_time,
				visit_end_time,
				status
			 ) VALUES ($1, $2, $3, $4, $5, $6)`,
			doctorID,
			patientID,
			clinicID,
			visitStart,
			visitStart.Add(time.Hour),
			"CREATED",
		)
		if err := savepoint.Rollback(ctx); err != nil {
			t.Fatalf("rollback unsupported status savepoint: %v", err)
		}

		var postgresError *pgconn.PgError
		if !errors.As(insertErr, &postgresError) {
			t.Fatalf("expected PostgreSQL constraint error, got %v", insertErr)
		}
		if postgresError.ConstraintName != "visits_status_check" {
			t.Fatalf("expected constraint %q, got %q", "visits_status_check", postgresError.ConstraintName)
		}
	})

	t.Run("rejects null status", func(t *testing.T) {
		savepoint, err := transaction.Begin(ctx)
		if err != nil {
			t.Fatalf("create null status savepoint: %v", err)
		}

		visitStart := startTime.Add(12 * time.Hour)
		_, insertErr := savepoint.Exec(
			ctx,
			`INSERT INTO visits (
				doctor_id,
				patient_id,
				clinic_id,
				visit_start_time,
				visit_end_time,
				status
			 ) VALUES ($1, $2, $3, $4, $5, NULL)`,
			doctorID,
			patientID,
			clinicID,
			visitStart,
			visitStart.Add(time.Hour),
		)
		if err := savepoint.Rollback(ctx); err != nil {
			t.Fatalf("rollback null status savepoint: %v", err)
		}

		var postgresError *pgconn.PgError
		if !errors.As(insertErr, &postgresError) {
			t.Fatalf("expected PostgreSQL not-null error, got %v", insertErr)
		}
		if postgresError.Code != "23502" || postgresError.ColumnName != "status" {
			t.Fatalf(
				"expected not-null violation for status, got code %q column %q",
				postgresError.Code,
				postgresError.ColumnName,
			)
		}
	})
}

func TestVisitTimeConflictValidation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := t.Context()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer connection.Close(ctx)

	transaction, err := connection.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer transaction.Rollback(ctx)

	doctorID, firstPatientID, secondPatientID, clinicID := createVisitScheduleFixtures(t, transaction)
	repository := &PostgresRepository{queries: sqlc.New(transaction)}
	startTime := time.Now().UTC().Add(24 * time.Hour).Truncate(time.Microsecond)

	firstVisit, err := repository.CreateVisit(ctx, CreateVisitRequest{
		DoctorID:       doctorID.String(),
		PatientID:      firstPatientID.String(),
		ClinicID:       clinicID.String(),
		VisitStartTime: startTime,
		VisitEndTime:   startTime.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create first visit: %v", err)
	}

	conflicts := []struct {
		name  string
		start time.Time
		end   time.Time
	}{
		{
			name:  "exact overlap",
			start: startTime,
			end:   startTime.Add(time.Hour),
		},
		{
			name:  "partial overlap",
			start: startTime.Add(30 * time.Minute),
			end:   startTime.Add(90 * time.Minute),
		},
		{
			name:  "contained overlap",
			start: startTime.Add(15 * time.Minute),
			end:   startTime.Add(45 * time.Minute),
		},
	}

	for _, test := range conflicts {
		t.Run(test.name, func(t *testing.T) {
			savepoint, err := transaction.Begin(ctx)
			if err != nil {
				t.Fatalf("create savepoint: %v", err)
			}

			conflictRepository := &PostgresRepository{queries: sqlc.New(savepoint)}
			_, createErr := conflictRepository.CreateVisit(ctx, CreateVisitRequest{
				DoctorID:       doctorID.String(),
				PatientID:      secondPatientID.String(),
				ClinicID:       clinicID.String(),
				VisitStartTime: test.start,
				VisitEndTime:   test.end,
			})
			if err := savepoint.Rollback(ctx); err != nil {
				t.Fatalf("rollback conflict savepoint: %v", err)
			}

			if !errors.Is(createErr, ErrVisitTimeConflict) {
				t.Fatalf("expected %v, got %v", ErrVisitTimeConflict, createErr)
			}
		})
	}

	adjacentVisit, err := repository.CreateVisit(ctx, CreateVisitRequest{
		DoctorID:       doctorID.String(),
		PatientID:      secondPatientID.String(),
		ClinicID:       clinicID.String(),
		VisitStartTime: startTime.Add(time.Hour),
		VisitEndTime:   startTime.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("expected adjacent visit to be allowed: %v", err)
	}

	conflictingStart := startTime.Add(30 * time.Minute)
	conflictingEnd := startTime.Add(90 * time.Minute)
	updateSavepoint, err := transaction.Begin(ctx)
	if err != nil {
		t.Fatalf("create update savepoint: %v", err)
	}
	conflictRepository := &PostgresRepository{queries: sqlc.New(updateSavepoint)}
	_, updateErr := conflictRepository.UpdateVisit(ctx, UpdateVisitRequest{
		VisitID:        adjacentVisit.ID,
		VisitStartTime: &conflictingStart,
		VisitEndTime:   &conflictingEnd,
	})
	if err := updateSavepoint.Rollback(ctx); err != nil {
		t.Fatalf("rollback update savepoint: %v", err)
	}
	if !errors.Is(updateErr, ErrVisitTimeConflict) {
		t.Fatalf("expected %v for overlapping update, got %v", ErrVisitTimeConflict, updateErr)
	}

	safeStart := startTime.Add(-30 * time.Minute)
	updatedVisit, err := repository.UpdateVisit(ctx, UpdateVisitRequest{
		VisitID:        firstVisit.ID,
		VisitStartTime: &safeStart,
	})
	if err != nil {
		t.Fatalf("expected visit update without self-overlap to be allowed: %v", err)
	}
	if !updatedVisit.VisitStartTime.Equal(safeStart) {
		t.Fatalf("expected updated start time %v, got %v", safeStart, updatedVisit.VisitStartTime)
	}
}

func TestPatientTimeConflictValidation(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := t.Context()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer connection.Close(ctx)

	transaction, err := connection.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer transaction.Rollback(ctx)

	firstDoctorID, firstClinicID, secondDoctorID, secondClinicID, patientID :=
		createPatientScheduleFixtures(t, transaction)
	repository := &PostgresRepository{queries: sqlc.New(transaction)}
	startTime := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Microsecond)

	firstVisit, err := repository.CreateVisit(ctx, CreateVisitRequest{
		DoctorID:       firstDoctorID.String(),
		PatientID:      patientID.String(),
		ClinicID:       firstClinicID.String(),
		VisitStartTime: startTime,
		VisitEndTime:   startTime.Add(time.Hour),
	})
	if err != nil {
		t.Fatalf("create first patient visit: %v", err)
	}

	createSavepoint, err := transaction.Begin(ctx)
	if err != nil {
		t.Fatalf("create overlapping visit savepoint: %v", err)
	}
	conflictRepository := &PostgresRepository{queries: sqlc.New(createSavepoint)}
	_, createErr := conflictRepository.CreateVisit(ctx, CreateVisitRequest{
		DoctorID:       secondDoctorID.String(),
		PatientID:      patientID.String(),
		ClinicID:       secondClinicID.String(),
		VisitStartTime: startTime.Add(30 * time.Minute),
		VisitEndTime:   startTime.Add(90 * time.Minute),
	})
	if err := createSavepoint.Rollback(ctx); err != nil {
		t.Fatalf("rollback overlapping visit savepoint: %v", err)
	}
	if !errors.Is(createErr, ErrPatientTimeConflict) {
		t.Fatalf("expected %v for overlapping patient create, got %v", ErrPatientTimeConflict, createErr)
	}

	adjacentVisit, err := repository.CreateVisit(ctx, CreateVisitRequest{
		DoctorID:       secondDoctorID.String(),
		PatientID:      patientID.String(),
		ClinicID:       secondClinicID.String(),
		VisitStartTime: startTime.Add(time.Hour),
		VisitEndTime:   startTime.Add(2 * time.Hour),
	})
	if err != nil {
		t.Fatalf("expected adjacent patient visit to be allowed: %v", err)
	}

	conflictingStart := startTime.Add(30 * time.Minute)
	conflictingEnd := startTime.Add(90 * time.Minute)
	updateSavepoint, err := transaction.Begin(ctx)
	if err != nil {
		t.Fatalf("create overlapping update savepoint: %v", err)
	}
	conflictRepository = &PostgresRepository{queries: sqlc.New(updateSavepoint)}
	_, updateErr := conflictRepository.UpdateVisit(ctx, UpdateVisitRequest{
		VisitID:        adjacentVisit.ID,
		VisitStartTime: &conflictingStart,
		VisitEndTime:   &conflictingEnd,
	})
	if err := updateSavepoint.Rollback(ctx); err != nil {
		t.Fatalf("rollback overlapping update savepoint: %v", err)
	}
	if !errors.Is(updateErr, ErrPatientTimeConflict) {
		t.Fatalf("expected %v for overlapping patient update, got %v", ErrPatientTimeConflict, updateErr)
	}

	safeStart := startTime.Add(-30 * time.Minute)
	updatedVisit, err := repository.UpdateVisit(ctx, UpdateVisitRequest{
		VisitID:        firstVisit.ID,
		VisitStartTime: &safeStart,
	})
	if err != nil {
		t.Fatalf("expected non-overlapping patient update to be allowed: %v", err)
	}
	if !updatedVisit.VisitStartTime.Equal(safeStart) {
		t.Fatalf("expected updated start time %v, got %v", safeStart, updatedVisit.VisitStartTime)
	}
}

func TestUpdateAndDeleteVisit(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	ctx := t.Context()
	connection, err := pgx.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	defer connection.Close(ctx)

	transaction, err := connection.Begin(ctx)
	if err != nil {
		t.Fatalf("begin transaction: %v", err)
	}
	defer transaction.Rollback(ctx)

	var specialtyID pgtype.UUID
	err = transaction.QueryRow(
		ctx,
		"INSERT INTO specialties (name) VALUES ($1) RETURNING id",
		"Update Delete Test Specialty",
	).Scan(&specialtyID)
	if err != nil {
		t.Fatalf("create specialty fixture: %v", err)
	}

	clinicID := createClinicFixture(t, transaction, "Update Delete Clinic")
	otherClinicID := createClinicFixture(t, transaction, "Update Delete Other Clinic")

	var doctorID pgtype.UUID
	err = transaction.QueryRow(
		ctx,
		`INSERT INTO doctors (specialty_id, clinic_id, full_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		specialtyID,
		clinicID,
		"Update Delete Test Doctor",
	).Scan(&doctorID)
	if err != nil {
		t.Fatalf("create doctor fixture: %v", err)
	}

	var patientID pgtype.UUID
	err = transaction.QueryRow(
		ctx,
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		"Update",
		"Patient",
		"1990-01-01",
		"other",
	).Scan(&patientID)
	if err != nil {
		t.Fatalf("create patient fixture: %v", err)
	}

	startTime := time.Now().UTC().Add(time.Hour).Truncate(time.Microsecond)
	created, err := sqlc.New(transaction).CreateVisit(ctx, sqlc.CreateVisitParams{
		DoctorID:       doctorID,
		PatientID:      patientID,
		ClinicID:       clinicID,
		VisitStartTime: pgtype.Timestamptz{Time: startTime, Valid: true},
		VisitEndTime:   pgtype.Timestamptz{Time: startTime.Add(time.Hour), Valid: true},
	})
	if err != nil {
		t.Fatalf("create visit fixture: %v", err)
	}

	repository := &PostgresRepository{queries: sqlc.New(transaction)}
	newStatus := StatusClosed
	updated, err := repository.UpdateVisit(ctx, UpdateVisitRequest{
		VisitID: created.ID,
		Status:  &newStatus,
	})
	if err != nil {
		t.Fatalf("update visit status: %v", err)
	}
	if updated.Status != newStatus {
		t.Fatalf("expected status %q after status-only update, got %q", newStatus, updated.Status)
	}
	if updated.DoctorID != doctorID.String() || updated.PatientID != patientID.String() || updated.ClinicID != clinicID.String() {
		t.Fatalf("expected status-only update to preserve relationships, got %+v", updated)
	}

	newEndTime := startTime.Add(2 * time.Hour)
	updated, err = repository.UpdateVisit(ctx, UpdateVisitRequest{
		VisitID:      created.ID,
		VisitEndTime: &newEndTime,
	})
	if err != nil {
		t.Fatalf("update visit: %v", err)
	}
	if updated.DoctorID != doctorID.String() || updated.PatientID != patientID.String() || updated.ClinicID != clinicID.String() {
		t.Fatalf("expected omitted relationships to be preserved, got %+v", updated)
	}
	if !updated.VisitStartTime.Equal(startTime) || !updated.VisitEndTime.Equal(newEndTime) {
		t.Fatalf("expected partial time update, got start %v end %v", updated.VisitStartTime, updated.VisitEndTime)
	}
	if updated.Status != newStatus {
		t.Fatalf("expected time-only update to preserve status %q, got %q", newStatus, updated.Status)
	}

	invalidStartTime := newEndTime.Add(time.Hour)
	invalidTimeTransaction, err := transaction.Begin(ctx)
	if err != nil {
		t.Fatalf("create invalid time range savepoint: %v", err)
	}
	invalidTimeRepository := &PostgresRepository{queries: sqlc.New(invalidTimeTransaction)}
	_, updateErr := invalidTimeRepository.UpdateVisit(ctx, UpdateVisitRequest{
		VisitID:        created.ID,
		VisitStartTime: &invalidStartTime,
	})
	if err := invalidTimeTransaction.Rollback(ctx); err != nil {
		t.Fatalf("rollback invalid time range savepoint: %v", err)
	}
	if !errors.Is(updateErr, ErrInvalidTimeRange) {
		t.Fatalf("expected %v for invalid final range, got %v", ErrInvalidTimeRange, updateErr)
	}

	otherClinic := otherClinicID.String()
	mismatchTransaction, err := transaction.Begin(ctx)
	if err != nil {
		t.Fatalf("create doctor-clinic mismatch savepoint: %v", err)
	}
	mismatchRepository := &PostgresRepository{queries: sqlc.New(mismatchTransaction)}
	_, updateErr = mismatchRepository.UpdateVisit(ctx, UpdateVisitRequest{
		VisitID:  created.ID,
		ClinicID: &otherClinic,
	})
	if err := mismatchTransaction.Rollback(ctx); err != nil {
		t.Fatalf("rollback doctor-clinic mismatch savepoint: %v", err)
	}
	if !errors.Is(updateErr, ErrDoctorClinicMismatch) {
		t.Fatalf("expected %v, got %v", ErrDoctorClinicMismatch, updateErr)
	}

	if err := repository.DeleteVisit(ctx, DeleteVisitRequest{VisitID: created.ID}); err != nil {
		t.Fatalf("delete visit: %v", err)
	}
	if err := repository.DeleteVisit(ctx, DeleteVisitRequest{VisitID: created.ID}); !errors.Is(err, ErrVisitNotFound) {
		t.Fatalf("expected %v after deleting visit, got %v", ErrVisitNotFound, err)
	}
}

func createClinicFixture(
	t *testing.T,
	transaction pgx.Tx,
	name string,
) pgtype.UUID {
	t.Helper()

	var clinicID pgtype.UUID
	err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO clinics (name, address, time_zone)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		name,
		"Integration Test Address",
		"UTC",
	).Scan(&clinicID)
	if err != nil {
		t.Fatalf("create clinic fixture: %v", err)
	}

	return clinicID
}

func createVisitScheduleFixtures(
	t *testing.T,
	transaction pgx.Tx,
) (pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID) {
	t.Helper()

	ctx := t.Context()
	var specialtyID pgtype.UUID
	if err := transaction.QueryRow(
		ctx,
		"INSERT INTO specialties (name) VALUES ($1) RETURNING id",
		"Visit Schedule Validation Specialty",
	).Scan(&specialtyID); err != nil {
		t.Fatalf("create specialty fixture: %v", err)
	}

	clinicID := createClinicFixture(t, transaction, "Visit Schedule Validation Clinic")

	var doctorID pgtype.UUID
	if err := transaction.QueryRow(
		ctx,
		`INSERT INTO doctors (specialty_id, clinic_id, full_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		specialtyID,
		clinicID,
		"Visit Schedule Validation Doctor",
	).Scan(&doctorID); err != nil {
		t.Fatalf("create doctor fixture: %v", err)
	}

	var firstPatientID pgtype.UUID
	if err := transaction.QueryRow(
		ctx,
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		"Schedule",
		"Patient",
		"1990-01-01",
		"other",
	).Scan(&firstPatientID); err != nil {
		t.Fatalf("create patient fixture: %v", err)
	}

	var secondPatientID pgtype.UUID
	if err := transaction.QueryRow(
		ctx,
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		"Other Schedule",
		"Patient",
		"1991-01-01",
		"other",
	).Scan(&secondPatientID); err != nil {
		t.Fatalf("create second patient fixture: %v", err)
	}

	return doctorID, firstPatientID, secondPatientID, clinicID
}

func createPatientScheduleFixtures(
	t *testing.T,
	transaction pgx.Tx,
) (pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID, pgtype.UUID) {
	t.Helper()

	ctx := t.Context()
	var specialtyID pgtype.UUID
	if err := transaction.QueryRow(
		ctx,
		"INSERT INTO specialties (name) VALUES ($1) RETURNING id",
		"Patient Schedule Validation Specialty",
	).Scan(&specialtyID); err != nil {
		t.Fatalf("create specialty fixture: %v", err)
	}

	firstClinicID := createClinicFixture(t, transaction, "Patient Schedule First Clinic")
	secondClinicID := createClinicFixture(t, transaction, "Patient Schedule Second Clinic")

	var firstDoctorID pgtype.UUID
	if err := transaction.QueryRow(
		ctx,
		`INSERT INTO doctors (specialty_id, clinic_id, full_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		specialtyID,
		firstClinicID,
		"Patient Schedule First Doctor",
	).Scan(&firstDoctorID); err != nil {
		t.Fatalf("create first doctor fixture: %v", err)
	}

	var secondDoctorID pgtype.UUID
	if err := transaction.QueryRow(
		ctx,
		`INSERT INTO doctors (specialty_id, clinic_id, full_name)
		 VALUES ($1, $2, $3)
		 RETURNING id`,
		specialtyID,
		secondClinicID,
		"Patient Schedule Second Doctor",
	).Scan(&secondDoctorID); err != nil {
		t.Fatalf("create second doctor fixture: %v", err)
	}

	var patientID pgtype.UUID
	if err := transaction.QueryRow(
		ctx,
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ($1, $2, $3, $4)
		 RETURNING id`,
		"Patient Schedule",
		"Conflict",
		"1990-01-01",
		"other",
	).Scan(&patientID); err != nil {
		t.Fatalf("create patient fixture: %v", err)
	}

	return firstDoctorID, firstClinicID, secondDoctorID, secondClinicID, patientID
}
