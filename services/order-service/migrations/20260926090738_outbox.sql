-- +goose Up
SELECT 'up SQL query';

CREATE TABLE outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    correlation_id UUID NOT NULL,

    aggregate_type TEXT NOT NULL,
    aggregate_id UUID NOT NULL,

    event_type TEXT NOT NULL,

    payload JSONB NOT NULL,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    published_at TIMESTAMPTZ,

    locked_until TIMESTAMPTZ,
    locked_by TEXT
);

CREATE INDEX idx_outbox_correlation_id ON outbox(correlation_id);
CREATE INDEX idx_outbox_event_type ON outbox(event_type);
CREATE INDEX IF NOT EXISTS idx_outbox_available
    ON outbox (created_at)
    WHERE published_at IS NULL;

-- +goose Down
SELECT 'down SQL query';

DROP TABLE IF EXISTS order_items;