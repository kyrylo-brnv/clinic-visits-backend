package elasticsearch

import (
	"context"
	"errors"
	"reflect"
	"strings"
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
				Gender: "Male", IsDeleted: true, CreatedAt: timestampValue(createdAt), UpdatedAt: timestampValue(updatedAt),
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
				PatientGender: "Male", PatientIsDeleted: true,
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
				IsDeleted: true, CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: []VisitSummary{summary},
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
					Gender: "Male", IsDeleted: true,
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
