//go:build integration

package elasticsearch

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/config"
	"github.com/smithautotest/clinic-visits/internal/outbox"
)

func TestAutomaticOutboxSyncUpdatesEveryReadModelAndRetries(t *testing.T) {
	pool, client := openAutomaticSyncInfrastructure(t)
	drainAutomaticSyncOutbox(t, pool, client)

	fixture := automaticSyncFixture{}
	fixture.specialtyID = insertAutomaticSyncSpecialty(t, pool)
	cleanupAutomaticSyncFixture(t, pool, client, &fixture)

	transaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin committed aggregate transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())
	if err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO clinics (name, address, time_zone)
		 VALUES ('Automatic Sync Clinic', 'Initial address', 'Europe/Kyiv')
		 RETURNING id::text`,
	).Scan(&fixture.clinicID); err != nil {
		t.Fatalf("insert committed clinic: %v", err)
	}
	if err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ('Automatic', 'Patient', DATE '1990-01-01', 'Female')
		 RETURNING id::text`,
	).Scan(&fixture.patientID); err != nil {
		t.Fatalf("insert committed patient: %v", err)
	}
	if err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO doctors (specialty_id, clinic_id, full_name)
		 VALUES ($1, $2, 'Automatic Sync Doctor')
		 RETURNING id::text`,
		fixture.specialtyID,
		fixture.clinicID,
	).Scan(&fixture.doctorID); err != nil {
		t.Fatalf("insert committed doctor: %v", err)
	}
	if err := transaction.Commit(t.Context()); err != nil {
		t.Fatalf("commit aggregate transaction: %v", err)
	}

	rolledBackTransaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin rolled-back aggregate transaction: %v", err)
	}
	defer rolledBackTransaction.Rollback(t.Context())
	if err := rolledBackTransaction.QueryRow(
		t.Context(),
		`INSERT INTO clinics (name, address, time_zone)
		 VALUES ('Rolled Back Sync Clinic', 'Rolled back address', 'Europe/Kyiv')
		 RETURNING id::text`,
	).Scan(&fixture.rolledBackClinicID); err != nil {
		t.Fatalf("insert rolled-back clinic: %v", err)
	}
	if err := rolledBackTransaction.QueryRow(
		t.Context(),
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ('RolledBack', 'Patient', DATE '1991-01-01', 'Male')
		 RETURNING id::text`,
	).Scan(&fixture.rolledBackPatientID); err != nil {
		t.Fatalf("insert rolled-back patient: %v", err)
	}
	if err := rolledBackTransaction.QueryRow(
		t.Context(),
		`INSERT INTO doctors (specialty_id, clinic_id, full_name)
		 VALUES ($1, $2, 'Rolled Back Sync Doctor')
		 RETURNING id::text`,
		fixture.specialtyID,
		fixture.rolledBackClinicID,
	).Scan(&fixture.rolledBackDoctorID); err != nil {
		t.Fatalf("insert rolled-back doctor: %v", err)
	}
	if err := rolledBackTransaction.Rollback(t.Context()); err != nil {
		t.Fatalf("roll back aggregate transaction: %v", err)
	}
	assertAutomaticSyncEventCount(t, pool, fixture.rolledBackAggregateIDs(), 0)

	processor := outbox.NewProcessor(pool, NewOutboxEventConsumer(pool, client))
	processAutomaticSyncBatch(t, processor, 3)
	assertAutomaticSyncEntityDocuments(t, client, fixture, "Automatic Sync Doctor", "Patient", "Automatic Sync Clinic")
	assertAutomaticSyncDocumentsMissing(t, client, fixture.rolledBackDocuments())

	visitStart := time.Now().UTC().Add(30 * 24 * time.Hour).Truncate(time.Second)
	fixture.visitID = insertAutomaticSyncVisit(t, pool, fixture, visitStart)
	processAutomaticSyncBatch(t, processor, 1)
	assertAutomaticSyncVisitDocuments(t, client, fixture, "Automatic Sync Doctor", "Patient", "Automatic Sync Clinic")

	transaction, err = pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin aggregate update transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())
	if _, err := transaction.Exec(t.Context(), "UPDATE doctors SET full_name = 'Updated Sync Doctor' WHERE id = $1", fixture.doctorID); err != nil {
		t.Fatalf("update doctor: %v", err)
	}
	if _, err := transaction.Exec(t.Context(), "UPDATE patients SET last_name = 'UpdatedPatient' WHERE id = $1", fixture.patientID); err != nil {
		t.Fatalf("update patient: %v", err)
	}
	if _, err := transaction.Exec(t.Context(), "UPDATE clinics SET name = 'Updated Sync Clinic' WHERE id = $1", fixture.clinicID); err != nil {
		t.Fatalf("update clinic: %v", err)
	}
	if err := transaction.Commit(t.Context()); err != nil {
		t.Fatalf("commit aggregate updates: %v", err)
	}
	processAutomaticSyncBatch(t, processor, 3)
	assertAutomaticSyncEntityDocuments(t, client, fixture, "Updated Sync Doctor", "UpdatedPatient", "Updated Sync Clinic")
	assertAutomaticSyncVisitDocuments(t, client, fixture, "Updated Sync Doctor", "UpdatedPatient", "Updated Sync Clinic")

	if _, err := pool.Exec(t.Context(), "UPDATE patients SET last_name = 'RetriedPatient' WHERE id = $1", fixture.patientID); err != nil {
		t.Fatalf("update patient for retry: %v", err)
	}
	pendingEvent := loadAutomaticSyncPendingEvent(t, pool, fixture.patientID)
	failedClient, err := NewClient(&config.ElasticsearchConfig{URL: "http://127.0.0.1:1"})
	if err != nil {
		t.Fatalf("create unavailable Elasticsearch client: %v", err)
	}
	failedProcessor := outbox.NewProcessor(pool, NewOutboxEventConsumer(pool, failedClient))
	processed, err := failedProcessor.ProcessBatch(t.Context(), 100)
	if err == nil {
		t.Fatal("expected Elasticsearch connection failure")
	}
	if processed != 0 {
		t.Fatalf("failed Elasticsearch batch processed %d events, want 0", processed)
	}
	assertAutomaticSyncEventPending(t, pool, pendingEvent.ID, true)

	processAutomaticSyncBatch(t, processor, 1)
	assertAutomaticSyncEventPending(t, pool, pendingEvent.ID, false)
	consumer := NewOutboxEventConsumer(pool, client)
	if err := consumer(t.Context(), pendingEvent); err != nil {
		t.Fatalf("redeliver processed event: %v", err)
	}
	if err := consumer(t.Context(), pendingEvent); err != nil {
		t.Fatalf("redeliver duplicate event: %v", err)
	}
	assertAutomaticSyncVisitDocuments(t, client, fixture, "Updated Sync Doctor", "RetriedPatient", "Updated Sync Clinic")

	deleteAutomaticSyncVisit(t, pool, fixture.visitID)
	processAutomaticSyncBatch(t, processor, 1)
	assertAutomaticSyncDocumentsMissing(t, client, []automaticSyncDocument{{indexName: VisitsIndexName, id: fixture.visitID}})
	assertAutomaticSyncEntityVisitsEmpty(t, client, fixture)

	transaction, err = pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin aggregate delete transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())
	if _, err := transaction.Exec(t.Context(), "DELETE FROM doctors WHERE id = $1", fixture.doctorID); err != nil {
		t.Fatalf("delete doctor: %v", err)
	}
	if _, err := transaction.Exec(t.Context(), "DELETE FROM patients WHERE id = $1", fixture.patientID); err != nil {
		t.Fatalf("delete patient: %v", err)
	}
	if _, err := transaction.Exec(t.Context(), "DELETE FROM clinics WHERE id = $1", fixture.clinicID); err != nil {
		t.Fatalf("delete clinic: %v", err)
	}
	if err := transaction.Commit(t.Context()); err != nil {
		t.Fatalf("commit aggregate deletes: %v", err)
	}
	processAutomaticSyncBatch(t, processor, 3)
	assertAutomaticSyncDocumentsMissing(t, client, fixture.primaryDocuments())
}

func TestAutomaticOutboxSyncPreservesAggregateOrderAcrossUpdateAndDelete(t *testing.T) {
	pool, client := openAutomaticSyncInfrastructure(t)
	drainAutomaticSyncOutbox(t, pool, client)

	var patientID string
	transaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin ordered mutation transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())
	if err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ('Ordered', 'Initial', DATE '1992-01-01', 'Female')
		 RETURNING id::text`,
	).Scan(&patientID); err != nil {
		t.Fatalf("insert ordered patient: %v", err)
	}
	if _, err := transaction.Exec(t.Context(), "UPDATE patients SET last_name = 'Newest' WHERE id = $1", patientID); err != nil {
		t.Fatalf("update ordered patient: %v", err)
	}
	if err := transaction.Commit(t.Context()); err != nil {
		t.Fatalf("commit ordered mutations: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM patients WHERE id = $1", patientID)
		_, _ = pool.Exec(context.Background(), "DELETE FROM outbox_events WHERE aggregate_id = $1", patientID)
		_ = client.DeleteDocument(context.Background(), PatientsIndexName, patientID)
	})

	eventIDs := loadAutomaticSyncEventIDs(t, pool, patientID)
	if len(eventIDs) != 2 {
		t.Fatalf("ordered patient event IDs = %v, want create and update", eventIDs)
	}
	failure := errors.New("oldest event failed")
	delivered := make([]string, 0, 2)
	failingProcessor := outbox.NewProcessor(pool, func(_ context.Context, event outbox.PersistedEvent) error {
		delivered = append(delivered, event.ID)
		return failure
	})
	processed, err := failingProcessor.ProcessBatch(t.Context(), 100)
	if !errors.Is(err, failure) || processed != 0 {
		t.Fatalf("failed ordered batch processed=%d error=%v", processed, err)
	}
	if !reflect.DeepEqual(delivered, eventIDs[:1]) {
		t.Fatalf("failed ordered deliveries = %v, want only %v", delivered, eventIDs[:1])
	}
	assertAutomaticSyncEventPending(t, pool, eventIDs[0], true)
	assertAutomaticSyncEventPending(t, pool, eventIDs[1], true)

	processor := outbox.NewProcessor(pool, NewOutboxEventConsumer(pool, client))
	processAutomaticSyncBatch(t, processor, 1)
	var document PatientDocument
	found, err := client.GetDocument(t.Context(), PatientsIndexName, patientID, &document)
	if err != nil {
		t.Fatalf("load patient after oldest event: %v", err)
	}
	if !found || document.LastName != "Newest" {
		t.Fatalf("patient after oldest event = found:%t document:%+v", found, document)
	}
	assertAutomaticSyncEventPending(t, pool, eventIDs[1], true)

	if _, err := pool.Exec(t.Context(), "DELETE FROM patients WHERE id = $1", patientID); err != nil {
		t.Fatalf("delete ordered patient: %v", err)
	}
	eventIDs = loadAutomaticSyncEventIDs(t, pool, patientID)
	if len(eventIDs) != 3 {
		t.Fatalf("ordered patient event IDs = %v, want create, update, and delete", eventIDs)
	}
	processAutomaticSyncBatch(t, processor, 1)
	assertAutomaticSyncDocumentsMissing(t, client, []automaticSyncDocument{{indexName: PatientsIndexName, id: patientID}})
	assertAutomaticSyncEventPending(t, pool, eventIDs[2], true)
	processAutomaticSyncBatch(t, processor, 1)
	assertAutomaticSyncDocumentsMissing(t, client, []automaticSyncDocument{{indexName: PatientsIndexName, id: patientID}})
}

type automaticSyncFixture struct {
	specialtyID         string
	clinicID            string
	patientID           string
	doctorID            string
	visitID             string
	rolledBackClinicID  string
	rolledBackPatientID string
	rolledBackDoctorID  string
}

type automaticSyncDocument struct {
	indexName string
	id        string
}

func openAutomaticSyncInfrastructure(t *testing.T) (*pgxpool.Pool, *Client) {
	t.Helper()
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	elasticsearchURL := os.Getenv("ELASTICSEARCH_URL")
	if elasticsearchURL == "" {
		t.Skip("ELASTICSEARCH_URL is not set")
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

	client, err := NewClient(&config.ElasticsearchConfig{URL: elasticsearchURL})
	if err != nil {
		t.Fatalf("create Elasticsearch client: %v", err)
	}
	if err := client.Initialize(t.Context()); err != nil {
		t.Fatalf("initialize Elasticsearch: %v", err)
	}
	return pool, client
}

func insertAutomaticSyncSpecialty(t *testing.T, pool *pgxpool.Pool) string {
	t.Helper()
	var id string
	if err := pool.QueryRow(
		t.Context(),
		"INSERT INTO specialties (name) VALUES ('automatic-sync-' || gen_random_uuid()::text) RETURNING id::text",
	).Scan(&id); err != nil {
		t.Fatalf("insert specialty: %v", err)
	}
	return id
}

func insertAutomaticSyncVisit(
	t *testing.T,
	pool *pgxpool.Pool,
	fixture automaticSyncFixture,
	visitStart time.Time,
) string {
	t.Helper()
	transaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin visit creation transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())

	var visitID string
	if err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO visits (doctor_id, patient_id, clinic_id, visit_start_time, visit_end_time)
		 VALUES ($1, $2, $3, $4, $5)
		 RETURNING id::text`,
		fixture.doctorID,
		fixture.patientID,
		fixture.clinicID,
		visitStart,
		visitStart.Add(time.Hour),
	).Scan(&visitID); err != nil {
		t.Fatalf("insert visit: %v", err)
	}
	if _, err := transaction.Exec(
		t.Context(),
		`INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		 SELECT 'visit', id, 'visit.created', to_jsonb(visits)
		 FROM visits
		 WHERE id = $1`,
		visitID,
	); err != nil {
		t.Fatalf("insert visit created outbox event: %v", err)
	}
	if err := transaction.Commit(t.Context()); err != nil {
		t.Fatalf("commit visit creation transaction: %v", err)
	}
	return visitID
}

func deleteAutomaticSyncVisit(t *testing.T, pool *pgxpool.Pool, visitID string) {
	t.Helper()
	transaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin visit deletion transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())
	result, err := transaction.Exec(
		t.Context(),
		`WITH deleted AS (
		    DELETE FROM visits
		    WHERE id = $1
		    RETURNING *
		)
		INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		SELECT 'visit', id, 'visit.deleted', to_jsonb(deleted)
		FROM deleted`,
		visitID,
	)
	if err != nil {
		t.Fatalf("delete visit and insert outbox event: %v", err)
	}
	if result.RowsAffected() != 1 {
		t.Fatalf("delete visit and insert outbox event affected %d rows, want 1", result.RowsAffected())
	}
	if err := transaction.Commit(t.Context()); err != nil {
		t.Fatalf("commit visit deletion transaction: %v", err)
	}
}

func cleanupAutomaticSyncFixture(t *testing.T, pool *pgxpool.Pool, client *Client, fixture *automaticSyncFixture) {
	t.Helper()
	t.Cleanup(func() {
		ctx := context.Background()
		if fixture.visitID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM visits WHERE id = $1", fixture.visitID)
		}
		if fixture.doctorID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM doctors WHERE id = $1", fixture.doctorID)
		}
		if fixture.patientID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM patients WHERE id = $1", fixture.patientID)
		}
		if fixture.clinicID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM clinics WHERE id = $1", fixture.clinicID)
		}
		if fixture.specialtyID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM specialties WHERE id = $1", fixture.specialtyID)
		}
		ids := fixture.allAggregateIDs()
		if len(ids) > 0 {
			_, _ = pool.Exec(ctx, "DELETE FROM outbox_events WHERE aggregate_id = ANY($1::uuid[])", ids)
		}
		for _, document := range append(fixture.primaryDocuments(), automaticSyncDocument{indexName: VisitsIndexName, id: fixture.visitID}) {
			if document.id != "" {
				_ = client.DeleteDocument(ctx, document.indexName, document.id)
			}
		}
	})
}

func (fixture automaticSyncFixture) primaryDocuments() []automaticSyncDocument {
	return []automaticSyncDocument{
		{indexName: DoctorsIndexName, id: fixture.doctorID},
		{indexName: PatientsIndexName, id: fixture.patientID},
		{indexName: ClinicsIndexName, id: fixture.clinicID},
	}
}

func (fixture automaticSyncFixture) rolledBackDocuments() []automaticSyncDocument {
	return []automaticSyncDocument{
		{indexName: DoctorsIndexName, id: fixture.rolledBackDoctorID},
		{indexName: PatientsIndexName, id: fixture.rolledBackPatientID},
		{indexName: ClinicsIndexName, id: fixture.rolledBackClinicID},
	}
}

func (fixture automaticSyncFixture) rolledBackAggregateIDs() []string {
	return []string{fixture.rolledBackClinicID, fixture.rolledBackPatientID, fixture.rolledBackDoctorID}
}

func (fixture automaticSyncFixture) allAggregateIDs() []string {
	return []string{
		fixture.clinicID, fixture.patientID, fixture.doctorID, fixture.visitID,
		fixture.rolledBackClinicID, fixture.rolledBackPatientID, fixture.rolledBackDoctorID,
	}
}

func drainAutomaticSyncOutbox(t *testing.T, pool *pgxpool.Pool, client *Client) {
	t.Helper()
	processor := outbox.NewProcessor(pool, NewOutboxEventConsumer(pool, client))
	for attempt := 0; attempt < 100; attempt++ {
		processed, err := processor.ProcessBatch(t.Context(), 100)
		if err != nil {
			t.Fatalf("drain pending outbox events: %v", err)
		}
		if processed == 0 {
			return
		}
	}
	t.Fatal("pending outbox did not drain within 100 batches")
}

func processAutomaticSyncBatch(t *testing.T, processor *outbox.Processor, expected int) {
	t.Helper()
	processed, err := processor.ProcessBatch(t.Context(), 100)
	if err != nil {
		t.Fatalf("process automatic sync batch: %v", err)
	}
	if processed != expected {
		t.Fatalf("automatic sync batch processed %d events, want %d", processed, expected)
	}
}

func assertAutomaticSyncEntityDocuments(
	t *testing.T,
	client *Client,
	fixture automaticSyncFixture,
	doctorName string,
	patientLastName string,
	clinicName string,
) {
	t.Helper()
	var doctor DoctorDocument
	if found, err := client.GetDocument(t.Context(), DoctorsIndexName, fixture.doctorID, &doctor); err != nil || !found {
		t.Fatalf("load doctor document: found=%t error=%v", found, err)
	}
	if doctor.FullName != doctorName {
		t.Fatalf("doctor full name = %q, want %q", doctor.FullName, doctorName)
	}
	var patient PatientDocument
	if found, err := client.GetDocument(t.Context(), PatientsIndexName, fixture.patientID, &patient); err != nil || !found {
		t.Fatalf("load patient document: found=%t error=%v", found, err)
	}
	if patient.LastName != patientLastName {
		t.Fatalf("patient last name = %q, want %q", patient.LastName, patientLastName)
	}
	var clinic ClinicDocument
	if found, err := client.GetDocument(t.Context(), ClinicsIndexName, fixture.clinicID, &clinic); err != nil || !found {
		t.Fatalf("load clinic document: found=%t error=%v", found, err)
	}
	if clinic.Name != clinicName {
		t.Fatalf("clinic name = %q, want %q", clinic.Name, clinicName)
	}
}

func assertAutomaticSyncVisitDocuments(
	t *testing.T,
	client *Client,
	fixture automaticSyncFixture,
	doctorName string,
	patientLastName string,
	clinicName string,
) {
	t.Helper()
	var document VisitDocument
	if found, err := client.GetDocument(t.Context(), VisitsIndexName, fixture.visitID, &document); err != nil || !found {
		t.Fatalf("load visit document: found=%t error=%v", found, err)
	}
	if document.Doctor.FullName != doctorName || document.Patient.LastName != patientLastName || document.Clinic.Name != clinicName {
		t.Fatalf("visit denormalized data = doctor:%q patient:%q clinic:%q", document.Doctor.FullName, document.Patient.LastName, document.Clinic.Name)
	}
	assertAutomaticSyncEntityVisit(t, client, DoctorsIndexName, fixture.doctorID, fixture.visitID)
	assertAutomaticSyncEntityVisit(t, client, PatientsIndexName, fixture.patientID, fixture.visitID)
	assertAutomaticSyncEntityVisit(t, client, ClinicsIndexName, fixture.clinicID, fixture.visitID)
}

func assertAutomaticSyncEntityVisit(t *testing.T, client *Client, indexName, id, visitID string) {
	t.Helper()
	var visits []VisitSummary
	switch indexName {
	case DoctorsIndexName:
		var document DoctorDocument
		found, err := client.GetDocument(t.Context(), indexName, id, &document)
		if err != nil || !found {
			t.Fatalf("load %s document: found=%t error=%v", indexName, found, err)
		}
		visits = document.Visits
	case PatientsIndexName:
		var document PatientDocument
		found, err := client.GetDocument(t.Context(), indexName, id, &document)
		if err != nil || !found {
			t.Fatalf("load %s document: found=%t error=%v", indexName, found, err)
		}
		visits = document.Visits
	case ClinicsIndexName:
		var document ClinicDocument
		found, err := client.GetDocument(t.Context(), indexName, id, &document)
		if err != nil || !found {
			t.Fatalf("load %s document: found=%t error=%v", indexName, found, err)
		}
		visits = document.Visits
	}
	if len(visits) != 1 || visits[0].ID != visitID {
		t.Fatalf("%s visits = %+v, want visit %s", indexName, visits, visitID)
	}
}

func assertAutomaticSyncEntityVisitsEmpty(t *testing.T, client *Client, fixture automaticSyncFixture) {
	t.Helper()
	for _, document := range fixture.primaryDocuments() {
		assertAutomaticSyncEntityVisitCount(t, client, document, 0)
	}
}

func assertAutomaticSyncEntityVisitCount(t *testing.T, client *Client, target automaticSyncDocument, expected int) {
	t.Helper()
	count := -1
	switch target.indexName {
	case DoctorsIndexName:
		var document DoctorDocument
		found, err := client.GetDocument(t.Context(), target.indexName, target.id, &document)
		if err != nil || !found {
			t.Fatalf("load %s document: found=%t error=%v", target.indexName, found, err)
		}
		count = len(document.Visits)
	case PatientsIndexName:
		var document PatientDocument
		found, err := client.GetDocument(t.Context(), target.indexName, target.id, &document)
		if err != nil || !found {
			t.Fatalf("load %s document: found=%t error=%v", target.indexName, found, err)
		}
		count = len(document.Visits)
	case ClinicsIndexName:
		var document ClinicDocument
		found, err := client.GetDocument(t.Context(), target.indexName, target.id, &document)
		if err != nil || !found {
			t.Fatalf("load %s document: found=%t error=%v", target.indexName, found, err)
		}
		count = len(document.Visits)
	}
	if count != expected {
		t.Fatalf("%s visit count = %d, want %d", target.indexName, count, expected)
	}
}

func assertAutomaticSyncDocumentsMissing(t *testing.T, client *Client, documents []automaticSyncDocument) {
	t.Helper()
	for _, document := range documents {
		var destination map[string]any
		found, err := client.GetDocument(t.Context(), document.indexName, document.id, &destination)
		if err != nil {
			t.Fatalf("load missing %s document %s: %v", document.indexName, document.id, err)
		}
		if found {
			t.Fatalf("%s document %s still exists: %+v", document.indexName, document.id, destination)
		}
	}
}

func assertAutomaticSyncEventCount(t *testing.T, pool *pgxpool.Pool, aggregateIDs []string, expected int) {
	t.Helper()
	var count int
	if err := pool.QueryRow(
		t.Context(),
		"SELECT count(*) FROM outbox_events WHERE aggregate_id = ANY($1::uuid[])",
		aggregateIDs,
	).Scan(&count); err != nil {
		t.Fatalf("count aggregate outbox events: %v", err)
	}
	if count != expected {
		t.Fatalf("aggregate outbox event count = %d, want %d", count, expected)
	}
}

func loadAutomaticSyncPendingEvent(t *testing.T, pool *pgxpool.Pool, aggregateID string) outbox.PersistedEvent {
	t.Helper()
	var event outbox.PersistedEvent
	if err := pool.QueryRow(
		t.Context(),
		`SELECT id::text, event_sequence, aggregate_type, aggregate_id::text, event_type, payload, created_at
		 FROM outbox_events
		 WHERE aggregate_id = $1 AND processed_at IS NULL
		 ORDER BY event_sequence
		 LIMIT 1`,
		aggregateID,
	).Scan(&event.ID, &event.Sequence, &event.AggregateType, &event.AggregateID, &event.EventType, &event.Payload, &event.CreatedAt); err != nil {
		t.Fatalf("load pending outbox event: %v", err)
	}
	return event
}

func loadAutomaticSyncEventIDs(t *testing.T, pool *pgxpool.Pool, aggregateID string) []string {
	t.Helper()
	rows, err := pool.Query(
		t.Context(),
		"SELECT id::text FROM outbox_events WHERE aggregate_id = $1 ORDER BY event_sequence",
		aggregateID,
	)
	if err != nil {
		t.Fatalf("load ordered event IDs: %v", err)
	}
	defer rows.Close()
	ids := make([]string, 0, 3)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan ordered event ID: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate ordered event IDs: %v", err)
	}
	return ids
}

func assertAutomaticSyncEventPending(t *testing.T, pool *pgxpool.Pool, eventID string, expected bool) {
	t.Helper()
	var pending bool
	if err := pool.QueryRow(
		t.Context(),
		"SELECT processed_at IS NULL FROM outbox_events WHERE id = $1",
		eventID,
	).Scan(&pending); err != nil {
		t.Fatalf("load outbox event state: %v", err)
	}
	if pending != expected {
		t.Fatalf("outbox event %s pending=%t, want %t", eventID, pending, expected)
	}
}
