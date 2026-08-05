package outbox

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/smithautotest/clinic-visits/internal/database/sqlc"
)

var (
	ErrInvalidBatchSize = errors.New("outbox batch size must be positive")
	ErrNilConsumer      = errors.New("outbox consumer must not be nil")
)

type PersistedEvent struct {
	ID            string
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

	processed := 0
	for _, row := range rows {
		event := mapPersistedEvent(row)
		if err := p.consumer(ctx, event); err != nil {
			consumerErr := fmt.Errorf("consume outbox event %s: %w", event.ID, err)
			if commitErr := transaction.Commit(ctx); commitErr != nil {
				return 0, errors.Join(
					consumerErr,
					fmt.Errorf("commit successful outbox events before failed event %s: %w", event.ID, commitErr),
				)
			}

			return processed, consumerErr
		}

		rowsAffected, err := queries.MarkOutboxEventProcessed(ctx, row.ID)
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

	if err := transaction.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit outbox processing transaction: %w", err)
	}

	return processed, nil
}

func mapPersistedEvent(row sqlc.OutboxEvent) PersistedEvent {
	return PersistedEvent{
		ID:            row.ID.String(),
		AggregateType: row.AggregateType,
		AggregateID:   row.AggregateID.String(),
		EventType:     row.EventType,
		Payload:       row.Payload,
		CreatedAt:     row.CreatedAt.Time,
	}
}
