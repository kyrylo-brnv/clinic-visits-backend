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
