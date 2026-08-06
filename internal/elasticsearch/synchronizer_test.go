package elasticsearch

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
	"github.com/smithautotest/clinic-visits/internal/outbox"
)

const (
	testVisitID      = "00000000-0000-4000-8000-000000000001"
	testOldDoctorID  = "00000000-0000-4000-8000-000000000002"
	testNewDoctorID  = "00000000-0000-4000-8000-000000000003"
	testOldPatientID = "00000000-0000-4000-8000-000000000004"
	testNewPatientID = "00000000-0000-4000-8000-000000000005"
	testOldClinicID  = "00000000-0000-4000-8000-000000000006"
	testNewClinicID  = "00000000-0000-4000-8000-000000000007"
)

func TestVisitEventConsumerSynchronizesCurrentVisitAndOldAndNewRelations(t *testing.T) {
	t.Parallel()

	indexedVisit := VisitDocument{
		ID: testVisitID, DoctorID: testOldDoctorID, PatientID: testOldPatientID, ClinicID: testOldClinicID,
	}
	currentVisit := VisitDocument{
		ID: testVisitID, DoctorID: testNewDoctorID, PatientID: testNewPatientID, ClinicID: testNewClinicID,
		Status: "IN_PROGRESS",
	}
	relations := relationIDs(
		[]string{testOldDoctorID, testNewDoctorID},
		[]string{testOldPatientID, testNewPatientID},
		[]string{testOldClinicID, testNewClinicID},
	)
	snapshot := visitSyncSnapshot{
		visit: &currentVisit,
		doctors: map[string]DoctorDocument{
			testOldDoctorID: {ID: testOldDoctorID, Visits: []VisitSummary{}},
			testNewDoctorID: {ID: testNewDoctorID, Visits: []VisitSummary{{ID: testVisitID}}},
		},
		patients: map[string]PatientDocument{
			testOldPatientID: {ID: testOldPatientID, Visits: []VisitSummary{}},
			testNewPatientID: {ID: testNewPatientID, Visits: []VisitSummary{{ID: testVisitID}}},
		},
		clinics: map[string]ClinicDocument{
			testOldClinicID: {ID: testOldClinicID, Visits: []VisitSummary{}},
			testNewClinicID: {ID: testNewClinicID, Visits: []VisitSummary{{ID: testVisitID}}},
		},
		relations: relations,
	}
	loader := &stubVisitSyncLoader{snapshot: snapshot}
	store := &recordingVisitDocumentStore{indexedVisit: &indexedVisit}
	consumer := &visitEventConsumer{loader: loader, store: store}

	err := consumer.Consume(t.Context(), outbox.PersistedEvent{
		AggregateType: outbox.AggregateTypeVisit,
		AggregateID:   testVisitID,
		EventType:     outbox.EventTypeVisitUpdated,
		Payload:       visitEventPayload(indexedVisit),
	})
	if err != nil {
		t.Fatalf("Consume() error = %v", err)
	}

	wantLoadedRelations := relationIDs(
		[]string{testOldDoctorID},
		[]string{testOldPatientID},
		[]string{testOldClinicID},
	)
	if !reflect.DeepEqual(loader.relations, wantLoadedRelations) {
		t.Fatalf("loader relations = %#v, want %#v", loader.relations, wantLoadedRelations)
	}
	wantCalls := []documentStoreCall{
		{operation: "upsert", index: DoctorsIndexName, id: testOldDoctorID, document: snapshot.doctors[testOldDoctorID]},
		{operation: "upsert", index: DoctorsIndexName, id: testNewDoctorID, document: snapshot.doctors[testNewDoctorID]},
		{operation: "upsert", index: PatientsIndexName, id: testOldPatientID, document: snapshot.patients[testOldPatientID]},
		{operation: "upsert", index: PatientsIndexName, id: testNewPatientID, document: snapshot.patients[testNewPatientID]},
		{operation: "upsert", index: ClinicsIndexName, id: testOldClinicID, document: snapshot.clinics[testOldClinicID]},
		{operation: "upsert", index: ClinicsIndexName, id: testNewClinicID, document: snapshot.clinics[testNewClinicID]},
		{operation: "upsert", index: VisitsIndexName, id: testVisitID, document: currentVisit},
	}
	if !reflect.DeepEqual(store.calls, wantCalls) {
		t.Fatalf("document calls = %#v, want %#v", store.calls, wantCalls)
	}
}

func TestVisitEventConsumerDeletesMissingVisitAfterRefreshingRelations(t *testing.T) {
	t.Parallel()

	indexedVisit := VisitDocument{
		ID: testVisitID, DoctorID: testOldDoctorID, PatientID: testOldPatientID, ClinicID: testOldClinicID,
	}
	relations := relationIDs(
		[]string{testOldDoctorID}, []string{testOldPatientID}, []string{testOldClinicID},
	)
	snapshot := visitSyncSnapshot{
		doctors:   map[string]DoctorDocument{testOldDoctorID: {ID: testOldDoctorID, Visits: []VisitSummary{}}},
		patients:  map[string]PatientDocument{testOldPatientID: {ID: testOldPatientID, Visits: []VisitSummary{}}},
		clinics:   map[string]ClinicDocument{testOldClinicID: {ID: testOldClinicID, Visits: []VisitSummary{}}},
		relations: relations,
	}
	store := &recordingVisitDocumentStore{indexedVisit: &indexedVisit}
	consumer := &visitEventConsumer{loader: &stubVisitSyncLoader{snapshot: snapshot}, store: store}

	if err := consumer.Consume(t.Context(), outbox.PersistedEvent{
		AggregateType: outbox.AggregateTypeVisit,
		AggregateID:   testVisitID,
		EventType:     outbox.EventTypeVisitDeleted,
		Payload:       visitEventPayload(indexedVisit),
	}); err != nil {
		t.Fatalf("Consume() error = %v", err)
	}

	lastCall := store.calls[len(store.calls)-1]
	if lastCall.operation != "delete" || lastCall.index != VisitsIndexName || lastCall.id != testVisitID {
		t.Fatalf("last document call = %#v, want visit delete", lastCall)
	}
	if len(store.calls) != 4 {
		t.Fatalf("document call count = %d, want 4", len(store.calls))
	}
}

func TestMapSyncPatientDocumentsPreservesNullDeletionStatus(t *testing.T) {
	t.Parallel()

	now := time.Date(2026, time.August, 5, 9, 0, 0, 0, time.UTC)
	documents, err := mapSyncPatientDocuments([]sqlc.ListPatientsForElasticsearchSyncRow{{
		ID: "patient-1", FirstName: "Ada", LastName: "Lovelace", DateOfBirth: dateValue(now), Gender: "Female",
		CreatedAt: timestampValue(now), UpdatedAt: timestampValue(now),
	}})
	if err != nil {
		t.Fatalf("mapSyncPatientDocuments() error = %v", err)
	}
	if documents["patient-1"].IsDeleted != nil {
		t.Fatalf("patient is_deleted = %v, want nil for a NULL database value", *documents["patient-1"].IsDeleted)
	}
}

func TestVisitEventConsumerFailureIsReturnedBeforeVisitReplacement(t *testing.T) {
	t.Parallel()

	indexedVisit := VisitDocument{
		ID: testVisitID, DoctorID: testOldDoctorID, PatientID: testOldPatientID, ClinicID: testOldClinicID,
	}
	snapshot := visitSyncSnapshot{
		visit:     &VisitDocument{ID: testVisitID, DoctorID: testNewDoctorID},
		doctors:   map[string]DoctorDocument{testOldDoctorID: {ID: testOldDoctorID}},
		patients:  map[string]PatientDocument{},
		clinics:   map[string]ClinicDocument{},
		relations: relationIDs([]string{testOldDoctorID}, nil, nil),
	}
	failure := errors.New("Elasticsearch unavailable")
	store := &recordingVisitDocumentStore{indexedVisit: &indexedVisit, failAtWrite: 1, err: failure}
	consumer := &visitEventConsumer{loader: &stubVisitSyncLoader{snapshot: snapshot}, store: store}

	err := consumer.Consume(t.Context(), outbox.PersistedEvent{
		AggregateType: outbox.AggregateTypeVisit,
		AggregateID:   testVisitID,
		EventType:     outbox.EventTypeVisitUpdated,
		Payload:       visitEventPayload(indexedVisit),
	})
	if !errors.Is(err, failure) {
		t.Fatalf("Consume() error = %v, want wrapped Elasticsearch failure", err)
	}
	for _, call := range store.calls {
		if call.index == VisitsIndexName {
			t.Fatalf("visit document changed after related document failure: %#v", call)
		}
	}
}

func TestVisitEventConsumerCanRetryAfterVisitWriteFailure(t *testing.T) {
	t.Parallel()

	currentVisit := VisitDocument{
		ID: testVisitID, DoctorID: testNewDoctorID, PatientID: testNewPatientID, ClinicID: testNewClinicID,
	}
	relations := relationIDs(
		[]string{testNewDoctorID}, []string{testNewPatientID}, []string{testNewClinicID},
	)
	snapshot := visitSyncSnapshot{
		visit:     &currentVisit,
		doctors:   map[string]DoctorDocument{testNewDoctorID: {ID: testNewDoctorID}},
		patients:  map[string]PatientDocument{testNewPatientID: {ID: testNewPatientID}},
		clinics:   map[string]ClinicDocument{testNewClinicID: {ID: testNewClinicID}},
		relations: relations,
	}
	failure := errors.New("visit index unavailable")
	store := &recordingVisitDocumentStore{failAtWrite: 4, err: failure}
	consumer := &visitEventConsumer{loader: &stubVisitSyncLoader{snapshot: snapshot}, store: store}
	event := outbox.PersistedEvent{
		AggregateType: outbox.AggregateTypeVisit,
		AggregateID:   testVisitID,
		EventType:     outbox.EventTypeVisitCreated,
		Payload:       visitEventPayload(currentVisit),
	}

	if err := consumer.Consume(t.Context(), event); !errors.Is(err, failure) {
		t.Fatalf("first Consume() error = %v, want wrapped visit failure", err)
	}
	store.failAtWrite = 0
	if err := consumer.Consume(t.Context(), event); err != nil {
		t.Fatalf("retry Consume() error = %v", err)
	}
	if len(store.calls) != 8 {
		t.Fatalf("write call count after retry = %d, want 8", len(store.calls))
	}
	if !reflect.DeepEqual(store.calls[:4], store.calls[4:]) {
		t.Fatalf("retry writes differ: first=%#v retry=%#v", store.calls[:4], store.calls[4:])
	}
}

func TestVisitEventConsumerRejectsUnsupportedEventsBeforeSynchronization(t *testing.T) {
	t.Parallel()

	tests := []outbox.PersistedEvent{
		{AggregateType: "patient", AggregateID: testVisitID, EventType: outbox.EventTypeVisitCreated},
		{AggregateType: outbox.AggregateTypeVisit, AggregateID: testVisitID, EventType: "visit.unknown"},
		{AggregateType: outbox.AggregateTypeVisit, AggregateID: "not-a-uuid", EventType: outbox.EventTypeVisitCreated},
		{AggregateType: outbox.AggregateTypeVisit, AggregateID: testVisitID, EventType: outbox.EventTypeVisitCreated, Payload: []byte("{")},
	}
	for _, event := range tests {
		loader := &stubVisitSyncLoader{}
		store := &recordingVisitDocumentStore{}
		consumer := &visitEventConsumer{loader: loader, store: store}

		if err := consumer.Consume(t.Context(), event); err == nil {
			t.Fatalf("Consume(%+v) error = nil, want rejection", event)
		}
		if loader.calls != 0 || store.getCalls != 0 || len(store.calls) != 0 {
			t.Fatalf("unsupported event performed synchronization: loader=%d gets=%d calls=%d", loader.calls, store.getCalls, len(store.calls))
		}
	}
}

func TestAddSyncVisitSummariesMaintainsOnlyLoadedEntityDocuments(t *testing.T) {
	t.Parallel()

	visitStart := time.Date(2026, time.August, 5, 10, 0, 0, 0, time.UTC)
	snapshot := visitSyncSnapshot{
		doctors:  map[string]DoctorDocument{testOldDoctorID: {ID: testOldDoctorID, Visits: []VisitSummary{}}},
		patients: map[string]PatientDocument{testOldPatientID: {ID: testOldPatientID, Visits: []VisitSummary{}}},
		clinics:  map[string]ClinicDocument{testOldClinicID: {ID: testOldClinicID, Visits: []VisitSummary{}}},
	}
	rows := []sqlc.ListVisitSummariesForElasticsearchSyncRow{
		{
			ID: testVisitID, DoctorID: testOldDoctorID, PatientID: testOldPatientID, ClinicID: testOldClinicID,
			Status: "SCHEDULED", VisitStartTime: timestampValue(visitStart),
			VisitEndTime: timestampValue(visitStart.Add(time.Hour)), CreatedAt: timestampValue(visitStart),
			UpdatedAt: timestampValue(visitStart),
		},
		{
			ID: "00000000-0000-4000-8000-000000000008", DoctorID: testNewDoctorID,
			PatientID: testNewPatientID, ClinicID: testNewClinicID, Status: "CLOSED",
			VisitStartTime: timestampValue(visitStart.Add(2 * time.Hour)),
			VisitEndTime:   timestampValue(visitStart.Add(3 * time.Hour)), CreatedAt: timestampValue(visitStart),
			UpdatedAt: timestampValue(visitStart),
		},
	}

	if err := addSyncVisitSummaries(snapshot, rows); err != nil {
		t.Fatalf("addSyncVisitSummaries() error = %v", err)
	}
	if got := snapshot.doctors[testOldDoctorID].Visits; len(got) != 1 || got[0].ID != testVisitID {
		t.Fatalf("doctor visits = %#v", got)
	}
	if got := snapshot.patients[testOldPatientID].Visits; len(got) != 1 || got[0].ID != testVisitID {
		t.Fatalf("patient visits = %#v", got)
	}
	if got := snapshot.clinics[testOldClinicID].Visits; len(got) != 1 || got[0].ID != testVisitID {
		t.Fatalf("clinic visits = %#v", got)
	}
}

func TestMapSyncVisitDocumentRejectsNullRequiredFields(t *testing.T) {
	t.Parallel()

	_, err := mapSyncVisitDocument(sqlc.GetVisitForElasticsearchSyncRow{ID: testVisitID})
	if err == nil || !strings.Contains(err.Error(), "has null visit_start_time") {
		t.Fatalf("mapSyncVisitDocument() error = %v", err)
	}
}

type stubVisitSyncLoader struct {
	snapshot  visitSyncSnapshot
	err       error
	calls     int
	relations visitRelationIDs
}

func (l *stubVisitSyncLoader) Load(_ context.Context, _ string, relations visitRelationIDs) (visitSyncSnapshot, error) {
	l.calls++
	l.relations = cloneRelationIDs(relations)
	return l.snapshot, l.err
}

type documentStoreCall struct {
	operation string
	index     string
	id        string
	document  any
}

type recordingVisitDocumentStore struct {
	indexedVisit *VisitDocument
	getErr       error
	getCalls     int
	calls        []documentStoreCall
	failAtWrite  int
	err          error
}

func (s *recordingVisitDocumentStore) GetDocument(_ context.Context, index, id string, document any) (bool, error) {
	s.getCalls++
	if s.getErr != nil {
		return false, s.getErr
	}
	if s.indexedVisit == nil {
		return false, nil
	}
	if index != VisitsIndexName || id != s.indexedVisit.ID {
		return false, errors.New("unexpected document get")
	}
	destination, ok := document.(*VisitDocument)
	if !ok {
		return false, errors.New("unexpected document destination")
	}
	*destination = *s.indexedVisit
	return true, nil
}

func (s *recordingVisitDocumentStore) UpsertDocument(_ context.Context, index, id string, document any) error {
	s.calls = append(s.calls, documentStoreCall{operation: "upsert", index: index, id: id, document: document})
	return s.writeError()
}

func (s *recordingVisitDocumentStore) DeleteDocument(_ context.Context, index, id string) error {
	s.calls = append(s.calls, documentStoreCall{operation: "delete", index: index, id: id})
	return s.writeError()
}

func (s *recordingVisitDocumentStore) writeError() error {
	if s.failAtWrite > 0 && len(s.calls) == s.failAtWrite {
		return s.err
	}
	return nil
}

func relationIDs(doctors, patients, clinics []string) visitRelationIDs {
	ids := newVisitRelationIDs()
	for _, id := range doctors {
		ids.doctors[id] = struct{}{}
	}
	for _, id := range patients {
		ids.patients[id] = struct{}{}
	}
	for _, id := range clinics {
		ids.clinics[id] = struct{}{}
	}
	return ids
}

func cloneRelationIDs(ids visitRelationIDs) visitRelationIDs {
	return relationIDs(sortedKeys(ids.doctors), sortedKeys(ids.patients), sortedKeys(ids.clinics))
}

func visitEventPayload(visit VisitDocument) []byte {
	return []byte(`{"id":"` + visit.ID + `","doctor_id":"` + visit.DoctorID +
		`","patient_id":"` + visit.PatientID + `","clinic_id":"` + visit.ClinicID + `"}`)
}
