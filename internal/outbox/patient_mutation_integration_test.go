//go:build integration

package outbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestPatientMutationsCreateAtomicOutboxEvents(t *testing.T) {
	pool := openOutboxTestPool(t)

	var committedPatientID, rolledBackPatientID string
	t.Cleanup(func() {
		ctx := context.Background()
		if committedPatientID != "" {
			if _, err := pool.Exec(ctx, "DELETE FROM outbox_events WHERE aggregate_id = $1", committedPatientID); err != nil {
				t.Errorf("delete committed patient outbox events: %v", err)
			}
		}
		if rolledBackPatientID != "" {
			if _, err := pool.Exec(ctx, "DELETE FROM outbox_events WHERE aggregate_id = $1", rolledBackPatientID); err != nil {
				t.Errorf("delete rolled-back patient outbox events: %v", err)
			}
		}
	})

	transaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin committed patient mutation transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())

	if err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ('Patient', 'Outbox Created', DATE '1990-01-01', 'Female')
		 RETURNING id::text`,
	).Scan(&committedPatientID); err != nil {
		t.Fatalf("insert patient: %v", err)
	}
	if _, err := transaction.Exec(
		t.Context(),
		"UPDATE patients SET last_name = 'Outbox Updated' WHERE id = $1",
		committedPatientID,
	); err != nil {
		t.Fatalf("update patient: %v", err)
	}
	if _, err := transaction.Exec(t.Context(), "DELETE FROM patients WHERE id = $1", committedPatientID); err != nil {
		t.Fatalf("delete patient: %v", err)
	}

	assertPatientMutationEvents(t, transaction, committedPatientID, map[string]int{
		EventTypePatientCreated: 1,
		EventTypePatientUpdated: 1,
		EventTypePatientDeleted: 1,
	})

	if err := transaction.Commit(t.Context()); err != nil {
		t.Fatalf("commit patient mutations: %v", err)
	}
	assertPatientMutationEvents(t, pool, committedPatientID, map[string]int{
		EventTypePatientCreated: 1,
		EventTypePatientUpdated: 1,
		EventTypePatientDeleted: 1,
	})

	rolledBackTransaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin rolled-back patient mutation transaction: %v", err)
	}
	defer rolledBackTransaction.Rollback(t.Context())

	if err := rolledBackTransaction.QueryRow(
		t.Context(),
		`INSERT INTO patients (first_name, last_name, date_of_birth, gender)
		 VALUES ('Patient', 'Outbox Rolled Back', DATE '1990-01-01', 'Female')
		 RETURNING id::text`,
	).Scan(&rolledBackPatientID); err != nil {
		t.Fatalf("insert rolled-back patient: %v", err)
	}
	assertPatientMutationEvents(t, rolledBackTransaction, rolledBackPatientID, map[string]int{
		EventTypePatientCreated: 1,
	})

	if err := rolledBackTransaction.Rollback(t.Context()); err != nil {
		t.Fatalf("roll back patient mutation: %v", err)
	}

	var patientCount, eventCount int
	if err := pool.QueryRow(
		t.Context(),
		`SELECT
			(SELECT count(*) FROM patients WHERE id = $1),
			(SELECT count(*) FROM outbox_events WHERE aggregate_id = $1)`,
		rolledBackPatientID,
	).Scan(&patientCount, &eventCount); err != nil {
		t.Fatalf("count rolled-back patient and outbox events: %v", err)
	}
	if patientCount != 0 || eventCount != 0 {
		t.Fatalf(
			"expected rolled-back patient and event to be absent, got patients=%d events=%d",
			patientCount,
			eventCount,
		)
	}
}

type patientMutationEventQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func assertPatientMutationEvents(
	t *testing.T,
	querier patientMutationEventQuerier,
	patientID string,
	expectedCounts map[string]int,
) {
	t.Helper()

	rows, err := querier.Query(
		t.Context(),
		`SELECT aggregate_type, aggregate_id::text, event_type, payload, processed_at IS NULL
		 FROM outbox_events
		 WHERE aggregate_id = $1`,
		patientID,
	)
	if err != nil {
		t.Fatalf("query patient outbox events: %v", err)
	}
	defer rows.Close()

	actualCounts := make(map[string]int, len(expectedCounts))
	for rows.Next() {
		var aggregateType, aggregateID, eventType string
		var payload []byte
		var pending bool
		if err := rows.Scan(&aggregateType, &aggregateID, &eventType, &payload, &pending); err != nil {
			t.Fatalf("scan patient outbox event: %v", err)
		}
		if aggregateType != AggregateTypePatient || aggregateID != patientID {
			t.Fatalf(
				"unexpected patient event association: aggregate_type=%q aggregate_id=%q",
				aggregateType,
				aggregateID,
			)
		}
		if !pending {
			t.Fatalf("expected patient event %q to be pending", eventType)
		}

		var decodedPayload struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(payload, &decodedPayload); err != nil {
			t.Fatalf("decode patient %q event payload: %v", eventType, err)
		}
		if decodedPayload.ID != patientID {
			t.Fatalf("patient %q payload ID = %q, want %q", eventType, decodedPayload.ID, patientID)
		}
		actualCounts[eventType]++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate patient outbox events: %v", err)
	}

	if len(actualCounts) != len(expectedCounts) {
		t.Fatalf("patient event counts = %v, want %v", actualCounts, expectedCounts)
	}
	for eventType, expectedCount := range expectedCounts {
		if actualCounts[eventType] != expectedCount {
			t.Fatalf("patient event counts = %v, want %v", actualCounts, expectedCounts)
		}
	}
}
