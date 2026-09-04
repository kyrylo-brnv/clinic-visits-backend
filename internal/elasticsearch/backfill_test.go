package elasticsearch

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
)

func TestBuildBackfillDocumentsEnrichesEveryDocument(t *testing.T) {
	t.Parallel()

	createdAt := time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC)
	updatedAt := createdAt.Add(time.Hour)
	dateOfBirth := time.Date(1985, time.January, 2, 0, 0, 0, 0, time.UTC)
	visitStart := createdAt.Add(24 * time.Hour)
	visitEnd := visitStart.Add(30 * time.Minute)

	documents, err := buildBackfillDocuments(backfillRows{
		doctors: []sqlc.ListDoctorsForElasticsearchBackfillRow{
			{
				ID: "doctor-1", SpecialtyID: "specialty-1", ClinicID: "clinic-1", FullName: "Ada Lovelace",
				CreatedAt: timestampValue(createdAt), UpdatedAt: timestampValue(updatedAt),
			},
			{
				ID: "doctor-2", SpecialtyID: "specialty-2", ClinicID: "clinic-2", FullName: "Grace Hopper",
				CreatedAt: timestampValue(createdAt), UpdatedAt: timestampValue(updatedAt),
			},
		},
		patients: []sqlc.ListPatientsForElasticsearchBackfillRow{
			{
				ID: "patient-1", FirstName: "Linus", LastName: "Pauling", DateOfBirth: dateValue(dateOfBirth),
				Gender: "Male", IsDeleted: boolValue(true), CreatedAt: timestampValue(createdAt), UpdatedAt: timestampValue(updatedAt),
			},
			{
				ID: "patient-2", FirstName: "Katherine", LastName: "Johnson", DateOfBirth: dateValue(dateOfBirth),
				Gender: "Female", CreatedAt: timestampValue(createdAt), UpdatedAt: timestampValue(updatedAt),
			},
		},
		clinics: []sqlc.ListClinicsForElasticsearchBackfillRow{
			{
				ID: "clinic-1", Name: "Central", Address: "1 Main Street", TimeZone: "Europe/Kyiv",
				CreatedAt: timestampValue(createdAt), UpdatedAt: timestampValue(updatedAt),
			},
			{
				ID: "clinic-2", Name: "North", Address: "2 Main Street", TimeZone: "Europe/Kyiv",
				CreatedAt: timestampValue(createdAt), UpdatedAt: timestampValue(updatedAt),
			},
		},
		visits: []sqlc.ListVisitsForElasticsearchBackfillRow{
			{
				ID: "visit-1", DoctorID: "doctor-1", PatientID: "patient-1", ClinicID: "clinic-1", Status: "CLOSED",
				VisitStartTime: timestampValue(visitStart), VisitEndTime: timestampValue(visitEnd),
				CreatedAt: timestampValue(createdAt), UpdatedAt: timestampValue(updatedAt),
				DoctorSpecialtyID: "specialty-1", DoctorClinicID: "clinic-1", DoctorFullName: "Ada Lovelace",
				PatientFirstName: "Linus", PatientLastName: "Pauling", PatientDateOfBirth: dateValue(dateOfBirth),
				PatientGender: "Male", PatientIsDeleted: boolValue(true),
				ClinicName: "Central", ClinicAddress: "1 Main Street", ClinicTimeZone: "Europe/Kyiv",
			},
		},
	})
	if err != nil {
		t.Fatalf("buildBackfillDocuments() error = %v", err)
	}

	summary := VisitSummary{
		ID: "visit-1", DoctorID: "doctor-1", PatientID: "patient-1", ClinicID: "clinic-1", Status: "CLOSED",
		VisitStartTime: visitStart, VisitEndTime: visitEnd, CreatedAt: createdAt, UpdatedAt: updatedAt,
	}
	want := backfillDocuments{
		doctors: []DoctorDocument{
			{
				ID: "doctor-1", SpecialtyID: "specialty-1", ClinicID: "clinic-1", FullName: "Ada Lovelace",
				CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{summary},
			},
			{
				ID: "doctor-2", SpecialtyID: "specialty-2", ClinicID: "clinic-2", FullName: "Grace Hopper",
				CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{},
			},
		},
		patients: []PatientDocument{
			{
				ID: "patient-1", FirstName: "Linus", LastName: "Pauling", DateOfBirth: dateOfBirth, Gender: "Male",
				IsDeleted: boolPointer(true), CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{summary},
			},
			{
				ID: "patient-2", FirstName: "Katherine", LastName: "Johnson", DateOfBirth: dateOfBirth, Gender: "Female",
				CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{},
			},
		},
		clinics: []ClinicDocument{
			{
				ID: "clinic-1", Name: "Central", Address: "1 Main Street", TimeZone: "Europe/Kyiv",
				CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{summary},
			},
			{
				ID: "clinic-2", Name: "North", Address: "2 Main Street", TimeZone: "Europe/Kyiv",
				CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{},
			},
		},
		visits: []VisitDocument{
			{
				ID: "visit-1", DoctorID: "doctor-1", PatientID: "patient-1", ClinicID: "clinic-1", Status: "CLOSED",
				VisitStartTime: visitStart, VisitEndTime: visitEnd, CreatedAt: createdAt, UpdatedAt: updatedAt,
				Doctor: VisitDoctorData{
					ID: "doctor-1", SpecialtyID: "specialty-1", ClinicID: "clinic-1", FullName: "Ada Lovelace",
				},
				Patient: VisitPatientData{
					ID: "patient-1", FirstName: "Linus", LastName: "Pauling", DateOfBirth: dateOfBirth,
					Gender: "Male", IsDeleted: boolPointer(true),
				},
				Clinic: VisitClinicData{
					ID: "clinic-1", Name: "Central", Address: "1 Main Street", TimeZone: "Europe/Kyiv",
				},
			},
		},
	}

	if !reflect.DeepEqual(documents, want) {
		t.Fatalf("buildBackfillDocuments() = %#v, want %#v", documents, want)
	}
}

func TestBuildBackfillDocumentsRejectsInconsistentSnapshot(t *testing.T) {
	t.Parallel()

	_, err := buildBackfillDocuments(backfillRows{
		visits: []sqlc.ListVisitsForElasticsearchBackfillRow{{ID: "visit-1", DoctorID: "missing-doctor"}},
	})
	if err == nil || !strings.Contains(err.Error(), "visit visit-1 references missing doctor missing-doctor") {
		t.Fatalf("buildBackfillDocuments() error = %v", err)
	}
}

func TestBuildBackfillDocumentsRejectsNullRequiredValues(t *testing.T) {
	t.Parallel()

	_, err := buildBackfillDocuments(backfillRows{
		doctors: []sqlc.ListDoctorsForElasticsearchBackfillRow{{ID: "doctor-1"}},
	})
	if err == nil || !strings.Contains(err.Error(), "doctor doctor-1 has null created_at") {
		t.Fatalf("buildBackfillDocuments() error = %v", err)
	}
}

func TestBuildBackfillDocumentsPreservesNullPatientDeletionStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC)
	documents, err := buildBackfillDocuments(backfillRows{
		patients: []sqlc.ListPatientsForElasticsearchBackfillRow{{
			ID: "patient-1", FirstName: "Ada", LastName: "Lovelace", DateOfBirth: dateValue(now), Gender: "Female",
			CreatedAt: timestampValue(now), UpdatedAt: timestampValue(now),
		}},
	})
	if err != nil {
		t.Fatalf("buildBackfillDocuments() error = %v", err)
	}
	if documents.patients[0].IsDeleted != nil {
		t.Fatalf("patient is_deleted = %v, want nil for a NULL database value", *documents.patients[0].IsDeleted)
	}
}

func TestUpsertBackfillDocumentsUsesEveryIndexInDeterministicOrder(t *testing.T) {
	t.Parallel()

	client := &recordingDocumentUpserter{}
	documents := backfillDocuments{
		doctors:  []DoctorDocument{{ID: "doctor-1"}, {ID: "doctor-2"}},
		patients: []PatientDocument{{ID: "patient-1"}},
		clinics:  []ClinicDocument{{ID: "clinic-1"}},
		visits:   []VisitDocument{{ID: "visit-1"}},
	}

	if err := upsertBackfillDocuments(t.Context(), client, documents); err != nil {
		t.Fatalf("upsertBackfillDocuments() error = %v", err)
	}

	want := []recordedDocumentUpsert{
		{index: DoctorsIndexName, id: "doctor-1", document: documents.doctors[0]},
		{index: DoctorsIndexName, id: "doctor-2", document: documents.doctors[1]},
		{index: PatientsIndexName, id: "patient-1", document: documents.patients[0]},
		{index: ClinicsIndexName, id: "clinic-1", document: documents.clinics[0]},
		{index: VisitsIndexName, id: "visit-1", document: documents.visits[0]},
	}
	if !reflect.DeepEqual(client.calls, want) {
		t.Fatalf("upsert calls = %#v, want %#v", client.calls, want)
	}
}

func TestUpsertBackfillDocumentsStopsWithContextOnFailure(t *testing.T) {
	t.Parallel()

	failure := errors.New("Elasticsearch unavailable")
	client := &recordingDocumentUpserter{failAtCall: 2, err: failure}
	err := upsertBackfillDocuments(t.Context(), client, backfillDocuments{
		doctors: []DoctorDocument{{ID: "doctor-1"}, {ID: "doctor-2"}, {ID: "doctor-3"}},
	})
	if !errors.Is(err, failure) {
		t.Fatalf("upsertBackfillDocuments() error = %v, want wrapped failure", err)
	}
	if !strings.Contains(err.Error(), "upsert doctor doctor-2") {
		t.Fatalf("upsertBackfillDocuments() error = %v, want doctor context", err)
	}
	if len(client.calls) != 2 {
		t.Fatalf("upsert call count = %d, want 2", len(client.calls))
	}
}

func TestUpsertBackfillDocumentsForIndexUsesOnlySelectedIndex(t *testing.T) {
	t.Parallel()

	documents := backfillDocuments{
		doctors:  []DoctorDocument{{ID: "doctor-1"}},
		patients: []PatientDocument{{ID: "patient-1"}},
		clinics:  []ClinicDocument{{ID: "clinic-1"}},
		visits:   []VisitDocument{{ID: "visit-1"}},
	}

	for _, testCase := range []struct {
		indexName string
		want      recordedDocumentUpsert
	}{
		{indexName: DoctorsIndexName, want: recordedDocumentUpsert{index: DoctorsIndexName, id: "doctor-1", document: documents.doctors[0]}},
		{indexName: PatientsIndexName, want: recordedDocumentUpsert{index: PatientsIndexName, id: "patient-1", document: documents.patients[0]}},
		{indexName: ClinicsIndexName, want: recordedDocumentUpsert{index: ClinicsIndexName, id: "clinic-1", document: documents.clinics[0]}},
		{indexName: VisitsIndexName, want: recordedDocumentUpsert{index: VisitsIndexName, id: "visit-1", document: documents.visits[0]}},
	} {
		testCase := testCase
		t.Run(testCase.indexName, func(t *testing.T) {
			t.Parallel()

			client := &recordingDocumentUpserter{}
			if err := upsertBackfillDocumentsForIndex(t.Context(), client, testCase.indexName, documents); err != nil {
				t.Fatalf("upsertBackfillDocumentsForIndex() error = %v", err)
			}
			if !reflect.DeepEqual(client.calls, []recordedDocumentUpsert{testCase.want}) {
				t.Fatalf("upsert calls = %#v, want %#v", client.calls, []recordedDocumentUpsert{testCase.want})
			}
		})
	}
}

func TestBackfillIndexRejectsUnsupportedIndexBeforeLoadingSnapshot(t *testing.T) {
	t.Parallel()

	err := BackfillIndex(t.Context(), nil, nil, "unknown-v1")
	if err == nil || !strings.Contains(err.Error(), `unsupported Elasticsearch index "unknown-v1"`) {
		t.Fatalf("BackfillIndex() error = %v", err)
	}
}

func TestValidateBackfillOptionsRejectsOutOfRangeValues(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		name    string
		options BackfillOptions
		want    error
	}{
		{name: "zero batch", options: BackfillOptions{BatchSize: 0, Concurrency: 1}, want: ErrInvalidBackfillBatchSize},
		{name: "large batch", options: BackfillOptions{BatchSize: MaxBackfillBatchSize + 1, Concurrency: 1}, want: ErrInvalidBackfillBatchSize},
		{name: "zero concurrency", options: BackfillOptions{BatchSize: 1, Concurrency: 0}, want: ErrInvalidBackfillConcurrency},
		{name: "large concurrency", options: BackfillOptions{BatchSize: 1, Concurrency: MaxBackfillConcurrency + 1}, want: ErrInvalidBackfillConcurrency},
	} {
		t.Run(testCase.name, func(t *testing.T) {
			t.Parallel()
			if err := ValidateBackfillOptions(testCase.options); !errors.Is(err, testCase.want) {
				t.Fatalf("ValidateBackfillOptions() error = %v, want %v", err, testCase.want)
			}
		})
	}
}

func TestUpsertDocumentsUsesBoundedConcurrentWorkerPool(t *testing.T) {
	t.Parallel()

	client := newBlockingDocumentUpserter()
	documents := []DoctorDocument{
		{ID: "doctor-1"}, {ID: "doctor-2"}, {ID: "doctor-3"},
		{ID: "doctor-4"}, {ID: "doctor-5"}, {ID: "doctor-6"},
	}
	done := make(chan error, 1)
	go func() {
		done <- upsertDocuments(
			context.Background(),
			client,
			DoctorsIndexName,
			"doctor",
			documents,
			func(document DoctorDocument) string { return document.ID },
			BackfillOptions{BatchSize: 1, Concurrency: 3},
		)
	}()

	for range 3 {
		select {
		case <-client.started:
		case <-time.After(time.Second):
			client.releaseAll()
			t.Fatal("configured backfill workers did not start concurrently")
		}
	}
	select {
	case <-client.started:
		client.releaseAll()
		t.Fatal("more than the configured three upserts started")
	case <-time.After(50 * time.Millisecond):
	}
	if got := client.maximum.Load(); got != 3 {
		client.releaseAll()
		t.Fatalf("maximum concurrent upserts = %d, want 3", got)
	}
	client.releaseAll()
	if err := <-done; err != nil {
		t.Fatalf("upsertDocuments() error = %v", err)
	}
}

func TestUpsertDocumentsReportsConcurrentFailuresInDocumentOrder(t *testing.T) {
	t.Parallel()

	client := &perDocumentErrorUpserter{errors: map[string]error{
		"doctor-1": errors.New("first failure"),
		"doctor-2": errors.New("second failure"),
	}}
	err := upsertDocuments(
		t.Context(),
		client,
		DoctorsIndexName,
		"doctor",
		[]DoctorDocument{{ID: "doctor-1"}, {ID: "doctor-2"}},
		func(document DoctorDocument) string { return document.ID },
		BackfillOptions{BatchSize: 1, Concurrency: 2},
	)
	if err == nil {
		t.Fatal("upsertDocuments() error = nil, want concurrent failures")
	}
	firstPosition := strings.Index(err.Error(), "upsert doctor doctor-1")
	secondPosition := strings.Index(err.Error(), "upsert doctor doctor-2")
	if firstPosition < 0 || secondPosition < 0 || firstPosition >= secondPosition {
		t.Fatalf("upsertDocuments() error = %q, want failures in document order", err)
	}
}

func TestUpsertDocumentsCancelsWorkers(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	started := make(chan struct{}, 1)
	client := documentUpserterFunc(func(ctx context.Context, _, _ string, _ any) error {
		select {
		case started <- struct{}{}:
		default:
		}
		<-ctx.Done()
		return ctx.Err()
	})
	done := make(chan error, 1)
	go func() {
		done <- upsertDocuments(
			ctx,
			client,
			DoctorsIndexName,
			"doctor",
			[]DoctorDocument{{ID: "doctor-1"}, {ID: "doctor-2"}},
			func(document DoctorDocument) string { return document.ID },
			BackfillOptions{BatchSize: 1, Concurrency: 2},
		)
	}()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("backfill worker did not start")
	}
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("upsertDocuments() error = %v, want cancellation", err)
		}
	case <-time.After(time.Second):
		t.Fatal("backfill workers did not stop after cancellation")
	}
}

type recordedDocumentUpsert struct {
	index    string
	id       string
	document any
}

type recordingDocumentUpserter struct {
	calls      []recordedDocumentUpsert
	failAtCall int
	err        error
}

type blockingDocumentUpserter struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	active  atomic.Int32
	maximum atomic.Int32
}

func newBlockingDocumentUpserter() *blockingDocumentUpserter {
	return &blockingDocumentUpserter{
		started: make(chan struct{}, MaxBackfillConcurrency),
		release: make(chan struct{}),
	}
}

func (c *blockingDocumentUpserter) UpsertDocument(context.Context, string, string, any) error {
	current := c.active.Add(1)
	for {
		observed := c.maximum.Load()
		if current <= observed || c.maximum.CompareAndSwap(observed, current) {
			break
		}
	}
	c.started <- struct{}{}
	<-c.release
	c.active.Add(-1)
	return nil
}

func (c *blockingDocumentUpserter) releaseAll() {
	c.once.Do(func() { close(c.release) })
}

type perDocumentErrorUpserter struct {
	errors map[string]error
}

func (c *perDocumentErrorUpserter) UpsertDocument(_ context.Context, _, id string, _ any) error {
	return c.errors[id]
}

type documentUpserterFunc func(context.Context, string, string, any) error

func (f documentUpserterFunc) UpsertDocument(ctx context.Context, index, id string, document any) error {
	return f(ctx, index, id, document)
}

func (c *recordingDocumentUpserter) UpsertDocument(_ context.Context, index, id string, document any) error {
	c.calls = append(c.calls, recordedDocumentUpsert{index: index, id: id, document: document})
	if c.failAtCall > 0 && len(c.calls) == c.failAtCall {
		return c.err
	}
	return nil
}

func timestampValue(value time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: value, Valid: true}
}

func dateValue(value time.Time) pgtype.Date {
	return pgtype.Date{Time: value, Valid: true}
}

func boolValue(value bool) pgtype.Bool {
	return pgtype.Bool{Bool: value, Valid: true}
}

func boolPointer(value bool) *bool {
	return &value
}
