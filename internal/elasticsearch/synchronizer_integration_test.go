//go:build integration

package elasticsearch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresVisitSyncSnapshotLoaderRebuildsOldAndCurrentRelations(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	if err := pool.Ping(t.Context()); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)

	fixture := insertVisitSyncFixture(t, pool)
	t.Cleanup(func() { deleteVisitSyncFixture(t, pool, fixture) })

	loader := &postgresVisitSyncSnapshotLoader{pool: pool}
	snapshot, err := loader.Load(t.Context(), fixture.visitID, relationIDs(
		[]string{fixture.oldDoctorID},
		[]string{fixture.oldPatientID},
		[]string{fixture.oldClinicID},
	))
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if snapshot.visit == nil || snapshot.visit.ID != fixture.visitID ||
		snapshot.visit.DoctorID != fixture.newDoctorID ||
		snapshot.visit.PatientID != fixture.newPatientID ||
		snapshot.visit.ClinicID != fixture.newClinicID {
		t.Fatalf("current visit document = %#v", snapshot.visit)
	}
	if len(snapshot.doctors[fixture.oldDoctorID].Visits) != 0 ||
		len(snapshot.patients[fixture.oldPatientID].Visits) != 0 ||
		len(snapshot.clinics[fixture.oldClinicID].Visits) != 0 {
		t.Fatal("old related documents still contain the reassigned visit")
	}
	if visits := snapshot.doctors[fixture.newDoctorID].Visits; len(visits) != 1 || visits[0].ID != fixture.visitID {
		t.Fatalf("new doctor visits = %#v", visits)
	}
	if visits := snapshot.patients[fixture.newPatientID].Visits; len(visits) != 1 || visits[0].ID != fixture.visitID {
		t.Fatalf("new patient visits = %#v", visits)
	}
	if visits := snapshot.clinics[fixture.newClinicID].Visits; len(visits) != 1 || visits[0].ID != fixture.visitID {
		t.Fatalf("new clinic visits = %#v", visits)
	}
}

type visitSyncFixture struct {
	visitID                  string
	oldDoctorID, newDoctorID string
	oldPatientID             string
	newPatientID             string
	oldClinicID              string
	newClinicID              string
	specialtyID              string
}

func insertVisitSyncFixture(t *testing.T, pool *pgxpool.Pool) visitSyncFixture {
	t.Helper()

	fixture := visitSyncFixture{}
	if err := pool.QueryRow(t.Context(),
		"INSERT INTO specialties (name) VALUES ('es-sync-' || gen_random_uuid()::text) RETURNING id::text",
	).Scan(&fixture.specialtyID); err != nil {
		t.Fatalf("insert specialty fixture: %v", err)
	}
	for name, destination := range map[string]*string{
		"Old sync clinic": &fixture.oldClinicID,
		"New sync clinic": &fixture.newClinicID,
	} {
		if err := pool.QueryRow(t.Context(),
			"INSERT INTO clinics (name, address, time_zone) VALUES ($1, 'Test address', 'Europe/Kyiv') RETURNING id::text",
			name,
		).Scan(destination); err != nil {
			t.Fatalf("insert clinic fixture: %v", err)
		}
	}
	for firstName, destination := range map[string]*string{
		"OldSync": &fixture.oldPatientID,
		"NewSync": &fixture.newPatientID,
	} {
		if err := pool.QueryRow(t.Context(),
			"INSERT INTO patients (first_name, last_name, date_of_birth, gender) VALUES ($1, 'Patient', DATE '1990-01-01', 'Female') RETURNING id::text",
			firstName,
		).Scan(destination); err != nil {
			t.Fatalf("insert patient fixture: %v", err)
		}
	}
	for _, doctor := range []struct {
		name, clinicID string
		destination    *string
	}{
		{name: "Old Sync Doctor", clinicID: fixture.oldClinicID, destination: &fixture.oldDoctorID},
		{name: "New Sync Doctor", clinicID: fixture.newClinicID, destination: &fixture.newDoctorID},
	} {
		if err := pool.QueryRow(t.Context(),
			"INSERT INTO doctors (specialty_id, clinic_id, full_name) VALUES ($1, $2, $3) RETURNING id::text",
			fixture.specialtyID, doctor.clinicID, doctor.name,
		).Scan(doctor.destination); err != nil {
			t.Fatalf("insert doctor fixture: %v", err)
		}
	}

	visitStart := time.Now().UTC().Add(30 * 24 * time.Hour).Truncate(time.Second)
	if err := pool.QueryRow(t.Context(),
		`INSERT INTO visits (doctor_id, patient_id, clinic_id, visit_start_time, visit_end_time)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id::text`,
		fixture.newDoctorID, fixture.newPatientID, fixture.newClinicID,
		visitStart, visitStart.Add(time.Hour),
	).Scan(&fixture.visitID); err != nil {
		t.Fatalf("insert visit fixture: %v", err)
	}

	return fixture
}

func deleteVisitSyncFixture(t *testing.T, pool *pgxpool.Pool, fixture visitSyncFixture) {
	t.Helper()
	queries := []struct {
		statement string
		args      []any
	}{
		{statement: "DELETE FROM visits WHERE id = $1", args: []any{fixture.visitID}},
		{statement: "DELETE FROM doctors WHERE id = ANY($1::uuid[])", args: []any{[]string{fixture.oldDoctorID, fixture.newDoctorID}}},
		{statement: "DELETE FROM patients WHERE id = ANY($1::uuid[])", args: []any{[]string{fixture.oldPatientID, fixture.newPatientID}}},
		{statement: "DELETE FROM clinics WHERE id = ANY($1::uuid[])", args: []any{[]string{fixture.oldClinicID, fixture.newClinicID}}},
		{statement: "DELETE FROM specialties WHERE id = $1", args: []any{fixture.specialtyID}},
	}
	for _, query := range queries {
		if _, err := pool.Exec(context.Background(), query.statement, query.args...); err != nil {
			t.Errorf("clean up Elasticsearch sync fixture: %v", err)
		}
	}
}
