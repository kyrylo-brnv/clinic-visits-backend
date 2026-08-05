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
    id,
    aggregate_type,
    aggregate_id,
    event_type,
    payload,
    created_at,
    processed_at
FROM outbox_events
WHERE processed_at IS NULL
ORDER BY created_at, id
LIMIT sqlc.arg('batch_size')::int
FOR UPDATE SKIP LOCKED;

-- name: MarkOutboxEventProcessed :execrows
UPDATE outbox_events
SET processed_at = CURRENT_TIMESTAMP
WHERE id = sqlc.arg('id')
  AND processed_at IS NULL;
