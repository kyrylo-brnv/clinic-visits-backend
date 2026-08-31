//go:build integration

package outbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestClinicMutationsCreateAtomicOutboxEvents(t *testing.T) {
	pool := openOutboxTestPool(t)

	var committedClinicID, rolledBackClinicID string
	t.Cleanup(func() {
		ctx := context.Background()
		if committedClinicID != "" {
			if _, err := pool.Exec(ctx, "DELETE FROM outbox_events WHERE aggregate_id = $1", committedClinicID); err != nil {
				t.Errorf("delete committed clinic outbox events: %v", err)
			}
		}
		if rolledBackClinicID != "" {
			if _, err := pool.Exec(ctx, "DELETE FROM outbox_events WHERE aggregate_id = $1", rolledBackClinicID); err != nil {
				t.Errorf("delete rolled-back clinic outbox events: %v", err)
			}
		}
	})

	transaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin committed clinic mutation transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())

	if err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO clinics (name, address, time_zone)
		 VALUES ('Clinic Outbox Created', 'Test address', 'Europe/Kyiv')
		 RETURNING id::text`,
	).Scan(&committedClinicID); err != nil {
		t.Fatalf("insert clinic: %v", err)
	}
	if _, err := transaction.Exec(
		t.Context(),
		"UPDATE clinics SET name = 'Clinic Outbox Updated' WHERE id = $1",
		committedClinicID,
	); err != nil {
		t.Fatalf("update clinic: %v", err)
	}
	if _, err := transaction.Exec(t.Context(), "DELETE FROM clinics WHERE id = $1", committedClinicID); err != nil {
		t.Fatalf("delete clinic: %v", err)
	}

	assertClinicMutationEvents(t, transaction, committedClinicID, map[string]int{
		EventTypeClinicCreated: 1,
		EventTypeClinicUpdated: 1,
		EventTypeClinicDeleted: 1,
	})

	if err := transaction.Commit(t.Context()); err != nil {
		t.Fatalf("commit clinic mutations: %v", err)
	}
	assertClinicMutationEvents(t, pool, committedClinicID, map[string]int{
		EventTypeClinicCreated: 1,
		EventTypeClinicUpdated: 1,
		EventTypeClinicDeleted: 1,
	})

	rolledBackTransaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin rolled-back clinic mutation transaction: %v", err)
	}
	defer rolledBackTransaction.Rollback(t.Context())

	if err := rolledBackTransaction.QueryRow(
		t.Context(),
		`INSERT INTO clinics (name, address, time_zone)
		 VALUES ('Clinic Outbox Rolled Back', 'Test address', 'Europe/Kyiv')
		 RETURNING id::text`,
	).Scan(&rolledBackClinicID); err != nil {
		t.Fatalf("insert rolled-back clinic: %v", err)
	}
	assertClinicMutationEvents(t, rolledBackTransaction, rolledBackClinicID, map[string]int{
		EventTypeClinicCreated: 1,
	})

	if err := rolledBackTransaction.Rollback(t.Context()); err != nil {
		t.Fatalf("roll back clinic mutation: %v", err)
	}

	var clinicCount, eventCount int
	if err := pool.QueryRow(
		t.Context(),
		`SELECT
			(SELECT count(*) FROM clinics WHERE id = $1),
			(SELECT count(*) FROM outbox_events WHERE aggregate_id = $1)`,
		rolledBackClinicID,
	).Scan(&clinicCount, &eventCount); err != nil {
		t.Fatalf("count rolled-back clinic and outbox events: %v", err)
	}
	if clinicCount != 0 || eventCount != 0 {
		t.Fatalf(
			"expected rolled-back clinic and event to be absent, got clinics=%d events=%d",
			clinicCount,
			eventCount,
		)
	}
}

type clinicMutationEventQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func assertClinicMutationEvents(
	t *testing.T,
	querier clinicMutationEventQuerier,
	clinicID string,
	expectedCounts map[string]int,
) {
	t.Helper()

	rows, err := querier.Query(
		t.Context(),
		`SELECT aggregate_type, aggregate_id::text, event_type, payload, processed_at IS NULL
		 FROM outbox_events
		 WHERE aggregate_id = $1`,
		clinicID,
	)
	if err != nil {
		t.Fatalf("query clinic outbox events: %v", err)
	}
	defer rows.Close()

	actualCounts := make(map[string]int, len(expectedCounts))
	for rows.Next() {
		var aggregateType, aggregateID, eventType string
		var payload []byte
		var pending bool
		if err := rows.Scan(&aggregateType, &aggregateID, &eventType, &payload, &pending); err != nil {
			t.Fatalf("scan clinic outbox event: %v", err)
		}
		if aggregateType != AggregateTypeClinic || aggregateID != clinicID {
			t.Fatalf(
				"unexpected clinic event association: aggregate_type=%q aggregate_id=%q",
				aggregateType,
				aggregateID,
			)
		}
		if !pending {
			t.Fatalf("expected clinic event %q to be pending", eventType)
		}

		var decodedPayload struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(payload, &decodedPayload); err != nil {
			t.Fatalf("decode clinic %q event payload: %v", eventType, err)
		}
		if decodedPayload.ID != clinicID {
			t.Fatalf("clinic %q payload ID = %q, want %q", eventType, decodedPayload.ID, clinicID)
		}
		actualCounts[eventType]++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate clinic outbox events: %v", err)
	}

	if len(actualCounts) != len(expectedCounts) {
		t.Fatalf("clinic event counts = %v, want %v", actualCounts, expectedCounts)
	}
	for eventType, expectedCount := range expectedCounts {
		if actualCounts[eventType] != expectedCount {
			t.Fatalf("clinic event counts = %v, want %v", actualCounts, expectedCounts)
		}
	}
}
