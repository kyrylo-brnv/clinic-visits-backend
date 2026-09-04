package outbox

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"sync/atomic"
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
	operations := &operationRecorder{}
	consumerValidation := make(chan error, 2)
	transaction := &fakeBatchTransaction{operations: operations}
	queries := &fakeBatchQueries{
		operations: operations,
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
				consumerValidation <- fmt.Errorf("unexpected persisted event: %+v", event)
			} else {
				consumerValidation <- nil
			}
			operations.add("consume " + event.EventType)
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
	for range 2 {
		if err := <-consumerValidation; err != nil {
			t.Fatal(err)
		}
	}
	if queries.batchSize != 2 {
		t.Fatalf("expected batch size 2, got %d", queries.batchSize)
	}

	gotOperations := operations.snapshot()
	if len(gotOperations) != 5 {
		t.Fatalf("operations = %v, want 5 operations", gotOperations)
	}
	if got := map[string]bool{gotOperations[0]: true, gotOperations[1]: true}; !got["consume first"] || !got["consume second"] {
		t.Fatalf("concurrent consume operations = %v, want first and second", gotOperations[:2])
	}
	expectedTail := []string{
		"mark " + firstID.String(),
		"mark " + secondID.String(),
		"commit",
	}
	if !reflect.DeepEqual(gotOperations[2:], expectedTail) {
		t.Fatalf("acknowledgement operations = %v, want %v", gotOperations[2:], expectedTail)
	}
}

func TestProcessBatchStopsOnConsumerFailureAndCommitsEarlierSuccesses(t *testing.T) {
	t.Parallel()

	firstID := mustTestUUID(t, "11111111-1111-4111-8111-111111111111")
	secondID := mustTestUUID(t, "22222222-2222-4222-8222-222222222222")
	thirdID := mustTestUUID(t, "33333333-3333-4333-8333-333333333333")
	consumerFailure := errors.New("search unavailable")
	operations := &operationRecorder{}
	transaction := &fakeBatchTransaction{operations: operations}
	queries := &fakeBatchQueries{
		operations: operations,
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
			operations.add("consume " + event.EventType)
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
	if processed != 2 {
		t.Fatalf("expected 2 independently successful events, got %d", processed)
	}

	gotOperations := operations.snapshot()
	for _, eventType := range []string{"first", "second", "third"} {
		if countOperation(gotOperations, "consume "+eventType) != 1 {
			t.Fatalf("operations = %v, want one consume of %s", gotOperations, eventType)
		}
	}
	if countOperation(gotOperations, "mark "+secondID.String()) != 0 {
		t.Fatalf("failed event was marked processed: %v", gotOperations)
	}
	wantAcknowledgements := []string{"mark " + firstID.String(), "mark " + thirdID.String(), "commit"}
	if !reflect.DeepEqual(gotOperations[len(gotOperations)-3:], wantAcknowledgements) {
		t.Fatalf("acknowledgements = %v, want %v", gotOperations[len(gotOperations)-3:], wantAcknowledgements)
	}
}

func TestProcessBatchUsesExactlyFourConcurrentWorkers(t *testing.T) {
	t.Parallel()

	rows := make([]sqlc.OutboxEvent, outboxWorkerCount)
	usedShards := make(map[int]struct{}, outboxWorkerCount)
	for candidate := 1; len(usedShards) < outboxWorkerCount; candidate++ {
		id := mustTestUUID(t, fmt.Sprintf("00000000-0000-4000-8000-%012d", candidate))
		shard := outboxShard(id.String())
		if _, exists := usedShards[shard]; exists {
			continue
		}
		usedShards[shard] = struct{}{}
		rows[shard] = sqlc.OutboxEvent{ID: id, AggregateID: id, EventSequence: int64(shard + 1)}
	}

	var active atomic.Int32
	var maximum atomic.Int32
	started := make(chan struct{}, outboxWorkerCount)
	release := make(chan struct{})
	processor := newProcessor(
		func(context.Context) (batchTransaction, batchQueries, error) {
			return &fakeBatchTransaction{}, &fakeBatchQueries{rows: rows}, nil
		},
		func(context.Context, PersistedEvent) error {
			current := active.Add(1)
			for {
				observed := maximum.Load()
				if current <= observed || maximum.CompareAndSwap(observed, current) {
					break
				}
			}
			started <- struct{}{}
			<-release
			active.Add(-1)
			return nil
		},
	)

	type batchResult struct {
		processed int
		err       error
	}
	done := make(chan batchResult, 1)
	go func() {
		processed, err := processor.ProcessBatch(context.Background(), int32(len(rows)))
		done <- batchResult{processed: processed, err: err}
	}()
	for range outboxWorkerCount {
		select {
		case <-started:
		case <-time.After(time.Second):
			close(release)
			t.Fatal("four outbox workers did not start concurrently")
		}
	}
	if got := maximum.Load(); got != outboxWorkerCount {
		close(release)
		t.Fatalf("maximum concurrent consumers = %d, want %d", got, outboxWorkerCount)
	}
	close(release)
	result := <-done
	if result.err != nil || result.processed != outboxWorkerCount {
		t.Fatalf("ProcessBatch() processed=%d error=%v", result.processed, result.err)
	}
}

func TestProcessBatchPreservesOrderWithinShard(t *testing.T) {
	t.Parallel()

	aggregateID := mustTestUUID(t, "11111111-1111-4111-8111-111111111111")
	rows := []sqlc.OutboxEvent{
		{ID: mustTestUUID(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa1"), AggregateID: aggregateID, EventSequence: 1},
		{ID: mustTestUUID(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa2"), AggregateID: aggregateID, EventSequence: 2},
		{ID: mustTestUUID(t, "aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaa3"), AggregateID: aggregateID, EventSequence: 3},
	}
	firstStarted := make(chan struct{})
	release := make(chan struct{})
	var sequencesMu sync.Mutex
	var sequences []int64
	processor := newProcessor(
		func(context.Context) (batchTransaction, batchQueries, error) {
			return &fakeBatchTransaction{}, &fakeBatchQueries{rows: rows}, nil
		},
		func(_ context.Context, event PersistedEvent) error {
			if event.AggregateID == aggregateID.String() {
				sequencesMu.Lock()
				sequences = append(sequences, event.Sequence)
				sequencesMu.Unlock()
				if event.Sequence == 1 {
					close(firstStarted)
					<-release
				}
			}
			return nil
		},
	)

	done := make(chan error, 1)
	go func() {
		_, err := processor.ProcessBatch(context.Background(), int32(len(rows)))
		done <- err
	}()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("first aggregate event did not start")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}
	sequencesMu.Lock()
	defer sequencesMu.Unlock()
	if want := []int64{1, 2, 3}; !reflect.DeepEqual(sequences, want) {
		t.Fatalf("same-aggregate sequences = %v, want %v", sequences, want)
	}
}

func TestProcessBatchFullCollidingShardDoesNotBlockIdleShard(t *testing.T) {
	t.Parallel()

	collidingIDs, idleID := mustTestShardCollision(t, 3)
	rows := []sqlc.OutboxEvent{
		{ID: collidingIDs[0], AggregateID: collidingIDs[0], EventSequence: 1},
		{ID: collidingIDs[1], AggregateID: collidingIDs[1], EventSequence: 2},
		{ID: collidingIDs[2], AggregateID: collidingIDs[2], EventSequence: 3},
		{ID: idleID, AggregateID: idleID, EventSequence: 4},
	}
	firstStarted := make(chan struct{})
	idleStarted := make(chan struct{}, 1)
	release := make(chan struct{})
	var collidingMu sync.Mutex
	var collidingSequences []int64
	processor := newProcessor(
		func(context.Context) (batchTransaction, batchQueries, error) {
			return &fakeBatchTransaction{}, &fakeBatchQueries{rows: rows}, nil
		},
		func(_ context.Context, event PersistedEvent) error {
			if outboxShard(event.AggregateID) == outboxShard(collidingIDs[0].String()) {
				collidingMu.Lock()
				collidingSequences = append(collidingSequences, event.Sequence)
				collidingMu.Unlock()
				if event.ID == collidingIDs[0].String() {
					close(firstStarted)
					<-release
				}
			} else {
				idleStarted <- struct{}{}
			}
			return nil
		},
	)

	done := make(chan error, 1)
	go func() {
		_, err := processor.ProcessBatch(context.Background(), int32(len(rows)))
		done <- err
	}()
	select {
	case <-firstStarted:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("first colliding aggregate did not start")
	}
	select {
	case <-idleStarted:
	case <-time.After(time.Second):
		close(release)
		t.Fatal("idle shard was blocked behind colliding aggregate IDs")
	}
	close(release)
	if err := <-done; err != nil {
		t.Fatalf("ProcessBatch() error = %v", err)
	}
	collidingMu.Lock()
	defer collidingMu.Unlock()
	if want := []int64{1, 2, 3}; !reflect.DeepEqual(collidingSequences, want) {
		t.Fatalf("colliding shard sequences = %v, want %v", collidingSequences, want)
	}
}

func TestProcessBatchAcknowledgesCompletedWorkDuringCancellation(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(context.Background())
	eventID := mustTestUUID(t, "11111111-1111-4111-8111-111111111111")
	transaction := &fakeBatchTransaction{}
	queries := &fakeBatchQueries{rows: []sqlc.OutboxEvent{{ID: eventID, AggregateID: eventID}}}
	processor := newProcessor(
		func(context.Context) (batchTransaction, batchQueries, error) { return transaction, queries, nil },
		func(context.Context, PersistedEvent) error {
			cancel()
			return nil
		},
	)

	processed, err := processor.ProcessBatch(ctx, 1)
	if processed != 1 || !errors.Is(err, context.Canceled) {
		t.Fatalf("ProcessBatch() processed=%d error=%v, want 1 and cancellation", processed, err)
	}
	if queries.markContextErr != nil {
		t.Fatalf("mark context error = %v, want draining context", queries.markContextErr)
	}
	if transaction.commitContextErr != nil {
		t.Fatalf("commit context error = %v, want draining context", transaction.commitContextErr)
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
	operations       *operationRecorder
	commitErr        error
	commitContextErr error
}

func (t *fakeBatchTransaction) Commit(ctx context.Context) error {
	t.commitContextErr = ctx.Err()
	if t.operations != nil {
		t.operations.add("commit")
	}
	return t.commitErr
}

func (*fakeBatchTransaction) Rollback(context.Context) error {
	return nil
}

type fakeBatchQueries struct {
	operations     *operationRecorder
	rows           []sqlc.OutboxEvent
	listErr        error
	markErr        error
	batchSize      int32
	markContextErr error
}

func (q *fakeBatchQueries) ListPendingOutboxEventsForUpdate(
	_ context.Context,
	batchSize int32,
) ([]sqlc.OutboxEvent, error) {
	q.batchSize = batchSize
	return q.rows, q.listErr
}

func (q *fakeBatchQueries) MarkOutboxEventProcessed(
	ctx context.Context,
	id pgtype.UUID,
) (int64, error) {
	q.markContextErr = ctx.Err()
	if q.operations != nil {
		q.operations.add("mark " + id.String())
	}
	if q.markErr != nil {
		return 0, q.markErr
	}
	return 1, nil
}

type operationRecorder struct {
	mu         sync.Mutex
	operations []string
}

func (r *operationRecorder) add(operation string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.operations = append(r.operations, operation)
}

func (r *operationRecorder) snapshot() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.operations...)
}

func countOperation(operations []string, target string) int {
	count := 0
	for _, operation := range operations {
		if operation == target {
			count++
		}
	}
	return count
}

func mustTestShardCollision(t *testing.T, collisionCount int) ([]pgtype.UUID, pgtype.UUID) {
	t.Helper()
	byShard := make(map[int][]pgtype.UUID, outboxWorkerCount)
	var colliding []pgtype.UUID
	collidingShard := -1
	for candidate := 1; ; candidate++ {
		id := mustTestUUID(t, fmt.Sprintf("99999999-9999-4999-8999-%012d", candidate))
		shard := outboxShard(id.String())
		if colliding == nil {
			byShard[shard] = append(byShard[shard], id)
			if len(byShard[shard]) == collisionCount {
				colliding = byShard[shard]
				collidingShard = shard
			}
			continue
		}
		if shard != collidingShard {
			return colliding, id
		}
	}
}

func mustTestUUID(t *testing.T, value string) pgtype.UUID {
	t.Helper()

	var id pgtype.UUID
	if err := id.Scan(value); err != nil {
		t.Fatalf("parse test UUID: %v", err)
	}
	return id
}
