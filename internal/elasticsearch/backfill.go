package elasticsearch

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
)

type backfillRows struct {
	doctors  []sqlc.ListDoctorsForElasticsearchBackfillRow
	patients []sqlc.ListPatientsForElasticsearchBackfillRow
	clinics  []sqlc.ListClinicsForElasticsearchBackfillRow
	visits   []sqlc.ListVisitsForElasticsearchBackfillRow
}

type backfillDocuments struct {
	doctors  []DoctorDocument
	patients []PatientDocument
	clinics  []ClinicDocument
	visits   []VisitDocument
}

type documentUpserter interface {
	UpsertDocument(context.Context, string, string, any) error
}

// Backfill loads one consistent PostgreSQL snapshot and indexes every current
// doctor, patient, clinic, and visit document in Elasticsearch.
func Backfill(ctx context.Context, pool *pgxpool.Pool, client *Client) error {
	rows, err := loadBackfillRows(ctx, pool)
	if err != nil {
		return fmt.Errorf("load PostgreSQL backfill snapshot: %w", err)
	}

	documents, err := buildBackfillDocuments(rows)
	if err != nil {
		return fmt.Errorf("build Elasticsearch backfill documents: %w", err)
	}

	if err := upsertBackfillDocuments(ctx, client, documents); err != nil {
		return fmt.Errorf("upsert Elasticsearch backfill documents: %w", err)
	}

	return nil
}

// BackfillIndex loads one consistent PostgreSQL snapshot, builds the existing
// enriched read models, and upserts documents only into indexName.
func BackfillIndex(ctx context.Context, pool *pgxpool.Pool, client *Client, indexName string) error {
	if err := ValidateIndexName(indexName); err != nil {
		return err
	}

	rows, err := loadBackfillRows(ctx, pool)
	if err != nil {
		return fmt.Errorf("load PostgreSQL backfill snapshot: %w", err)
	}

	documents, err := buildBackfillDocuments(rows)
	if err != nil {
		return fmt.Errorf("build Elasticsearch backfill documents: %w", err)
	}

	if err := upsertBackfillDocumentsForIndex(ctx, client, indexName, documents); err != nil {
		return fmt.Errorf("upsert Elasticsearch backfill documents: %w", err)
	}

	return nil
}

func loadBackfillRows(ctx context.Context, pool *pgxpool.Pool) (backfillRows, error) {
	transaction, err := pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return backfillRows{}, fmt.Errorf("begin read-only repeatable-read transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	queries := sqlc.New(transaction)
	rows := backfillRows{}

	rows.doctors, err = queries.ListDoctorsForElasticsearchBackfill(ctx)
	if err != nil {
		return backfillRows{}, fmt.Errorf("list doctors: %w", err)
	}

	rows.patients, err = queries.ListPatientsForElasticsearchBackfill(ctx)
	if err != nil {
		return backfillRows{}, fmt.Errorf("list patients: %w", err)
	}

	rows.clinics, err = queries.ListClinicsForElasticsearchBackfill(ctx)
	if err != nil {
		return backfillRows{}, fmt.Errorf("list clinics: %w", err)
	}

	rows.visits, err = queries.ListVisitsForElasticsearchBackfill(ctx)
	if err != nil {
		return backfillRows{}, fmt.Errorf("list visits: %w", err)
	}

	if err := transaction.Commit(ctx); err != nil {
		return backfillRows{}, fmt.Errorf("commit read-only repeatable-read transaction: %w", err)
	}

	return rows, nil
}

func buildBackfillDocuments(rows backfillRows) (backfillDocuments, error) {
	documents := backfillDocuments{
		doctors:  make([]DoctorDocument, 0, len(rows.doctors)),
		patients: make([]PatientDocument, 0, len(rows.patients)),
		clinics:  make([]ClinicDocument, 0, len(rows.clinics)),
		visits:   make([]VisitDocument, 0, len(rows.visits)),
	}
	doctorPositions := make(map[string]int, len(rows.doctors))
	patientPositions := make(map[string]int, len(rows.patients))
	clinicPositions := make(map[string]int, len(rows.clinics))

	for _, row := range rows.doctors {
		createdAt, err := requiredTimestamp("doctor", row.ID, "created_at", row.CreatedAt)
		if err != nil {
			return backfillDocuments{}, err
		}
		updatedAt, err := requiredTimestamp("doctor", row.ID, "updated_at", row.UpdatedAt)
		if err != nil {
			return backfillDocuments{}, err
		}

		doctorPositions[row.ID] = len(documents.doctors)
		documents.doctors = append(documents.doctors, DoctorDocument{
			ID:          row.ID,
			SpecialtyID: row.SpecialtyID,
			ClinicID:    row.ClinicID,
			FullName:    row.FullName,
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
			Visits:      make([]VisitSummary, 0),
		})
	}

	for _, row := range rows.patients {
		dateOfBirth, err := requiredDate("patient", row.ID, "date_of_birth", row.DateOfBirth)
		if err != nil {
			return backfillDocuments{}, err
		}
		createdAt, err := requiredTimestamp("patient", row.ID, "created_at", row.CreatedAt)
		if err != nil {
			return backfillDocuments{}, err
		}
		updatedAt, err := requiredTimestamp("patient", row.ID, "updated_at", row.UpdatedAt)
		if err != nil {
			return backfillDocuments{}, err
		}

		patientPositions[row.ID] = len(documents.patients)
		documents.patients = append(documents.patients, PatientDocument{
			ID:          row.ID,
			FirstName:   row.FirstName,
			LastName:    row.LastName,
			DateOfBirth: dateOfBirth,
			Gender:      row.Gender,
			IsDeleted:   nullableBool(row.IsDeleted),
			CreatedAt:   createdAt,
			UpdatedAt:   updatedAt,
			Visits:      make([]VisitSummary, 0),
		})
	}

	for _, row := range rows.clinics {
		createdAt, err := requiredTimestamp("clinic", row.ID, "created_at", row.CreatedAt)
		if err != nil {
			return backfillDocuments{}, err
		}
		updatedAt, err := requiredTimestamp("clinic", row.ID, "updated_at", row.UpdatedAt)
		if err != nil {
			return backfillDocuments{}, err
		}

		clinicPositions[row.ID] = len(documents.clinics)
		documents.clinics = append(documents.clinics, ClinicDocument{
			ID:        row.ID,
			Name:      row.Name,
			Address:   row.Address,
			TimeZone:  row.TimeZone,
			CreatedAt: createdAt,
			UpdatedAt: updatedAt,
			Visits:    make([]VisitSummary, 0),
		})
	}

	for _, row := range rows.visits {
		doctorPosition, ok := doctorPositions[row.DoctorID]
		if !ok {
			return backfillDocuments{}, fmt.Errorf("visit %s references missing doctor %s", row.ID, row.DoctorID)
		}
		patientPosition, ok := patientPositions[row.PatientID]
		if !ok {
			return backfillDocuments{}, fmt.Errorf("visit %s references missing patient %s", row.ID, row.PatientID)
		}
		clinicPosition, ok := clinicPositions[row.ClinicID]
		if !ok {
			return backfillDocuments{}, fmt.Errorf("visit %s references missing clinic %s", row.ID, row.ClinicID)
		}

		visitStartTime, err := requiredTimestamp("visit", row.ID, "visit_start_time", row.VisitStartTime)
		if err != nil {
			return backfillDocuments{}, err
		}
		visitEndTime, err := requiredTimestamp("visit", row.ID, "visit_end_time", row.VisitEndTime)
		if err != nil {
			return backfillDocuments{}, err
		}
		createdAt, err := requiredTimestamp("visit", row.ID, "created_at", row.CreatedAt)
		if err != nil {
			return backfillDocuments{}, err
		}
		updatedAt, err := requiredTimestamp("visit", row.ID, "updated_at", row.UpdatedAt)
		if err != nil {
			return backfillDocuments{}, err
		}
		patientDateOfBirth, err := requiredDate("visit", row.ID, "patient.date_of_birth", row.PatientDateOfBirth)
		if err != nil {
			return backfillDocuments{}, err
		}

		summary := VisitSummary{
			ID:             row.ID,
			DoctorID:       row.DoctorID,
			PatientID:      row.PatientID,
			ClinicID:       row.ClinicID,
			Status:         row.Status,
			VisitStartTime: visitStartTime,
			VisitEndTime:   visitEndTime,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
		}
		documents.doctors[doctorPosition].Visits = append(documents.doctors[doctorPosition].Visits, summary)
		documents.patients[patientPosition].Visits = append(documents.patients[patientPosition].Visits, summary)
		documents.clinics[clinicPosition].Visits = append(documents.clinics[clinicPosition].Visits, summary)

		documents.visits = append(documents.visits, VisitDocument{
			ID:             row.ID,
			DoctorID:       row.DoctorID,
			PatientID:      row.PatientID,
			ClinicID:       row.ClinicID,
			Status:         row.Status,
			VisitStartTime: visitStartTime,
			VisitEndTime:   visitEndTime,
			CreatedAt:      createdAt,
			UpdatedAt:      updatedAt,
			Doctor: VisitDoctorData{
				ID:          row.DoctorID,
				SpecialtyID: row.DoctorSpecialtyID,
				ClinicID:    row.DoctorClinicID,
				FullName:    row.DoctorFullName,
			},
			Patient: VisitPatientData{
				ID:          row.PatientID,
				FirstName:   row.PatientFirstName,
				LastName:    row.PatientLastName,
				DateOfBirth: patientDateOfBirth,
				Gender:      row.PatientGender,
				IsDeleted:   nullableBool(row.PatientIsDeleted),
			},
			Clinic: VisitClinicData{
				ID:       row.ClinicID,
				Name:     row.ClinicName,
				Address:  row.ClinicAddress,
				TimeZone: row.ClinicTimeZone,
			},
		})
	}

	return documents, nil
}

func upsertBackfillDocuments(ctx context.Context, client documentUpserter, documents backfillDocuments) error {
	for _, indexName := range []string{DoctorsIndexName, PatientsIndexName, ClinicsIndexName, VisitsIndexName} {
		if err := upsertBackfillDocumentsForIndex(ctx, client, indexName, documents); err != nil {
			return err
		}
	}

	return nil
}

func upsertBackfillDocumentsForIndex(ctx context.Context, client documentUpserter, indexName string, documents backfillDocuments) error {
	switch indexName {
	case DoctorsIndexName:
		return upsertDocuments(ctx, client, indexName, "doctor", documents.doctors, func(document DoctorDocument) string { return document.ID })
	case PatientsIndexName:
		return upsertDocuments(ctx, client, indexName, "patient", documents.patients, func(document PatientDocument) string { return document.ID })
	case ClinicsIndexName:
		return upsertDocuments(ctx, client, indexName, "clinic", documents.clinics, func(document ClinicDocument) string { return document.ID })
	case VisitsIndexName:
		return upsertDocuments(ctx, client, indexName, "visit", documents.visits, func(document VisitDocument) string { return document.ID })
	default:
		return ValidateIndexName(indexName)
	}
}

func upsertDocuments[T any](ctx context.Context, client documentUpserter, indexName, entityName string, documents []T, id func(T) string) error {
	for _, document := range documents {
		documentID := id(document)
		if err := client.UpsertDocument(ctx, indexName, documentID, document); err != nil {
			return fmt.Errorf("upsert %s %s: %w", entityName, documentID, err)
		}
	}

	return nil
}

func requiredTimestamp(entity, id, field string, value pgtype.Timestamptz) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, fmt.Errorf("%s %s has null %s", entity, id, field)
	}

	return value.Time, nil
}

func requiredDate(entity, id, field string, value pgtype.Date) (time.Time, error) {
	if !value.Valid {
		return time.Time{}, fmt.Errorf("%s %s has null %s", entity, id, field)
	}

	return value.Time, nil
}

func nullableBool(value pgtype.Bool) *bool {
	if !value.Valid {
		return nil
	}

	return &value.Bool
}
