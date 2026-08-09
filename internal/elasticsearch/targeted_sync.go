package elasticsearch

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
	"github.com/smithautotest/clinic-visits/internal/uuid"
)

// MaxSyncIndexIDs bounds one targeted synchronization request.
const MaxSyncIndexIDs = 100

type deltaSyncSnapshotLoader interface {
	Load(context.Context, string, []string, deltaSyncHints) (deltaSyncSnapshot, error)
}

type deltaSyncHints struct {
	visits    map[string]struct{}
	relations visitRelationIDs
}

type deltaSyncSnapshot struct {
	indexName string
	targets   map[string]struct{}
	visits    map[string]VisitDocument
	visitIDs  map[string]struct{}
	doctors   map[string]DoctorDocument
	patients  map[string]PatientDocument
	clinics   map[string]ClinicDocument
	relations visitRelationIDs
}

type postgresDeltaSyncSnapshotLoader struct {
	pool *pgxpool.Pool
}

type deltaSynchronizer struct {
	loader deltaSyncSnapshotLoader
	store  visitDocumentStore
}

// ValidateSyncIndexRequest validates and deterministically de-duplicates one
// targeted synchronization request.
func ValidateSyncIndexRequest(indexName string, ids []string) ([]string, error) {
	if err := ValidateIndexName(indexName); err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return nil, fmt.Errorf("expected at least one PostgreSQL UUID ID")
	}
	if len(ids) > MaxSyncIndexIDs {
		return nil, fmt.Errorf("expected at most %d PostgreSQL UUID IDs", MaxSyncIndexIDs)
	}

	unique := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		parsedID, err := uuid.Parse(id)
		if err != nil {
			return nil, fmt.Errorf("invalid PostgreSQL UUID ID %q: %w", id, err)
		}
		if parsedID.String() != id {
			return nil, fmt.Errorf("invalid PostgreSQL UUID ID %q: expected canonical UUID format", id)
		}
		unique[id] = struct{}{}
	}

	return sortedKeys(unique), nil
}

// SyncIndex synchronizes the requested PostgreSQL entities and every directly
// dependent denormalized Elasticsearch document without rebuilding an index.
func SyncIndex(ctx context.Context, pool *pgxpool.Pool, client *Client, indexName string, ids []string) error {
	targetIDs, err := ValidateSyncIndexRequest(indexName, ids)
	if err != nil {
		return err
	}

	synchronizer := &deltaSynchronizer{
		loader: &postgresDeltaSyncSnapshotLoader{pool: pool},
		store:  client,
	}
	if err := synchronizer.Sync(ctx, indexName, targetIDs); err != nil {
		return fmt.Errorf("synchronize Elasticsearch index %s: %w", indexName, err)
	}
	return nil
}

func (s *deltaSynchronizer) Sync(ctx context.Context, indexName string, ids []string) error {
	targetIDs, err := ValidateSyncIndexRequest(indexName, ids)
	if err != nil {
		return err
	}

	hints, err := s.loadIndexedHints(ctx, indexName, targetIDs)
	if err != nil {
		return err
	}
	snapshot, err := s.loader.Load(ctx, indexName, targetIDs, hints)
	if err != nil {
		return fmt.Errorf("load PostgreSQL delta snapshot: %w", err)
	}

	if indexName == VisitsIndexName {
		if err := syncDeltaRelatedDocuments(ctx, s.store, snapshot); err != nil {
			return err
		}
		return syncDeltaVisitDocuments(ctx, s.store, snapshot)
	}

	// Entity documents are kept last so a retry can still discover visits that
	// disappeared or moved since the entity was last indexed.
	if err := syncDeltaVisitDocuments(ctx, s.store, snapshot); err != nil {
		return err
	}
	return syncDeltaRelatedDocuments(ctx, s.store, snapshot)
}

func (s *deltaSynchronizer) loadIndexedHints(ctx context.Context, indexName string, ids []string) (deltaSyncHints, error) {
	hints := newDeltaSyncHints()
	for _, id := range ids {
		switch indexName {
		case DoctorsIndexName:
			var document DoctorDocument
			found, err := s.store.GetDocument(ctx, indexName, id, &document)
			if err != nil {
				return deltaSyncHints{}, fmt.Errorf("load currently indexed doctor %s: %w", id, err)
			}
			if found {
				addVisitSummaryIDs(hints.visits, document.Visits)
			}
		case PatientsIndexName:
			var document PatientDocument
			found, err := s.store.GetDocument(ctx, indexName, id, &document)
			if err != nil {
				return deltaSyncHints{}, fmt.Errorf("load currently indexed patient %s: %w", id, err)
			}
			if found {
				addVisitSummaryIDs(hints.visits, document.Visits)
			}
		case ClinicsIndexName:
			var document ClinicDocument
			found, err := s.store.GetDocument(ctx, indexName, id, &document)
			if err != nil {
				return deltaSyncHints{}, fmt.Errorf("load currently indexed clinic %s: %w", id, err)
			}
			if found {
				addVisitSummaryIDs(hints.visits, document.Visits)
			}
		case VisitsIndexName:
			var document VisitDocument
			found, err := s.store.GetDocument(ctx, indexName, id, &document)
			if err != nil {
				return deltaSyncHints{}, fmt.Errorf("load currently indexed visit %s: %w", id, err)
			}
			if found {
				hints.relations.addVisit(document)
			}
		}
	}
	return hints, nil
}

func (l *postgresDeltaSyncSnapshotLoader) Load(
	ctx context.Context,
	indexName string,
	targetIDs []string,
	hints deltaSyncHints,
) (deltaSyncSnapshot, error) {
	if err := ValidateIndexName(indexName); err != nil {
		return deltaSyncSnapshot{}, err
	}

	transaction, err := l.pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return deltaSyncSnapshot{}, fmt.Errorf("begin read-only repeatable-read transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	queries := sqlc.New(transaction)
	snapshot := newDeltaSyncSnapshot(indexName, targetIDs)

	if indexName == VisitsIndexName {
		snapshot.relations = cloneVisitRelationIDs(hints.relations)
		if err := loadDeltaVisitDocuments(ctx, queries, &snapshot, snapshot.targets); err != nil {
			return deltaSyncSnapshot{}, err
		}
		for _, document := range snapshot.visits {
			snapshot.relations.addVisit(document)
		}
		if err := loadDeltaRelatedDocuments(ctx, queries, &snapshot); err != nil {
			return deltaSyncSnapshot{}, err
		}
	} else {
		addDeltaEntityTargets(&snapshot, indexName)
		if err := loadDeltaRelatedDocuments(ctx, queries, &snapshot); err != nil {
			return deltaSyncSnapshot{}, err
		}
		for id := range hints.visits {
			snapshot.visitIDs[id] = struct{}{}
		}
		if err := loadDeltaVisitDocuments(ctx, queries, &snapshot, snapshot.visitIDs); err != nil {
			return deltaSyncSnapshot{}, err
		}
	}

	if err := transaction.Commit(ctx); err != nil {
		return deltaSyncSnapshot{}, fmt.Errorf("commit read-only repeatable-read transaction: %w", err)
	}
	return snapshot, nil
}

func loadDeltaRelatedDocuments(ctx context.Context, queries *sqlc.Queries, snapshot *deltaSyncSnapshot) error {
	doctorIDs, err := parseSortedUUIDs(snapshot.relations.doctors)
	if err != nil {
		return fmt.Errorf("parse related doctor IDs: %w", err)
	}
	patientIDs, err := parseSortedUUIDs(snapshot.relations.patients)
	if err != nil {
		return fmt.Errorf("parse related patient IDs: %w", err)
	}
	clinicIDs, err := parseSortedUUIDs(snapshot.relations.clinics)
	if err != nil {
		return fmt.Errorf("parse related clinic IDs: %w", err)
	}

	doctorRows, err := queries.ListDoctorsForElasticsearchSync(ctx, doctorIDs)
	if err != nil {
		return fmt.Errorf("list related doctors: %w", err)
	}
	patientRows, err := queries.ListPatientsForElasticsearchSync(ctx, patientIDs)
	if err != nil {
		return fmt.Errorf("list related patients: %w", err)
	}
	clinicRows, err := queries.ListClinicsForElasticsearchSync(ctx, clinicIDs)
	if err != nil {
		return fmt.Errorf("list related clinics: %w", err)
	}
	visitRows, err := queries.ListVisitSummariesForElasticsearchSync(ctx, sqlc.ListVisitSummariesForElasticsearchSyncParams{
		DoctorIds: doctorIDs, PatientIds: patientIDs, ClinicIds: clinicIDs,
	})
	if err != nil {
		return fmt.Errorf("list related visits: %w", err)
	}

	snapshot.doctors, err = mapSyncDoctorDocuments(doctorRows)
	if err != nil {
		return err
	}
	snapshot.patients, err = mapSyncPatientDocuments(patientRows)
	if err != nil {
		return err
	}
	snapshot.clinics, err = mapSyncClinicDocuments(clinicRows)
	if err != nil {
		return err
	}
	if err := addSyncVisitSummaries(visitSyncSnapshot{
		doctors: snapshot.doctors, patients: snapshot.patients, clinics: snapshot.clinics,
	}, visitRows); err != nil {
		return err
	}
	if snapshot.indexName != VisitsIndexName {
		for _, row := range visitRows {
			snapshot.visitIDs[row.ID] = struct{}{}
		}
	}
	return nil
}

func loadDeltaVisitDocuments(
	ctx context.Context,
	queries *sqlc.Queries,
	snapshot *deltaSyncSnapshot,
	ids map[string]struct{},
) error {
	for _, id := range sortedKeys(ids) {
		parsedID, err := uuid.Parse(id)
		if err != nil {
			return fmt.Errorf("parse visit ID %q: %w", id, err)
		}
		row, err := queries.GetVisitForElasticsearchSync(ctx, parsedID)
		if errors.Is(err, pgx.ErrNoRows) {
			continue
		}
		if err != nil {
			return fmt.Errorf("get visit %s: %w", id, err)
		}
		document, err := mapSyncVisitDocument(row)
		if err != nil {
			return err
		}
		snapshot.visits[id] = document
	}
	return nil
}

func syncDeltaRelatedDocuments(ctx context.Context, store visitDocumentStore, snapshot deltaSyncSnapshot) error {
	return syncRelatedDocuments(ctx, store, visitSyncSnapshot{
		doctors: snapshot.doctors, patients: snapshot.patients, clinics: snapshot.clinics,
		relations: snapshot.relations,
	})
}

func syncDeltaVisitDocuments(ctx context.Context, store visitDocumentStore, snapshot deltaSyncSnapshot) error {
	for _, id := range sortedKeys(snapshot.visitIDs) {
		document, ok := snapshot.visits[id]
		if !ok {
			if err := store.DeleteDocument(ctx, VisitsIndexName, id); err != nil {
				return fmt.Errorf("delete visit %s: %w", id, err)
			}
			continue
		}
		if err := store.UpsertDocument(ctx, VisitsIndexName, id, document); err != nil {
			return fmt.Errorf("upsert visit %s: %w", id, err)
		}
	}
	return nil
}

func newDeltaSyncHints() deltaSyncHints {
	return deltaSyncHints{visits: make(map[string]struct{}), relations: newVisitRelationIDs()}
}

func newDeltaSyncSnapshot(indexName string, targetIDs []string) deltaSyncSnapshot {
	targets := make(map[string]struct{}, len(targetIDs))
	for _, id := range targetIDs {
		targets[id] = struct{}{}
	}
	visitIDs := make(map[string]struct{})
	if indexName == VisitsIndexName {
		for id := range targets {
			visitIDs[id] = struct{}{}
		}
	}
	return deltaSyncSnapshot{
		indexName: indexName,
		targets:   targets,
		visits:    make(map[string]VisitDocument),
		visitIDs:  visitIDs,
		doctors:   make(map[string]DoctorDocument),
		patients:  make(map[string]PatientDocument),
		clinics:   make(map[string]ClinicDocument),
		relations: newVisitRelationIDs(),
	}
}

func addDeltaEntityTargets(snapshot *deltaSyncSnapshot, indexName string) {
	var destination map[string]struct{}
	switch indexName {
	case DoctorsIndexName:
		destination = snapshot.relations.doctors
	case PatientsIndexName:
		destination = snapshot.relations.patients
	case ClinicsIndexName:
		destination = snapshot.relations.clinics
	}
	for id := range snapshot.targets {
		destination[id] = struct{}{}
	}
}

func addVisitSummaryIDs(destination map[string]struct{}, visits []VisitSummary) {
	for _, visit := range visits {
		destination[visit.ID] = struct{}{}
	}
}

func cloneVisitRelationIDs(ids visitRelationIDs) visitRelationIDs {
	cloned := newVisitRelationIDs()
	for id := range ids.doctors {
		cloned.doctors[id] = struct{}{}
	}
	for id := range ids.patients {
		cloned.patients[id] = struct{}{}
	}
	for id := range ids.clinics {
		cloned.clinics[id] = struct{}{}
	}
	return cloned
}
