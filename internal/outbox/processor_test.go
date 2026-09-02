package outbox

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

func TestProcessBatchDeliversAndMarksEventsInOrder(t *testing.T) {
	t.Parallel()

	firstID := mustTestUUID(t, "11111111-1111-4111-8111-111111111111")
	secondID := mustTestUUID(t, "22222222-2222-4222-8222-222222222222")
	createdAt := time.Date(2026, time.August, 5, 12, 0, 0, 0, time.UTC)
	operations := make([]string, 0, 5)
	transaction := &fakeBatchTransaction{operations: &operations}
	queries := &fakeBatchQueries{
		operations: &operations,
		rows: []sqlc.OutboxEvent{
			{
				ID:            firstID,
				EventSequence: 7,
				AggregateType: "visit",
				AggregateID:   firstID,
				EventType:     "first",
				Payload:       []byte(`{"visit_id":"first"}`),
				CreatedAt:     pgtype.Timestamptz{Time: createdAt, Valid: true},
			},
			{ID: secondID, AggregateID: secondID, EventType: "second"},
		},
	}
	processor := newProcessor(
		func(context.Context) (batchTransaction, batchQueries, error) {
			return transaction, queries, nil
		},
		func(_ context.Context, event PersistedEvent) error {
			if event.EventType == "first" && (event.ID != firstID.String() ||
				event.Sequence != 7 ||
				event.AggregateType != "visit" ||
				event.AggregateID != firstID.String() ||
				string(event.Payload) != `{"visit_id":"first"}` ||
				!event.CreatedAt.Equal(createdAt)) {
				t.Fatalf("unexpected persisted event: %+v", event)
			}
			operations = append(operations, "consume "+event.EventType)
			return nil
		},
	)

	processed, err := processor.ProcessBatch(t.Context(), 2)
	if err != nil {
		t.Fatalf("process batch: %v", err)
	}
	if processed != 2 {
		t.Fatalf("expected 2 processed events, got %d", processed)
	}
	if queries.batchSize != 2 {
		t.Fatalf("expected batch size 2, got %d", queries.batchSize)
	}

	expectedOperations := []string{
		"consume first",
		"mark " + firstID.String(),
		"consume second",
		"mark " + secondID.String(),
		"commit",
	}
	if !reflect.DeepEqual(operations, expectedOperations) {
		t.Fatalf("expected operations %v, got %v", expectedOperations, operations)
	}
}

func TestProcessBatchStopsOnConsumerFailureAndCommitsEarlierSuccesses(t *testing.T) {
	t.Parallel()

	firstID := mustTestUUID(t, "11111111-1111-4111-8111-111111111111")
	secondID := mustTestUUID(t, "22222222-2222-4222-8222-222222222222")
	thirdID := mustTestUUID(t, "33333333-3333-4333-8333-333333333333")
	consumerFailure := errors.New("search unavailable")
	operations := make([]string, 0, 5)
	transaction := &fakeBatchTransaction{operations: &operations}
	queries := &fakeBatchQueries{
		operations: &operations,
		rows: []sqlc.OutboxEvent{
			{ID: firstID, AggregateID: firstID, EventType: "first"},
			{ID: secondID, AggregateID: secondID, EventType: "second"},
			{ID: thirdID, AggregateID: thirdID, EventType: "third"},
		},
	}
	processor := newProcessor(
		func(context.Context) (batchTransaction, batchQueries, error) {
			return transaction, queries, nil
		},
		func(_ context.Context, event PersistedEvent) error {
			operations = append(operations, "consume "+event.EventType)
			if event.EventType == "second" {
				return consumerFailure
			}
			return nil
		},
	)

	processed, err := processor.ProcessBatch(t.Context(), 3)
	if !errors.Is(err, consumerFailure) {
		t.Fatalf("expected wrapped consumer error, got %v", err)
	}
	if processed != 1 {
		t.Fatalf("expected 1 processed event, got %d", processed)
	}

	expectedOperations := []string{
		"consume first",
		"mark " + firstID.String(),
		"consume second",
		"commit",
	}
	if !reflect.DeepEqual(operations, expectedOperations) {
		t.Fatalf("expected operations %v, got %v", expectedOperations, operations)
	}
}

func TestProcessBatchRejectsInvalidInputBeforeStartingTransaction(t *testing.T) {
	t.Parallel()

	beginCalls := 0
	begin := func(context.Context) (batchTransaction, batchQueries, error) {
		beginCalls++
		return nil, nil, nil
	}

	if _, err := newProcessor(begin, func(context.Context, PersistedEvent) error {
		return nil
	}).ProcessBatch(t.Context(), 0); !errors.Is(err, ErrInvalidBatchSize) {
		t.Fatalf("expected invalid batch size error, got %v", err)
	}
	if _, err := newProcessor(begin, nil).ProcessBatch(t.Context(), 1); !errors.Is(err, ErrNilConsumer) {
		t.Fatalf("expected nil consumer error, got %v", err)
	}
	if beginCalls != 0 {
		t.Fatalf("expected no transaction, got %d begin calls", beginCalls)
	}
}

func TestProcessBatchWrapsDatabaseErrors(t *testing.T) {
	t.Parallel()

	databaseFailure := errors.New("database unavailable")
	eventID := mustTestUUID(t, "11111111-1111-4111-8111-111111111111")
	tests := []struct {
		name         string
		begin        beginBatchFunc
		expectedText string
	}{
		{
			name: "begin",
			begin: func(context.Context) (batchTransaction, batchQueries, error) {
				return nil, nil, databaseFailure
			},
			expectedText: "begin outbox processing transaction",
		},
		{
			name: "select",
			begin: func(context.Context) (batchTransaction, batchQueries, error) {
				return &fakeBatchTransaction{}, &fakeBatchQueries{listErr: databaseFailure}, nil
			},
			expectedText: "select pending outbox events",
		},
		{
			name: "mark",
			begin: func(context.Context) (batchTransaction, batchQueries, error) {
				return &fakeBatchTransaction{}, &fakeBatchQueries{
					rows:    []sqlc.OutboxEvent{{ID: eventID, AggregateID: eventID}},
					markErr: databaseFailure,
				}, nil
			},
			expectedText: "mark outbox event",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			processor := newProcessor(test.begin, func(context.Context, PersistedEvent) error {
				return nil
			})
			_, err := processor.ProcessBatch(t.Context(), 1)
			if !errors.Is(err, databaseFailure) {
				t.Fatalf("expected wrapped database error, got %v", err)
			}
			if !strings.Contains(err.Error(), test.expectedText) {
				t.Fatalf("expected error context %q, got %v", test.expectedText, err)
			}
		})
	}
}

type fakeBatchTransaction struct {
	operations *[]string
	commitErr  error
}

func (t *fakeBatchTransaction) Commit(context.Context) error {
	if t.operations != nil {
		*t.operations = append(*t.operations, "commit")
	}
	return t.commitErr
}

func (*fakeBatchTransaction) Rollback(context.Context) error {
	return nil
}

type fakeBatchQueries struct {
	operations *[]string
	rows       []sqlc.OutboxEvent
	listErr    error
	markErr    error
	batchSize  int32
}

func (q *fakeBatchQueries) ListPendingOutboxEventsForUpdate(
	_ context.Context,
	batchSize int32,
) ([]sqlc.OutboxEvent, error) {
	q.batchSize = batchSize
	return q.rows, q.listErr
}

func (q *fakeBatchQueries) MarkOutboxEventProcessed(
	_ context.Context,
	id pgtype.UUID,
) (int64, error) {
	if q.operations != nil {
		*q.operations = append(*q.operations, "mark "+id.String())
	}
	if q.markErr != nil {
		return 0, q.markErr
	}
	return 1, nil
}

func mustTestUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()

	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		t.Fatalf("parse test UUID: %v", err)
	}
	return id
}
