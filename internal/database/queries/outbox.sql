-- name: CreateOutboxEvent :exec
INSERT INTO outbox_events (
    aggregate_type,
    aggregate_id,
    event_type,
    payload
)
VALUES (
    sqlc.arg('aggregate_type'),
    sqlc.arg('aggregate_id'),
    sqlc.arg('event_type'),
    sqlc.arg('payload')
);

-- name: ListPendingOutboxEventsForUpdate :many
SELECT
    candidate.id,
    candidate.aggregate_type,
    candidate.aggregate_id,
    candidate.event_type,
    candidate.payload,
    candidate.created_at,
    candidate.processed_at,
    candidate.event_sequence
FROM outbox_events AS candidate
WHERE candidate.processed_at IS NULL
  AND NOT EXISTS (
      SELECT 1
      FROM outbox_events AS earlier
      WHERE earlier.aggregate_type = candidate.aggregate_type
        AND earlier.aggregate_id = candidate.aggregate_id
        AND earlier.processed_at IS NULL
        AND earlier.event_sequence < candidate.event_sequence
  )
ORDER BY candidate.event_sequence
LIMIT sqlc.arg('batch_size')::int
FOR UPDATE OF candidate SKIP LOCKED;

-- name: MarkOutboxEventProcessed :execrows
UPDATE outbox_events
SET processed_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')
  AND processed_at IS NULL;
