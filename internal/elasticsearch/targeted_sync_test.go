package elasticsearch

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

const (
	testStaleVisitID   = "00000000-0000-4000-8000-000000000008"
	testCurrentVisitID = "00000000-0000-4000-8000-000000000009"
)

func TestValidateSyncIndexRequestSortsAndDeduplicatesCanonicalIDs(t *testing.T) {
	t.Parallel()

	ids, err := ValidateSyncIndexRequest(DoctorsIndexName, []string{testNewDoctorID, testOldDoctorID, testNewDoctorID})
	if err != nil {
		t.Fatalf("ValidateSyncIndexRequest() error = %v", err)
	}
	want := []string{testOldDoctorID, testNewDoctorID}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ValidateSyncIndexRequest() IDs = %q, want %q", ids, want)
	}
}

func TestDeltaSynchronizerEntityTargetsRefreshDependentVisitsThenPrimary(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		indexName string
		id        string
		indexed   any
		document  any
		snapshot  deltaSyncSnapshot
	}{
		{
			indexName: DoctorsIndexName,
			id:        testOldDoctorID,
			indexed:   DoctorDocument{ID: testOldDoctorID, Visits: []VisitSummary{{ID: testStaleVisitID}}},
			document:  DoctorDocument{ID: testOldDoctorID, FullName: "Current Doctor", Visits: []VisitSummary{{ID: testCurrentVisitID}}},
			snapshot: deltaSyncSnapshot{
				doctors:   map[string]DoctorDocument{testOldDoctorID: {ID: testOldDoctorID, FullName: "Current Doctor", Visits: []VisitSummary{{ID: testCurrentVisitID}}}},
				relations: relationIDs([]string{testOldDoctorID}, nil, nil),
			},
		},
		{
			indexName: PatientsIndexName,
			id:        testOldPatientID,
			indexed:   PatientDocument{ID: testOldPatientID, Visits: []VisitSummary{{ID: testStaleVisitID}}},
			document:  PatientDocument{ID: testOldPatientID, FirstName: "Current", Visits: []VisitSummary{{ID: testCurrentVisitID}}},
			snapshot: deltaSyncSnapshot{
				patients:  map[string]PatientDocument{testOldPatientID: {ID: testOldPatientID, FirstName: "Current", Visits: []VisitSummary{{ID: testCurrentVisitID}}}},
				relations: relationIDs(nil, []string{testOldPatientID}, nil),
			},
		},
		{
			indexName: ClinicsIndexName,
			id:        testOldClinicID,
			indexed:   ClinicDocument{ID: testOldClinicID, Visits: []VisitSummary{{ID: testStaleVisitID}}},
			document:  ClinicDocument{ID: testOldClinicID, Name: "Current Clinic", Visits: []VisitSummary{{ID: testCurrentVisitID}}},
			snapshot: deltaSyncSnapshot{
				clinics:   map[string]ClinicDocument{testOldClinicID: {ID: testOldClinicID, Name: "Current Clinic", Visits: []VisitSummary{{ID: testCurrentVisitID}}}},
				relations: relationIDs(nil, nil, []string{testOldClinicID}),
			},
		},
	} {
		testCase := testCase
		t.Run(testCase.indexName, func(t *testing.T) {
			t.Parallel()

			currentVisit := VisitDocument{ID: testCurrentVisitID}
			testCase.snapshot.indexName = testCase.indexName
			testCase.snapshot.targets = stringSet(testCase.id)
			testCase.snapshot.visits = map[string]VisitDocument{testCurrentVisitID: currentVisit}
			testCase.snapshot.visitIDs = stringSet(testStaleVisitID, testCurrentVisitID)
			loader := &recordingDeltaLoader{snapshot: testCase.snapshot}
			store := &recordingDeltaStore{indexed: map[string]map[string]any{
				testCase.indexName: {testCase.id: testCase.indexed},
			}}
			synchronizer := &deltaSynchronizer{loader: loader, store: store}

			if err := synchronizer.Sync(t.Context(), testCase.indexName, []string{testCase.id}); err != nil {
				t.Fatalf("Sync() error = %v", err)
			}
			if _, ok := loader.hints.visits[testStaleVisitID]; !ok {
				t.Fatalf("indexed stale visit was not passed to loader: %#v", loader.hints.visits)
			}
			wantCalls := []documentStoreCall{
				{operation: "delete", index: VisitsIndexName, id: testStaleVisitID},
				{operation: "upsert", index: VisitsIndexName, id: testCurrentVisitID, document: currentVisit},
				{operation: "upsert", index: testCase.indexName, id: testCase.id, document: testCase.document},
			}
			if !reflect.DeepEqual(store.calls, wantCalls) {
				t.Fatalf("document calls = %#v, want %#v", store.calls, wantCalls)
			}
		})
	}
}

func TestDeltaSynchronizerMissingEntityDeletesStaleVisitsAndPrimary(t *testing.T) {
	t.Parallel()

	for _, testCase := range []struct {
		indexName string
		id        string
		indexed   any
		relations visitRelationIDs
	}{
		{DoctorsIndexName, testOldDoctorID, DoctorDocument{ID: testOldDoctorID, Visits: []VisitSummary{{ID: testStaleVisitID}}}, relationIDs([]string{testOldDoctorID}, nil, nil)},
		{PatientsIndexName, testOldPatientID, PatientDocument{ID: testOldPatientID, Visits: []VisitSummary{{ID: testStaleVisitID}}}, relationIDs(nil, []string{testOldPatientID}, nil)},
		{ClinicsIndexName, testOldClinicID, ClinicDocument{ID: testOldClinicID, Visits: []VisitSummary{{ID: testStaleVisitID}}}, relationIDs(nil, nil, []string{testOldClinicID})},
	} {
		testCase := testCase
		t.Run(testCase.indexName, func(t *testing.T) {
			t.Parallel()

			loader := &recordingDeltaLoader{snapshot: deltaSyncSnapshot{
				indexName: testCase.indexName,
				targets:   stringSet(testCase.id),
				visits:    map[string]VisitDocument{},
				visitIDs:  stringSet(testStaleVisitID),
				doctors:   map[string]DoctorDocument{},
				patients:  map[string]PatientDocument{},
				clinics:   map[string]ClinicDocument{},
				relations: testCase.relations,
			}}
			store := &recordingDeltaStore{indexed: map[string]map[string]any{
				testCase.indexName: {testCase.id: testCase.indexed},
			}}

			if err := (&deltaSynchronizer{loader: loader, store: store}).Sync(t.Context(), testCase.indexName, []string{testCase.id}); err != nil {
				t.Fatalf("Sync() error = %v", err)
			}
			want := []documentStoreCall{
				{operation: "delete", index: VisitsIndexName, id: testStaleVisitID},
				{operation: "delete", index: testCase.indexName, id: testCase.id},
			}
			if !reflect.DeepEqual(store.calls, want) {
				t.Fatalf("document calls = %#v, want %#v", store.calls, want)
			}
		})
	}
}

func TestDeltaSynchronizerVisitRefreshesOldAndCurrentRelationsThenVisit(t *testing.T) {
	t.Parallel()

	indexed := VisitDocument{ID: testVisitID, DoctorID: testOldDoctorID, PatientID: testOldPatientID, ClinicID: testOldClinicID}
	current := VisitDocument{ID: testVisitID, DoctorID: testNewDoctorID, PatientID: testNewPatientID, ClinicID: testNewClinicID}
	relations := relationIDs(
		[]string{testOldDoctorID, testNewDoctorID},
		[]string{testOldPatientID, testNewPatientID},
		[]string{testOldClinicID, testNewClinicID},
	)
	snapshot := deltaSyncSnapshot{
		indexName: VisitsIndexName,
		targets:   stringSet(testVisitID),
		visits:    map[string]VisitDocument{testVisitID: current},
		visitIDs:  stringSet(testVisitID),
		doctors: map[string]DoctorDocument{
			testOldDoctorID: {ID: testOldDoctorID}, testNewDoctorID: {ID: testNewDoctorID},
		},
		patients: map[string]PatientDocument{
			testOldPatientID: {ID: testOldPatientID}, testNewPatientID: {ID: testNewPatientID},
		},
		clinics: map[string]ClinicDocument{
			testOldClinicID: {ID: testOldClinicID}, testNewClinicID: {ID: testNewClinicID},
		},
		relations: relations,
	}
	loader := &recordingDeltaLoader{snapshot: snapshot}
	store := &recordingDeltaStore{indexed: map[string]map[string]any{
		VisitsIndexName: {testVisitID: indexed},
	}}

	if err := (&deltaSynchronizer{loader: loader, store: store}).Sync(t.Context(), VisitsIndexName, []string{testVisitID}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if !reflect.DeepEqual(loader.hints.relations, relationIDs(
		[]string{testOldDoctorID}, []string{testOldPatientID}, []string{testOldClinicID},
	)) {
		t.Fatalf("loader hints = %#v", loader.hints.relations)
	}
	want := []documentStoreCall{
		{operation: "upsert", index: DoctorsIndexName, id: testOldDoctorID, document: snapshot.doctors[testOldDoctorID]},
		{operation: "upsert", index: DoctorsIndexName, id: testNewDoctorID, document: snapshot.doctors[testNewDoctorID]},
		{operation: "upsert", index: PatientsIndexName, id: testOldPatientID, document: snapshot.patients[testOldPatientID]},
		{operation: "upsert", index: PatientsIndexName, id: testNewPatientID, document: snapshot.patients[testNewPatientID]},
		{operation: "upsert", index: ClinicsIndexName, id: testOldClinicID, document: snapshot.clinics[testOldClinicID]},
		{operation: "upsert", index: ClinicsIndexName, id: testNewClinicID, document: snapshot.clinics[testNewClinicID]},
		{operation: "upsert", index: VisitsIndexName, id: testVisitID, document: current},
	}
	if !reflect.DeepEqual(store.calls, want) {
		t.Fatalf("document calls = %#v, want %#v", store.calls, want)
	}
}

func TestDeltaSynchronizerPatientRefreshesAffectedDoctorAndClinic(t *testing.T) {
	t.Parallel()

	updatedSummary := VisitSummary{ID: testVisitID, PatientID: testOldPatientID, PatientFullName: "Updated Patient"}
	snapshot := deltaSyncSnapshot{
		indexName: PatientsIndexName,
		targets:   stringSet(testOldPatientID),
		visits:    map[string]VisitDocument{testVisitID: {ID: testVisitID}},
		visitIDs:  stringSet(testVisitID),
		doctors: map[string]DoctorDocument{
			testNewDoctorID: {ID: testNewDoctorID, Visits: []VisitSummary{updatedSummary}},
		},
		patients: map[string]PatientDocument{
			testOldPatientID: {ID: testOldPatientID, Visits: []VisitSummary{updatedSummary}},
		},
		clinics: map[string]ClinicDocument{
			testNewClinicID: {ID: testNewClinicID, Visits: []VisitSummary{updatedSummary}},
		},
		relations: relationIDs([]string{testNewDoctorID}, []string{testOldPatientID}, []string{testNewClinicID}),
	}
	store := &recordingDeltaStore{indexed: map[string]map[string]any{
		PatientsIndexName: {testOldPatientID: PatientDocument{ID: testOldPatientID}},
	}}

	if err := (&deltaSynchronizer{loader: &recordingDeltaLoader{snapshot: snapshot}, store: store}).Sync(t.Context(), PatientsIndexName, []string{testOldPatientID}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	want := []documentStoreCall{
		{operation: "upsert", index: VisitsIndexName, id: testVisitID, document: snapshot.visits[testVisitID]},
		{operation: "upsert", index: DoctorsIndexName, id: testNewDoctorID, document: snapshot.doctors[testNewDoctorID]},
		{operation: "upsert", index: PatientsIndexName, id: testOldPatientID, document: snapshot.patients[testOldPatientID]},
		{operation: "upsert", index: ClinicsIndexName, id: testNewClinicID, document: snapshot.clinics[testNewClinicID]},
	}
	if !reflect.DeepEqual(store.calls, want) {
		t.Fatalf("document calls = %#v, want %#v", store.calls, want)
	}
}

func TestDeltaSynchronizerMissingVisitRefreshesRelationsThenDeletesVisit(t *testing.T) {
	t.Parallel()

	indexed := VisitDocument{ID: testVisitID, DoctorID: testOldDoctorID, PatientID: testOldPatientID, ClinicID: testOldClinicID}
	snapshot := deltaSyncSnapshot{
		indexName: VisitsIndexName,
		targets:   stringSet(testVisitID),
		visits:    map[string]VisitDocument{},
		visitIDs:  stringSet(testVisitID),
		doctors:   map[string]DoctorDocument{testOldDoctorID: {ID: testOldDoctorID}},
		patients:  map[string]PatientDocument{testOldPatientID: {ID: testOldPatientID}},
		clinics:   map[string]ClinicDocument{testOldClinicID: {ID: testOldClinicID}},
		relations: relationIDs([]string{testOldDoctorID}, []string{testOldPatientID}, []string{testOldClinicID}),
	}
	store := &recordingDeltaStore{indexed: map[string]map[string]any{
		VisitsIndexName: {testVisitID: indexed},
	}}

	if err := (&deltaSynchronizer{loader: &recordingDeltaLoader{snapshot: snapshot}, store: store}).Sync(t.Context(), VisitsIndexName, []string{testVisitID}); err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	last := store.calls[len(store.calls)-1]
	if last.operation != "delete" || last.index != VisitsIndexName || last.id != testVisitID {
		t.Fatalf("last document call = %#v, want visit delete", last)
	}
}

func TestDeltaSynchronizerStopsBeforeWritesWhenIndexedHintLoadFails(t *testing.T) {
	t.Parallel()

	failure := errors.New("Elasticsearch unavailable")
	store := &recordingDeltaStore{getErr: failure}
	loader := &recordingDeltaLoader{}
	err := (&deltaSynchronizer{loader: loader, store: store}).Sync(t.Context(), VisitsIndexName, []string{testVisitID})
	if !errors.Is(err, failure) {
		t.Fatalf("Sync() error = %v, want wrapped failure", err)
	}
	if loader.calls != 0 || len(store.calls) != 0 {
		t.Fatalf("failed hint load continued: loader calls=%d writes=%d", loader.calls, len(store.calls))
	}
}

type recordingDeltaLoader struct {
	snapshot deltaSyncSnapshot
	err      error
	calls    int
	index    string
	ids      []string
	hints    deltaSyncHints
}

func (l *recordingDeltaLoader) Load(_ context.Context, indexName string, ids []string, hints deltaSyncHints) (deltaSyncSnapshot, error) {
	l.calls++
	l.index = indexName
	l.ids = append([]string(nil), ids...)
	l.hints = cloneDeltaSyncHints(hints)
	return l.snapshot, l.err
}

type recordingDeltaStore struct {
	indexed map[string]map[string]any
	getErr  error
	gets    []documentStoreCall
	calls   []documentStoreCall
}

func (s *recordingDeltaStore) GetDocument(_ context.Context, index, id string, destination any) (bool, error) {
	s.gets = append(s.gets, documentStoreCall{operation: "get", index: index, id: id})
	if s.getErr != nil {
		return false, s.getErr
	}
	document, ok := s.indexed[index][id]
	if !ok {
		return false, nil
	}
	switch value := destination.(type) {
	case *DoctorDocument:
		*value = document.(DoctorDocument)
	case *PatientDocument:
		*value = document.(PatientDocument)
	case *ClinicDocument:
		*value = document.(ClinicDocument)
	case *VisitDocument:
		*value = document.(VisitDocument)
	default:
		return false, errors.New("unexpected document destination")
	}
	return true, nil
}

func (s *recordingDeltaStore) UpsertDocument(_ context.Context, index, id string, document any) error {
	s.calls = append(s.calls, documentStoreCall{operation: "upsert", index: index, id: id, document: document})
	return nil
}

func (s *recordingDeltaStore) DeleteDocument(_ context.Context, index, id string) error {
	s.calls = append(s.calls, documentStoreCall{operation: "delete", index: index, id: id})
	return nil
}

func stringSet(ids ...string) map[string]struct{} {
	values := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		values[id] = struct{}{}
	}
	return values
}

func cloneDeltaSyncHints(hints deltaSyncHints) deltaSyncHints {
	cloned := newDeltaSyncHints()
	for id := range hints.visits {
		cloned.visits[id] = struct{}{}
	}
	cloned.relations = cloneVisitRelationIDs(hints.relations)
	return cloned
}
