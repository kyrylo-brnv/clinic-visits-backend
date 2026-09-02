//go:build integration

package outbox

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/jackc/pgx/v5"
)

func TestDoctorMutationsCreateAtomicOutboxEvents(t *testing.T) {
	pool := openOutboxTestPool(t)

	var specialtyID, clinicID string
	if err := pool.QueryRow(
		t.Context(),
		"INSERT INTO specialties (name) VALUES ('doctor-outbox-' || gen_random_uuid()::text) RETURNING id::text",
	).Scan(&specialtyID); err != nil {
		t.Fatalf("insert specialty fixture: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM specialties WHERE id = $1", specialtyID); err != nil {
			t.Errorf("delete specialty fixture: %v", err)
		}
	})
	if err := pool.QueryRow(
		t.Context(),
		`INSERT INTO clinics (name, address, time_zone)
		 VALUES ('Doctor outbox clinic', 'Test address', 'Europe/Kyiv')
		 RETURNING id::text`,
	).Scan(&clinicID); err != nil {
		t.Fatalf("insert clinic fixture: %v", err)
	}
	t.Cleanup(func() {
		if _, err := pool.Exec(context.Background(), "DELETE FROM clinics WHERE id = $1", clinicID); err != nil {
			t.Errorf("delete clinic fixture: %v", err)
		}
		if _, err := pool.Exec(context.Background(), "DELETE FROM outbox_events WHERE aggregate_id = $1", clinicID); err != nil {
			t.Errorf("delete clinic fixture outbox events: %v", err)
		}
	})

	var committedDoctorID, rolledBackDoctorID string
	t.Cleanup(func() {
		ctx := context.Background()
		if committedDoctorID != "" {
			if _, err := pool.Exec(ctx, "DELETE FROM outbox_events WHERE aggregate_id = $1", committedDoctorID); err != nil {
				t.Errorf("delete committed doctor outbox events: %v", err)
			}
		}
		if rolledBackDoctorID != "" {
			if _, err := pool.Exec(ctx, "DELETE FROM outbox_events WHERE aggregate_id = $1", rolledBackDoctorID); err != nil {
				t.Errorf("delete rolled-back doctor outbox events: %v", err)
			}
		}
	})

	transaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin committed doctor mutation transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())

	if err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO doctors (specialty_id, clinic_id, full_name)
		 VALUES ($1, $2, 'Doctor Outbox Created')
		 RETURNING id::text`,
		specialtyID,
		clinicID,
	).Scan(&committedDoctorID); err != nil {
		t.Fatalf("insert doctor: %v", err)
	}
	if _, err := transaction.Exec(
		t.Context(),
		"UPDATE doctors SET full_name = 'Doctor Outbox Updated' WHERE id = $1",
		committedDoctorID,
	); err != nil {
		t.Fatalf("update doctor: %v", err)
	}
	if _, err := transaction.Exec(t.Context(), "DELETE FROM doctors WHERE id = $1", committedDoctorID); err != nil {
		t.Fatalf("delete doctor: %v", err)
	}

	assertDoctorMutationEvents(t, transaction, committedDoctorID, map[string]int{
		EventTypeDoctorCreated: 1,
		EventTypeDoctorUpdated: 1,
		EventTypeDoctorDeleted: 1,
	})

	if err := transaction.Commit(t.Context()); err != nil {
		t.Fatalf("commit doctor mutations: %v", err)
	}
	assertDoctorMutationEvents(t, pool, committedDoctorID, map[string]int{
		EventTypeDoctorCreated: 1,
		EventTypeDoctorUpdated: 1,
		EventTypeDoctorDeleted: 1,
	})

	rolledBackTransaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin rolled-back doctor mutation transaction: %v", err)
	}
	defer rolledBackTransaction.Rollback(t.Context())

	if err := rolledBackTransaction.QueryRow(
		t.Context(),
		`INSERT INTO doctors (specialty_id, clinic_id, full_name)
		 VALUES ($1, $2, 'Doctor Outbox Rolled Back')
		 RETURNING id::text`,
		specialtyID,
		clinicID,
	).Scan(&rolledBackDoctorID); err != nil {
		t.Fatalf("insert rolled-back doctor: %v", err)
	}
	assertDoctorMutationEvents(t, rolledBackTransaction, rolledBackDoctorID, map[string]int{
		EventTypeDoctorCreated: 1,
	})

	if err := rolledBackTransaction.Rollback(t.Context()); err != nil {
		t.Fatalf("roll back doctor mutation: %v", err)
	}

	var doctorCount, eventCount int
	if err := pool.QueryRow(
		t.Context(),
		`SELECT
			(SELECT count(*) FROM doctors WHERE id = $1),
			(SELECT count(*) FROM outbox_events WHERE aggregate_id = $1)`,
		rolledBackDoctorID,
	).Scan(&doctorCount, &eventCount); err != nil {
		t.Fatalf("count rolled-back doctor and outbox events: %v", err)
	}
	if doctorCount != 0 || eventCount != 0 {
		t.Fatalf(
			"expected rolled-back doctor and event to be absent, got doctors=%d events=%d",
			doctorCount,
			eventCount,
		)
	}
}

type doctorMutationEventQuerier interface {
	Query(context.Context, string, ...any) (pgx.Rows, error)
}

func assertDoctorMutationEvents(
	t *testing.T,
	querier doctorMutationEventQuerier,
	doctorID string,
	expectedCounts map[string]int,
) {
	t.Helper()

	rows, err := querier.Query(
		t.Context(),
		`SELECT aggregate_type, aggregate_id::text, event_type, payload, processed_at IS NULL
		 FROM outbox_events
		 WHERE aggregate_id = $1`,
		doctorID,
	)
	if err != nil {
		t.Fatalf("query doctor outbox events: %v", err)
	}
	defer rows.Close()

	actualCounts := make(map[string]int, len(expectedCounts))
	for rows.Next() {
		var aggregateType, aggregateID, eventType string
		var payload []byte
		var pending bool
		if err := rows.Scan(&aggregateType, &aggregateID, &eventType, &payload, &pending); err != nil {
			t.Fatalf("scan doctor outbox event: %v", err)
		}
		if aggregateType != AggregateTypeDoctor || aggregateID != doctorID {
			t.Fatalf(
				"unexpected doctor event association: aggregate_type=%q aggregate_id=%q",
				aggregateType,
				aggregateID,
			)
		}
		if !pending {
			t.Fatalf("expected doctor event %q to be pending", eventType)
		}

		var decodedPayload struct {
			ID string `json:"id"`
		}
		if err := json.Unmarshal(payload, &decodedPayload); err != nil {
			t.Fatalf("decode doctor %q event payload: %v", eventType, err)
		}
		if decodedPayload.ID != doctorID {
			t.Fatalf("doctor %q payload ID = %q, want %q", eventType, decodedPayload.ID, doctorID)
		}
		actualCounts[eventType]++
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate doctor outbox events: %v", err)
	}

	if len(actualCounts) != len(expectedCounts) {
		t.Fatalf("doctor event counts = %v, want %v", actualCounts, expectedCounts)
	}
	for eventType, expectedCount := range expectedCounts {
		if actualCounts[eventType] != expectedCount {
			t.Fatalf("doctor event counts = %v, want %v", actualCounts, expectedCounts)
		}
	}
}
