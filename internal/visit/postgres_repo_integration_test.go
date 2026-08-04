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
