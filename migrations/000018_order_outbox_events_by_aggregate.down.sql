DROP INDEX idx_outbox_events_pending_aggregate_sequence;
DROP INDEX idx_outbox_events_pending_sequence;

ALTER TABLE outbox_events
DROP COLUMN event_sequence;

CREATE INDEX idx_outbox_events_pending_created_at
ON outbox_events (created_at, id)
WHERE processed_at IS NULL;
