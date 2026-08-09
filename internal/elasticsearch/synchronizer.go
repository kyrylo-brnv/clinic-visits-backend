package elasticsearch

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
	"github.com/smithautotest/clinic-visits/internal/outbox"
	"github.com/smithautotest/clinic-visits/internal/uuid"
)

type visitDocumentStore interface {
	GetDocument(context.Context, string, string, any) (bool, error)
	UpsertDocument(context.Context, string, string, any) error
	DeleteDocument(context.Context, string, string) error
}

type visitRelationIDs struct {
	doctors  map[string]struct{}
	patients map[string]struct{}
	clinics  map[string]struct{}
}

type visitEventConsumer struct {
	synchronizer *deltaSynchronizer
}

// NewVisitEventConsumer returns an outbox consumer for visit create, update,
// and delete events. The event identifies the visit; document contents are
// rebuilt from the current PostgreSQL state.
func NewVisitEventConsumer(pool *pgxpool.Pool, client *Client) outbox.Consumer {
	consumer := &visitEventConsumer{
		synchronizer: &deltaSynchronizer{
			loader: &postgresDeltaSyncSnapshotLoader{pool: pool},
			store:  client,
		},
	}
	return consumer.Consume
}

func (c *visitEventConsumer) Consume(ctx context.Context, event outbox.PersistedEvent) error {
	if event.AggregateType != outbox.AggregateTypeVisit {
		return fmt.Errorf("unsupported outbox aggregate type %q", event.AggregateType)
	}
	switch event.EventType {
	case outbox.EventTypeVisitCreated, outbox.EventTypeVisitUpdated, outbox.EventTypeVisitDeleted:
	default:
		return fmt.Errorf("unsupported visit outbox event type %q", event.EventType)
	}
	if _, err := uuid.Parse(event.AggregateID); err != nil {
		return fmt.Errorf("parse visit outbox aggregate ID: %w", err)
	}

	relations, err := relationIDsFromEventPayload(event.Payload, event.AggregateID)
	if err != nil {
		return err
	}
	hints := newDeltaSyncHints()
	hints.relations = relations
	if err := c.synchronizer.syncWithHints(
		ctx,
		VisitsIndexName,
		[]string{event.AggregateID},
		hints,
	); err != nil {
		return fmt.Errorf("synchronize visit %s: %w", event.AggregateID, err)
	}

	return nil
}

func relationIDsFromEventPayload(payload []byte, aggregateID string) (visitRelationIDs, error) {
	var visit struct {
		ID        string `json:"id"`
		DoctorID  string `json:"doctor_id"`
		PatientID string `json:"patient_id"`
		ClinicID  string `json:"clinic_id"`
	}
	if err := json.Unmarshal(payload, &visit); err != nil {
		return visitRelationIDs{}, fmt.Errorf("decode visit outbox event payload: %w", err)
	}
	if visit.ID != aggregateID {
		return visitRelationIDs{}, fmt.Errorf(
			"visit outbox event payload ID %q does not match aggregate ID %q",
			visit.ID,
			aggregateID,
		)
	}

	relations := newVisitRelationIDs()
	relations.doctors[visit.DoctorID] = struct{}{}
	relations.patients[visit.PatientID] = struct{}{}
	relations.clinics[visit.ClinicID] = struct{}{}
	return relations, nil
}

func newVisitRelationIDs() visitRelationIDs {
	return visitRelationIDs{
		doctors:  make(map[string]struct{}),
		patients: make(map[string]struct{}),
		clinics:  make(map[string]struct{}),
	}
}

func (ids visitRelationIDs) addVisit(visit VisitDocument) {
	ids.doctors[visit.DoctorID] = struct{}{}
	ids.patients[visit.PatientID] = struct{}{}
	ids.clinics[visit.ClinicID] = struct{}{}
}

func parseSortedUUIDs(ids map[string]struct{}) ([]pgtype.UUID, error) {
	values := sortedKeys(ids)
	parsed := make([]pgtype.UUID, 0, len(values))
	for _, value := range values {
		id, err := uuid.Parse(value)
		if err != nil {
			return nil, fmt.Errorf("parse %q: %w", value, err)
		}
		parsed = append(parsed, id)
	}
	return parsed, nil
}

func syncRelatedDocuments(
	ctx context.Context,
	store visitDocumentStore,
	doctors map[string]DoctorDocument,
	patients map[string]PatientDocument,
	clinics map[string]ClinicDocument,
	relations visitRelationIDs,
) error {
	for _, id := range sortedKeys(relations.doctors) {
		if document, ok := doctors[id]; ok {
			if err := store.UpsertDocument(ctx, DoctorsIndexName, id, document); err != nil {
				return fmt.Errorf("upsert doctor %s: %w", id, err)
			}
		} else if err := store.DeleteDocument(ctx, DoctorsIndexName, id); err != nil {
			return fmt.Errorf("delete doctor %s: %w", id, err)
		}
	}
	for _, id := range sortedKeys(relations.patients) {
		if document, ok := patients[id]; ok {
			if err := store.UpsertDocument(ctx, PatientsIndexName, id, document); err != nil {
				return fmt.Errorf("upsert patient %s: %w", id, err)
			}
		} else if err := store.DeleteDocument(ctx, PatientsIndexName, id); err != nil {
			return fmt.Errorf("delete patient %s: %w", id, err)
		}
	}
	for _, id := range sortedKeys(relations.clinics) {
		if document, ok := clinics[id]; ok {
			if err := store.UpsertDocument(ctx, ClinicsIndexName, id, document); err != nil {
				return fmt.Errorf("upsert clinic %s: %w", id, err)
			}
		} else if err := store.DeleteDocument(ctx, ClinicsIndexName, id); err != nil {
			return fmt.Errorf("delete clinic %s: %w", id, err)
		}
	}
	return nil
}

func sortedKeys(values map[string]struct{}) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func mapSyncDoctorDocuments(rows []sqlc.ListDoctorsForElasticsearchSyncRow) (map[string]DoctorDocument, error) {
	documents := make(map[string]DoctorDocument, len(rows))
	for _, row := range rows {
		createdAt, err := requiredTimestamp("doctor", row.ID, "created_at", row.CreatedAt)
		if err != nil {
			return nil, err
		}
		updatedAt, err := requiredTimestamp("doctor", row.ID, "updated_at", row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		documents[row.ID] = DoctorDocument{
			ID: row.ID, SpecialtyID: row.SpecialtyID, ClinicID: row.ClinicID, FullName: row.FullName,
			CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: make([]VisitSummary, 0),
		}
	}
	return documents, nil
}

func mapSyncPatientDocuments(rows []sqlc.ListPatientsForElasticsearchSyncRow) (map[string]PatientDocument, error) {
	documents := make(map[string]PatientDocument, len(rows))
	for _, row := range rows {
		dateOfBirth, err := requiredDate("patient", row.ID, "date_of_birth", row.DateOfBirth)
		if err != nil {
			return nil, err
		}
		createdAt, err := requiredTimestamp("patient", row.ID, "created_at", row.CreatedAt)
		if err != nil {
			return nil, err
		}
		updatedAt, err := requiredTimestamp("patient", row.ID, "updated_at", row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		documents[row.ID] = PatientDocument{
			ID: row.ID, FirstName: row.FirstName, LastName: row.LastName, DateOfBirth: dateOfBirth,
			Gender: row.Gender, IsDeleted: nullableBool(row.IsDeleted), CreatedAt: createdAt, UpdatedAt: updatedAt,
			Visits: make([]VisitSummary, 0),
		}
	}
	return documents, nil
}

func mapSyncClinicDocuments(rows []sqlc.ListClinicsForElasticsearchSyncRow) (map[string]ClinicDocument, error) {
	documents := make(map[string]ClinicDocument, len(rows))
	for _, row := range rows {
		createdAt, err := requiredTimestamp("clinic", row.ID, "created_at", row.CreatedAt)
		if err != nil {
			return nil, err
		}
		updatedAt, err := requiredTimestamp("clinic", row.ID, "updated_at", row.UpdatedAt)
		if err != nil {
			return nil, err
		}
		documents[row.ID] = ClinicDocument{
			ID: row.ID, Name: row.Name, Address: row.Address, TimeZone: row.TimeZone,
			CreatedAt: createdAt, UpdatedAt: updatedAt, Visits: make([]VisitSummary, 0),
		}
	}
	return documents, nil
}

func addSyncVisitSummaries(
	doctors map[string]DoctorDocument,
	patients map[string]PatientDocument,
	clinics map[string]ClinicDocument,
	rows []sqlc.ListVisitSummariesForElasticsearchSyncRow,
) error {
	for _, row := range rows {
		visitStartTime, err := requiredTimestamp("visit", row.ID, "visit_start_time", row.VisitStartTime)
		if err != nil {
			return err
		}
		visitEndTime, err := requiredTimestamp("visit", row.ID, "visit_end_time", row.VisitEndTime)
		if err != nil {
			return err
		}
		createdAt, err := requiredTimestamp("visit", row.ID, "created_at", row.CreatedAt)
		if err != nil {
			return err
		}
		updatedAt, err := requiredTimestamp("visit", row.ID, "updated_at", row.UpdatedAt)
		if err != nil {
			return err
		}
		summary := VisitSummary{
			ID: row.ID, DoctorID: row.DoctorID, PatientID: row.PatientID, ClinicID: row.ClinicID,
			Status: row.Status, VisitStartTime: visitStartTime, VisitEndTime: visitEndTime,
			CreatedAt: createdAt, UpdatedAt: updatedAt,
		}
		if document, ok := doctors[row.DoctorID]; ok {
			document.Visits = append(document.Visits, summary)
			doctors[row.DoctorID] = document
		}
		if document, ok := patients[row.PatientID]; ok {
			document.Visits = append(document.Visits, summary)
			patients[row.PatientID] = document
		}
		if document, ok := clinics[row.ClinicID]; ok {
			document.Visits = append(document.Visits, summary)
			clinics[row.ClinicID] = document
		}
	}
	return nil
}

func mapSyncVisitDocument(row sqlc.GetVisitForElasticsearchSyncRow) (VisitDocument, error) {
	visitStartTime, err := requiredTimestamp("visit", row.ID, "visit_start_time", row.VisitStartTime)
	if err != nil {
		return VisitDocument{}, err
	}
	visitEndTime, err := requiredTimestamp("visit", row.ID, "visit_end_time", row.VisitEndTime)
	if err != nil {
		return VisitDocument{}, err
	}
	createdAt, err := requiredTimestamp("visit", row.ID, "created_at", row.CreatedAt)
	if err != nil {
		return VisitDocument{}, err
	}
	updatedAt, err := requiredTimestamp("visit", row.ID, "updated_at", row.UpdatedAt)
	if err != nil {
		return VisitDocument{}, err
	}
	dateOfBirth, err := requiredDate("visit", row.ID, "patient.date_of_birth", row.PatientDateOfBirth)
	if err != nil {
		return VisitDocument{}, err
	}

	return VisitDocument{
		ID: row.ID, DoctorID: row.DoctorID, PatientID: row.PatientID, ClinicID: row.ClinicID,
		Status: row.Status, VisitStartTime: visitStartTime, VisitEndTime: visitEndTime,
		CreatedAt: createdAt, UpdatedAt: updatedAt,
		Doctor: VisitDoctorData{
			ID: row.DoctorID, SpecialtyID: row.DoctorSpecialtyID,
			ClinicID: row.DoctorClinicID, FullName: row.DoctorFullName,
		},
		Patient: VisitPatientData{
			ID: row.PatientID, FirstName: row.PatientFirstName, LastName: row.PatientLastName,
			DateOfBirth: dateOfBirth, Gender: row.PatientGender, IsDeleted: nullableBool(row.PatientIsDeleted),
		},
		Clinic: VisitClinicData{
			ID: row.ClinicID, Name: row.ClinicName, Address: row.ClinicAddress, TimeZone: row.ClinicTimeZone,
		},
	}, nil
}
