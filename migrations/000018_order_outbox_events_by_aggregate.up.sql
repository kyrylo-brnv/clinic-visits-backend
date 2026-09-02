DROP INDEX idx_outbox_events_pending_created_at;

ALTER TABLE outbox_events
ADD COLUMN event_sequence BIGINT;

WITH ordered_events AS (
    SELECT
        id,
        row_number() OVER (ORDER BY created_at, id) AS event_sequence
    FROM outbox_events
)
UPDATE outbox_events
SET event_sequence = ordered_events.event_sequence
FROM ordered_events
WHERE outbox_events.id = ordered_events.id;

ALTER TABLE outbox_events
ALTER COLUMN event_sequence SET NOT NULL;

ALTER TABLE outbox_events
ALTER COLUMN event_sequence ADD GENERATED ALWAYS AS IDENTITY;

SELECT setval(
    pg_get_serial_sequence('outbox_events', 'event_sequence'),
    COALESCE(MAX(event_sequence), 0) + 1,
    false
)
FROM outbox_events;

CREATE INDEX idx_outbox_events_pending_sequence
ON outbox_events (event_sequence)
WHERE processed_at IS NULL;

CREATE INDEX idx_outbox_events_pending_aggregate_sequence
ON outbox_events (aggregate_type, aggregate_id, event_sequence)
WHERE processed_at IS NULL;
