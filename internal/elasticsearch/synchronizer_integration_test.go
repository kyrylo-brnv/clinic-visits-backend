//go:build integration

package elasticsearch

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPostgresDeltaSyncSnapshotLoaderBuildsEveryTargetIndex(t *testing.T) {
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
	var missingVisitID string
	if err := pool.QueryRow(t.Context(), "SELECT gen_random_uuid()::text").Scan(&missingVisitID); err != nil {
		t.Fatalf("create missing visit ID: %v", err)
	}

	loader := &postgresDeltaSyncSnapshotLoader{pool: pool}
	for _, testCase := range []struct {
		indexName string
		id        string
		assert    func(*testing.T, deltaSyncSnapshot)
	}{
		{
			indexName: DoctorsIndexName,
			id:        fixture.newDoctorID,
			assert: func(t *testing.T, snapshot deltaSyncSnapshot) {
				document, ok := snapshot.doctors[fixture.newDoctorID]
				if !ok || len(document.Visits) != 1 || document.Visits[0].ID != fixture.visitID {
					t.Fatalf("doctor document = %#v", document)
				}
			},
		},
		{
			indexName: PatientsIndexName,
			id:        fixture.newPatientID,
			assert: func(t *testing.T, snapshot deltaSyncSnapshot) {
				document, ok := snapshot.patients[fixture.newPatientID]
				if !ok || len(document.Visits) != 1 || document.Visits[0].ID != fixture.visitID {
					t.Fatalf("patient document = %#v", document)
				}
				doctor, ok := snapshot.doctors[fixture.newDoctorID]
				if !ok || len(doctor.Visits) != 1 || doctor.Visits[0].PatientFullName != "NewSync Patient" {
					t.Fatalf("affected doctor document = %#v", doctor)
				}
				clinic, ok := snapshot.clinics[fixture.newClinicID]
				if !ok || len(clinic.Visits) != 1 || clinic.Visits[0].PatientFullName != "NewSync Patient" {
					t.Fatalf("affected clinic document = %#v", clinic)
				}
			},
		},
		{
			indexName: ClinicsIndexName,
			id:        fixture.newClinicID,
			assert: func(t *testing.T, snapshot deltaSyncSnapshot) {
				document, ok := snapshot.clinics[fixture.newClinicID]
				if !ok || len(document.Visits) != 1 || document.Visits[0].ID != fixture.visitID {
					t.Fatalf("clinic document = %#v", document)
				}
			},
		},
	} {
		testCase := testCase
		t.Run(testCase.indexName, func(t *testing.T) {
			hints := newDeltaSyncHints()
			hints.visits[missingVisitID] = struct{}{}
			snapshot, err := loader.Load(t.Context(), testCase.indexName, []string{testCase.id}, hints)
			if err != nil {
				t.Fatalf("Load() error = %v", err)
			}
			testCase.assert(t, snapshot)
			if document, ok := snapshot.visits[fixture.visitID]; !ok || document.ID != fixture.visitID {
				t.Fatalf("dependent visit document = %#v, found=%t", document, ok)
			}
			if _, ok := snapshot.visitIDs[missingVisitID]; !ok {
				t.Fatalf("missing indexed visit ID was not retained for deletion: %#v", snapshot.visitIDs)
			}
			if _, ok := snapshot.visits[missingVisitID]; ok {
				t.Fatalf("missing visit unexpectedly loaded: %#v", snapshot.visits[missingVisitID])
			}
		})
	}

	hints := newDeltaSyncHints()
	hints.relations = relationIDs(
		[]string{fixture.oldDoctorID}, []string{fixture.oldPatientID}, []string{fixture.oldClinicID},
	)
	snapshot, err := loader.Load(t.Context(), VisitsIndexName, []string{fixture.visitID}, hints)
	if err != nil {
		t.Fatalf("load visit target: %v", err)
	}
	if snapshot.visits[fixture.visitID].DoctorID != fixture.newDoctorID {
		t.Fatalf("current visit document = %#v", snapshot.visits[fixture.visitID])
	}
	if len(snapshot.doctors[fixture.oldDoctorID].Visits) != 0 || len(snapshot.doctors[fixture.newDoctorID].Visits) != 1 {
		t.Fatalf("old/new doctor documents = %#v / %#v", snapshot.doctors[fixture.oldDoctorID], snapshot.doctors[fixture.newDoctorID])
	}
	if len(snapshot.patients[fixture.oldPatientID].Visits) != 0 || len(snapshot.patients[fixture.newPatientID].Visits) != 1 {
		t.Fatalf("old/new patient documents = %#v / %#v", snapshot.patients[fixture.oldPatientID], snapshot.patients[fixture.newPatientID])
	}
	if len(snapshot.clinics[fixture.oldClinicID].Visits) != 0 || len(snapshot.clinics[fixture.newClinicID].Visits) != 1 {
		t.Fatalf("old/new clinic documents = %#v / %#v", snapshot.clinics[fixture.oldClinicID], snapshot.clinics[fixture.newClinicID])
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
