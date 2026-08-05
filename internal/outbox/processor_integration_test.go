//go:build integration

package outbox

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

func TestProcessorIntegrationOrdersLimitsAndMarksSuccessfulEvents(t *testing.T) {
	pool := openOutboxTestPool(t)
	baseTime := time.Date(1000, time.January, 1, 0, 0, 0, 0, time.UTC)

	thirdID := insertOutboxTestEvent(t, pool, "third", baseTime.Add(2*time.Second))
	firstID := insertOutboxTestEvent(t, pool, "first", baseTime)
	secondID := insertOutboxTestEvent(t, pool, "second", baseTime.Add(time.Second))
	cleanupOutboxTestEvents(t, pool, firstID, secondID, thirdID)

	delivered := make([]string, 0, 2)
	processor := NewProcessor(pool, func(_ context.Context, event PersistedEvent) error {
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
	if expected := []string{firstID.String(), secondID.String()}; !reflect.DeepEqual(delivered, expected) {
		t.Fatalf("expected delivery order %v, got %v", expected, delivered)
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

func TestProcessorIntegrationConcurrentProcessorsSkipLockedEvents(t *testing.T) {
	pool := openOutboxTestPool(t)
	baseTime := time.Date(1002, time.January, 1, 0, 0, 0, 0, time.UTC)
	firstID := insertOutboxTestEvent(t, pool, "locked-first", baseTime)
	secondID := insertOutboxTestEvent(t, pool, "skipped-to-second", baseTime.Add(time.Second))
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
			created_at
		 )
		 VALUES ('outbox.processor.integration', gen_random_uuid(), $1, '{}'::jsonb, $2)
		 RETURNING id`,
		eventType,
		createdAt,
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
