-- +goose Up
SELECT 'up SQL query';

CREATE TABLE inbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    event_id UUID NOT NULL UNIQUE,
    correlation_id UUID NOT NULL,

    event_type VARCHAR(100) NOT NULL,

    payload JSONB,

    processed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
SELECT 'down SQL query';

DROP TABLE IF EXISTS inbox_events;