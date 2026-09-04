package outbox

import (
	"context"
	"errors"
	"fmt"
	"hash/fnv"
	"reflect"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
)

var (
	ErrInvalidBatchSize = errors.New("outbox batch size must be positive")
	ErrNilConsumer      = errors.New("outbox consumer must not be nil")
)

const (
	outboxWorkerCount     = 4
	outboxWorkerQueueSize = 1
	outboxDrainTimeout    = 5 * time.Second
)

type PersistedEvent struct {
	ID            string
	Sequence      int64
	AggregateType string
	AggregateID   string
	EventType     string
	Payload       []byte
	CreatedAt     time.Time
}

type Consumer func(context.Context, PersistedEvent) error

type Processor struct {
	beginBatch beginBatchFunc
	consumer   Consumer
}

type batchTransaction interface {
	Commit(context.Context) error
	Rollback(context.Context) error
}

type batchQueries interface {
	ListPendingOutboxEventsForUpdate(context.Context, int32) ([]sqlc.OutboxEvent, error)
	MarkOutboxEventProcessed(context.Context, pgtype.UUID) (int64, error)
}

type beginBatchFunc func(context.Context) (batchTransaction, batchQueries, error)

type queuedEvent struct {
	position int
	event    PersistedEvent
}

type eventResult struct {
	position int
	err      error
}

func NewProcessor(pool *pgxpool.Pool, consumer Consumer) *Processor {
	return newProcessor(func(ctx context.Context) (batchTransaction, batchQueries, error) {
		transaction, err := pool.Begin(ctx)
		if err != nil {
			return nil, nil, err
		}

		return transaction, sqlc.New(transaction), nil
	}, consumer)
}

func newProcessor(beginBatch beginBatchFunc, consumer Consumer) *Processor {
	return &Processor{
		beginBatch: beginBatch,
		consumer:   consumer,
	}
}

func (p *Processor) ProcessBatch(ctx context.Context, batchSize int32) (int, error) {
	if batchSize <= 0 {
		return 0, ErrInvalidBatchSize
	}
	if p.consumer == nil {
		return 0, ErrNilConsumer
	}

	transaction, queries, err := p.beginBatch(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin outbox processing transaction: %w", err)
	}
	defer transaction.Rollback(ctx)

	rows, err := queries.ListPendingOutboxEventsForUpdate(ctx, batchSize)
	if err != nil {
		return 0, fmt.Errorf("select pending outbox events: %w", err)
	}

	results, dispatchErr := p.consumeBatch(ctx, rows)

	// A completed Elasticsearch write must still be acknowledged if shutdown
	// cancels ctx between the write and the database update.
	drainCtx, cancelDrain := context.WithTimeout(context.WithoutCancel(ctx), outboxDrainTimeout)
	defer cancelDrain()

	processed := 0
	var processingErrors []error
	for _, result := range results {
		event := mapPersistedEvent(rows[result.position])
		if result.err != nil {
			processingErrors = append(processingErrors, fmt.Errorf("consume outbox event %s: %w", event.ID, result.err))
			continue
		}

		rowsAffected, err := queries.MarkOutboxEventProcessed(drainCtx, rows[result.position].ID)
		if err != nil {
			return 0, fmt.Errorf("mark outbox event %s processed: %w", event.ID, err)
		}
		if rowsAffected != 1 {
			return 0, fmt.Errorf(
				"mark outbox event %s processed: expected 1 affected row, got %d",
				event.ID,
				rowsAffected,
			)
		}
		processed++
	}
	if dispatchErr != nil {
		processingErrors = append(processingErrors, dispatchErr)
	} else if ctx.Err() != nil {
		processingErrors = append(processingErrors, ctx.Err())
	}

	if err := transaction.Commit(drainCtx); err != nil {
		return 0, errors.Join(
			errors.Join(processingErrors...),
			fmt.Errorf("commit outbox processing transaction: %w", err),
		)
	}

	return processed, errors.Join(processingErrors...)
}

// consumeBatch is a bounded producer/four-worker pipeline. Each aggregate is
// deterministically assigned to one worker, whose channel preserves the query
// order. Results are returned by original position for deterministic
// acknowledgement and error reporting.
func (p *Processor) consumeBatch(ctx context.Context, rows []sqlc.OutboxEvent) ([]eventResult, error) {
	workerQueues := make([]chan queuedEvent, outboxWorkerCount)
	resultQueue := make(chan eventResult, outboxWorkerCount)
	resultsByPosition := make([]eventResult, len(rows))
	completed := make([]bool, len(rows))
	acknowledgerDone := make(chan struct{})
	go func() {
		defer close(acknowledgerDone)
		for result := range resultQueue {
			resultsByPosition[result.position] = result
			completed[result.position] = true
		}
	}()

	var workers sync.WaitGroup
	workers.Add(outboxWorkerCount)

	for worker := range outboxWorkerCount {
		workerQueues[worker] = make(chan queuedEvent, outboxWorkerQueueSize)
		go func(queue <-chan queuedEvent) {
			defer workers.Done()
			failedAggregates := make(map[string]struct{})
			for item := range queue {
				aggregateKey := item.event.AggregateType + ":" + item.event.AggregateID
				if _, blocked := failedAggregates[aggregateKey]; blocked {
					resultQueue <- eventResult{
						position: item.position,
						err:      fmt.Errorf("earlier event for aggregate %s failed", aggregateKey),
					}
					continue
				}

				err := p.consumer(ctx, item.event)
				if err != nil {
					failedAggregates[aggregateKey] = struct{}{}
				}
				resultQueue <- eventResult{position: item.position, err: err}
			}
		}(workerQueues[worker])
	}

	pending := make([][]queuedEvent, outboxWorkerCount)
	for position, row := range rows {
		event := mapPersistedEvent(row)
		shard := outboxShard(event.AggregateID)
		pending[shard] = append(pending[shard], queuedEvent{position: position, event: event})
	}

	remaining := len(rows)
	var dispatchErr error
	for remaining > 0 {
		if err := ctx.Err(); err != nil {
			dispatchErr = err
			break
		}
		cases := []reflect.SelectCase{{Dir: reflect.SelectRecv, Chan: reflect.ValueOf(ctx.Done())}}
		caseShards := make([]int, 0, outboxWorkerCount)
		for shard, events := range pending {
			if len(events) == 0 {
				continue
			}
			cases = append(cases, reflect.SelectCase{
				Dir:  reflect.SelectSend,
				Chan: reflect.ValueOf(workerQueues[shard]),
				Send: reflect.ValueOf(events[0]),
			})
			caseShards = append(caseShards, shard)
		}

		chosen, _, _ := reflect.Select(cases)
		if chosen == 0 {
			dispatchErr = ctx.Err()
			break
		}
		shard := caseShards[chosen-1]
		pending[shard] = pending[shard][1:]
		remaining--
	}
	for _, queue := range workerQueues {
		close(queue)
	}
	workers.Wait()
	close(resultQueue)
	<-acknowledgerDone

	results := make([]eventResult, 0, len(rows)-remaining)
	for position, done := range completed {
		if done {
			results = append(results, resultsByPosition[position])
		}
	}
	return results, dispatchErr
}

func outboxShard(aggregateID string) int {
	hash := fnv.New32a()
	_, _ = hash.Write([]byte(aggregateID))
	return int(hash.Sum32() % outboxWorkerCount)
}

func mapPersistedEvent(row sqlc.OutboxEvent) PersistedEvent {
	return PersistedEvent{
		ID:            row.ID.String(),
		Sequence:      row.EventSequence,
		AggregateType: row.AggregateType,
		AggregateID:   row.AggregateID.String(),
		EventType:     row.EventType,
		Payload:       row.Payload,
		CreatedAt:     row.CreatedAt.Time,
	}
}
