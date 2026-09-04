//go:build integration

package outbox

import (
	"context"
	"errors"
	"os"
	"reflect"
	"sort"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProcessorIntegrationLimitsAndMarksSuccessfulEvents(t *testing.T) {
	pool := openOutboxTestPool(t)
	baseTime := time.Date(1000, time.January, 1, 0, 0, 0, 0, time.UTC)
	baseSequence := outboxIntegrationSequenceBase(t, pool)

	thirdID := insertOutboxTestEvent(t, pool, "third", baseTime.Add(2*time.Second), baseSequence+2)
	firstID := insertOutboxTestEvent(t, pool, "first", baseTime, baseSequence)
	secondID := insertOutboxTestEvent(t, pool, "second", baseTime.Add(time.Second), baseSequence+1)
	cleanupOutboxTestEvents(t, pool, firstID, secondID, thirdID)

	delivered := make([]string, 0, 2)
	var deliveredMu sync.Mutex
	processor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
		deliveredMu.Lock()
		defer deliveredMu.Unlock()
		delivered = append(delivered, event.ID)
		return nil
	})

	processed, err := processor.ProcessBatch(t.Context(), 2)
	if err != nil {
		t.Fatalf("process outbox batch: %v", err)
	}
	if processed != 2 {
		t.Fatalf("expected 2 processed events, got %d", processed)
	}
	sort.Strings(delivered)
	expected := []string{firstID.String(), secondID.String()}
	sort.Strings(expected)
	if !reflect.DeepEqual(delivered, expected) {
		t.Fatalf("expected deliveries %v, got %v", expected, delivered)
	}

	assertOutboxProcessed(t, pool, firstID, true)
	assertOutboxProcessed(t, pool, secondID, true)
	assertOutboxProcessed(t, pool, thirdID, false)
}

func TestProcessorIntegrationRetriesEventAfterConsumerFailure(t *testing.T) {
	pool := openOutboxTestPool(t)
	eventID := insertOutboxTestEvent(
		t,
		pool,
		"retry",
		time.Date(1001, time.January, 1, 0, 0, 0, 0, time.UTC),
		outboxIntegrationSequenceBase(t, pool),
	)
	cleanupOutboxTestEvents(t, pool, eventID)

	consumerFailure := errors.New("consumer unavailable")
	failedProcessor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
		if event.ID != eventID.String() {
			t.Fatalf("expected event %s, got %s", eventID.String(), event.ID)
		}
		return consumerFailure
	})

	processed, err := failedProcessor.ProcessBatch(t.Context(), 1)
	if !errors.Is(err, consumerFailure) {
		t.Fatalf("expected consumer error, got %v", err)
	}
	if processed != 0 {
		t.Fatalf("expected no processed events, got %d", processed)
	}
	assertOutboxProcessed(t, pool, eventID, false)

	retryDeliveries := 0
	retryProcessor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
		if event.ID != eventID.String() {
			t.Fatalf("expected retry event %s, got %s", eventID.String(), event.ID)
		}
		retryDeliveries++
		return nil
	})

	processed, err = retryProcessor.ProcessBatch(t.Context(), 1)
	if err != nil {
		t.Fatalf("retry outbox event: %v", err)
	}
	if processed != 1 || retryDeliveries != 1 {
		t.Fatalf("expected one successful retry, got processed=%d deliveries=%d", processed, retryDeliveries)
	}
	assertOutboxProcessed(t, pool, eventID, true)
}

func TestProcessorIntegrationDoesNotOvertakeEarlierPendingEventForAggregate(t *testing.T) {
	pool := openOutboxTestPool(t)
	var aggregateID pgtype.UUID
	if err := pool.QueryRow(t.Context(), "SELECT gen_random_uuid()").Scan(&aggregateID); err != nil {
		t.Fatalf("create aggregate ID: %v", err)
	}

	baseTime := time.Date(1002, time.January, 1, 0, 0, 0, 0, time.UTC)
	baseSequence := outboxIntegrationSequenceBase(t, pool)
	firstID := insertOutboxTestEventForAggregate(t, pool, aggregateID, "first", baseTime.Add(time.Hour), baseSequence)
	secondID := insertOutboxTestEventForAggregate(t, pool, aggregateID, "second", baseTime, baseSequence+1)
	cleanupOutboxTestEvents(t, pool, firstID, secondID)

	failure := errors.New("first delivery failed")
	delivered := make([]string, 0, 2)
	processor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
		delivered = append(delivered, event.ID)
		return failure
	})

	processed, err := processor.ProcessBatch(t.Context(), 2)
	if !errors.Is(err, failure) {
		t.Fatalf("expected first delivery failure, got %v", err)
	}
	if processed != 0 || !reflect.DeepEqual(delivered, []string{firstID.String()}) {
		t.Fatalf("expected only first event delivery, got processed=%d delivered=%v", processed, delivered)
	}
	assertOutboxProcessed(t, pool, firstID, false)
	assertOutboxProcessed(t, pool, secondID, false)

	delivered = delivered[:0]
	retryProcessor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
		delivered = append(delivered, event.ID)
		return nil
	})
	processed, err = retryProcessor.ProcessBatch(t.Context(), 1)
	if err != nil {
		t.Fatalf("retry first event: %v", err)
	}
	if processed != 1 || !reflect.DeepEqual(delivered, []string{firstID.String()}) {
		t.Fatalf("expected only first event retry, got processed=%d delivered=%v", processed, delivered)
	}
	assertOutboxProcessed(t, pool, firstID, true)
	assertOutboxProcessed(t, pool, secondID, false)

	processed, err = retryProcessor.ProcessBatch(t.Context(), 1)
	if err != nil {
		t.Fatalf("process second event: %v", err)
	}
	if processed != 1 || !reflect.DeepEqual(delivered, []string{firstID.String(), secondID.String()}) {
		t.Fatalf("expected ordered deliveries, got processed=%d delivered=%v", processed, delivered)
	}
	assertOutboxProcessed(t, pool, secondID, true)
}

func TestProcessorIntegrationConcurrentProcessorsSkipLockedEvents(t *testing.T) {
	pool := openOutboxTestPool(t)
	baseTime := time.Date(1002, time.January, 1, 0, 0, 0, 0, time.UTC)
	baseSequence := outboxIntegrationSequenceBase(t, pool)
	firstID := insertOutboxTestEvent(t, pool, "locked-first", baseTime, baseSequence)
	secondID := insertOutboxTestEvent(t, pool, "skipped-to-second", baseTime.Add(time.Second), baseSequence+1)
	cleanupOutboxTestEvents(t, pool, firstID, secondID)

	firstConsumerStarted := make(chan struct{})
	releaseFirstConsumer := make(chan struct{})
	firstResult := make(chan error, 1)
	firstProcessor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
		if event.ID != firstID.String() {
			return errors.New("first processor received an unexpected event")
		}
		close(firstConsumerStarted)
		<-releaseFirstConsumer
		return nil
	})
	go func() {
		processed, err := firstProcessor.ProcessBatch(t.Context(), 1)
		if err == nil && processed != 1 {
			err = errors.New("first processor did not process exactly one event")
		}
		firstResult <- err
	}()

	select {
	case <-firstConsumerStarted:
	case <-time.After(5 * time.Second):
		close(releaseFirstConsumer)
		t.Fatal("timed out waiting for first processor to lock its event")
	}

	secondDeliveredID := ""
	secondProcessor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
		secondDeliveredID = event.ID
		return nil
	})
	processed, err := secondProcessor.ProcessBatch(t.Context(), 1)
	if err != nil {
		close(releaseFirstConsumer)
		t.Fatalf("process second concurrent batch: %v", err)
	}
	if processed != 1 || secondDeliveredID != secondID.String() {
		close(releaseFirstConsumer)
		t.Fatalf(
			"expected second processor to skip %s and process %s, got processed=%d event=%s",
			firstID.String(),
			secondID.String(),
			processed,
			secondDeliveredID,
		)
	}

	close(releaseFirstConsumer)
	select {
	case err := <-firstResult:
		if err != nil {
			t.Fatalf("process first concurrent batch: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first processor to finish")
	}

	assertOutboxProcessed(t, pool, firstID, true)
	assertOutboxProcessed(t, pool, secondID, true)
}

func TestProcessorIntegrationConcurrentProcessorCannotOvertakeLockedAggregateEvent(t *testing.T) {
	pool := openOutboxTestPool(t)
	var aggregateID pgtype.UUID
	if err := pool.QueryRow(t.Context(), "SELECT gen_random_uuid()").Scan(&aggregateID); err != nil {
		t.Fatalf("create aggregate ID: %v", err)
	}
	baseSequence := outboxIntegrationSequenceBase(t, pool)
	baseTime := time.Date(1003, time.January, 1, 0, 0, 0, 0, time.UTC)
	firstID := insertOutboxTestEventForAggregate(t, pool, aggregateID, "locked-first", baseTime, baseSequence)
	secondID := insertOutboxTestEventForAggregate(t, pool, aggregateID, "blocked-second", baseTime.Add(time.Second), baseSequence+1)
	cleanupOutboxTestEvents(t, pool, firstID, secondID)

	firstConsumerStarted := make(chan struct{})
	releaseFirstConsumer := make(chan struct{})
	firstResult := make(chan error, 1)
	firstProcessor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
		if event.ID != firstID.String() {
			return errors.New("first processor received an unexpected event")
		}
		close(firstConsumerStarted)
		<-releaseFirstConsumer
		return nil
	})
	go func() {
		processed, err := firstProcessor.ProcessBatch(t.Context(), 1)
		if err == nil && processed != 1 {
			err = errors.New("first processor did not process exactly one event")
		}
		firstResult <- err
	}()

	select {
	case <-firstConsumerStarted:
	case <-time.After(5 * time.Second):
		close(releaseFirstConsumer)
		t.Fatal("timed out waiting for first processor to lock the aggregate event")
	}

	secondDeliveredID := ""
	secondProcessor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
		secondDeliveredID = event.ID
		return nil
	})
	processed, err := secondProcessor.ProcessBatch(t.Context(), 1)
	if err != nil {
		close(releaseFirstConsumer)
		t.Fatalf("process concurrent same-aggregate batch: %v", err)
	}
	if processed != 0 || secondDeliveredID != "" {
		close(releaseFirstConsumer)
		t.Fatalf("same-aggregate concurrent batch processed=%d delivered=%s, later=%s, want no delivery", processed, secondDeliveredID, secondID.String())
	}
	assertOutboxProcessed(t, pool, firstID, false)
	assertOutboxProcessed(t, pool, secondID, false)

	close(releaseFirstConsumer)
	select {
	case err := <-firstResult:
		if err != nil {
			t.Fatalf("process first aggregate event: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for first aggregate event to finish")
	}
	assertOutboxProcessed(t, pool, firstID, true)
	assertOutboxProcessed(t, pool, secondID, false)

	processed, err = secondProcessor.ProcessBatch(t.Context(), 1)
	if err != nil {
		t.Fatalf("process unblocked aggregate event: %v", err)
	}
	if processed != 1 || secondDeliveredID != secondID.String() {
		t.Fatalf("unblocked aggregate batch processed=%d delivered=%s, want %s", processed, secondDeliveredID, secondID.String())
	}
	assertOutboxProcessed(t, pool, secondID, true)
}

func TestOutboxOrderingMigrationBackfillsExistingEvents(t *testing.T) {
	pool := openOutboxTestPool(t)
	downMigration, err := os.ReadFile("../../migrations/000018_order_outbox_events_by_aggregate.down.sql")
	if err != nil {
		t.Fatalf("read ordering down migration: %v", err)
	}
	upMigration, err := os.ReadFile("../../migrations/000018_order_outbox_events_by_aggregate.up.sql")
	if err != nil {
		t.Fatalf("read ordering up migration: %v", err)
	}

	transaction, err := pool.Begin(t.Context())
	if err != nil {
		t.Fatalf("begin migration verification transaction: %v", err)
	}
	defer transaction.Rollback(t.Context())
	if _, err := transaction.Exec(t.Context(), string(downMigration)); err != nil {
		t.Fatalf("apply ordering down migration: %v", err)
	}

	var aggregateID string
	if err := transaction.QueryRow(t.Context(), "SELECT gen_random_uuid()::text").Scan(&aggregateID); err != nil {
		t.Fatalf("create migration aggregate ID: %v", err)
	}
	baseTime := time.Date(2026, time.August, 31, 12, 0, 0, 0, time.UTC)
	for _, event := range []struct {
		id        string
		eventType string
		createdAt time.Time
	}{
		{id: "33333333-3333-4333-8333-333333333333", eventType: "third", createdAt: baseTime.Add(time.Second)},
		{id: "22222222-2222-4222-8222-222222222222", eventType: "second", createdAt: baseTime.Add(time.Second)},
		{id: "11111111-1111-4111-8111-111111111111", eventType: "first", createdAt: baseTime},
	} {
		if _, err := transaction.Exec(
			t.Context(),
			`INSERT INTO outbox_events (id, aggregate_type, aggregate_id, event_type, payload, created_at)
			 VALUES ($1, 'outbox.migration.integration', $2, $3, '{}'::jsonb, $4)`,
			event.id,
			aggregateID,
			event.eventType,
			event.createdAt,
		); err != nil {
			t.Fatalf("insert historical %s event: %v", event.eventType, err)
		}
	}
	if _, err := transaction.Exec(t.Context(), string(upMigration)); err != nil {
		t.Fatalf("apply ordering up migration: %v", err)
	}

	rows, err := transaction.Query(
		t.Context(),
		`SELECT event_type
		 FROM outbox_events
		 WHERE aggregate_id = $1
		 ORDER BY event_sequence`,
		aggregateID,
	)
	if err != nil {
		t.Fatalf("query backfilled event order: %v", err)
	}
	var eventTypes []string
	for rows.Next() {
		var eventType string
		if err := rows.Scan(&eventType); err != nil {
			rows.Close()
			t.Fatalf("scan backfilled event type: %v", err)
		}
		eventTypes = append(eventTypes, eventType)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		t.Fatalf("iterate backfilled event types: %v", err)
	}
	rows.Close()
	if expected := []string{"first", "second", "third"}; !reflect.DeepEqual(eventTypes, expected) {
		t.Fatalf("backfilled event order = %v, want %v", eventTypes, expected)
	}

	var previousMaximum, nextSequence int64
	if err := transaction.QueryRow(t.Context(), "SELECT MAX(event_sequence) FROM outbox_events").Scan(&previousMaximum); err != nil {
		t.Fatalf("load backfilled maximum sequence: %v", err)
	}
	if err := transaction.QueryRow(
		t.Context(),
		`INSERT INTO outbox_events (aggregate_type, aggregate_id, event_type, payload)
		 VALUES ('outbox.migration.integration', $1, 'future', '{}'::jsonb)
		 RETURNING event_sequence`,
		aggregateID,
	).Scan(&nextSequence); err != nil {
		t.Fatalf("insert future sequenced event: %v", err)
	}
	if nextSequence != previousMaximum+1 {
		t.Fatalf("future event sequence = %d, want %d", nextSequence, previousMaximum+1)
	}
}

func openOutboxTestPool(t *testing.T) *pgxpool.Pool {
	t.Helper()

	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	pool, err := pgxpool.New(t.Context(), databaseURL)
	if err != nil {
		t.Fatalf("connect to PostgreSQL: %v", err)
	}
	if err := pool.Ping(t.Context()); err != nil {
		pool.Close()
		t.Fatalf("ping PostgreSQL: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func insertOutboxTestEvent(
	t *testing.T,
	pool *pgxpool.Pool,
	eventType string,
	createdAt time.Time,
	eventSequence int64,
) pgtype.UUID {
	t.Helper()
	var aggregateID pgtype.UUID
	if err := pool.QueryRow(t.Context(), "SELECT gen_random_uuid()").Scan(&aggregateID); err != nil {
		t.Fatalf("create outbox aggregate ID: %v", err)
	}
	return insertOutboxTestEventForAggregate(t, pool, aggregateID, eventType, createdAt, eventSequence)
}

func insertOutboxTestEventForAggregate(
	t *testing.T,
	pool *pgxpool.Pool,
	aggregateID pgtype.UUID,
	eventType string,
	createdAt time.Time,
	eventSequence int64,
) pgtype.UUID {
	t.Helper()

	var id pgtype.UUID
	if err := pool.QueryRow(
		t.Context(),
		`INSERT INTO outbox_events (
			aggregate_type,
			aggregate_id,
			event_type,
			payload,
			created_at,
			event_sequence
		 )
		 OVERRIDING SYSTEM VALUE
		 VALUES ('outbox.processor.integration', $1, $2, '{}'::jsonb, $3, $4)
		 RETURNING id`,
		aggregateID,
		eventType,
		createdAt,
		eventSequence,
	).Scan(&id); err != nil {
		t.Fatalf("insert outbox event fixture: %v", err)
	}

	return id
}

func cleanupOutboxTestEvents(t *testing.T, pool *pgxpool.Pool, eventIDs ...pgtype.UUID) {
	t.Helper()

	t.Cleanup(func() {
		for _, eventID := range eventIDs {
			if _, err := pool.Exec(context.Background(), "DELETE FROM outbox_events WHERE id = $1", eventID); err != nil {
				t.Errorf("delete outbox event fixture %s: %v", eventID.String(), err)
			}
		}
	})
}

func outboxIntegrationSequenceBase(t *testing.T, pool *pgxpool.Pool) int64 {
	t.Helper()
	var sequence int64
	if err := pool.QueryRow(
		t.Context(),
		"SELECT COALESCE(MIN(event_sequence), 0) - 1000 FROM outbox_events",
	).Scan(&sequence); err != nil {
		t.Fatalf("choose outbox integration event sequence: %v", err)
	}
	return sequence
}

func assertOutboxProcessed(
	t *testing.T,
	pool *pgxpool.Pool,
	eventID pgtype.UUID,
	expected bool,
) {
	t.Helper()

	var processed bool
	if err := pool.QueryRow(
		t.Context(),
		"SELECT processed_at IS NOT NULL FROM outbox_events WHERE id = $1",
		eventID,
	).Scan(&processed); err != nil {
		t.Fatalf("read outbox event processed state: %v", err)
	}
	if processed != expected {
		t.Fatalf("expected event %s processed=%t, got %t", eventID.String(), expected, processed)
	}
}
