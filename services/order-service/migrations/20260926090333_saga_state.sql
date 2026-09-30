-- +goose Up
SELECT 'up SQL query';

CREATE TABLE saga_state (
    order_id UUID PRIMARY KEY REFERENCES orders(id),
    correlation_id UUID NOT NULL UNIQUE,

    current_step VARCHAR(50) NOT NULL CHECK (current_step IN (
        'RESERVING_INVENTORY',
        'CHARGING_PAYMENT',
        'COMPENSATING_PAYMENT',
        'COMPENSATING_INVENTORY',
        'COMPLETED'
    )),

    status VARCHAR(20) NOT NULL CHECK (status IN (
        'IN_PROGRESS',
        'COMPLETED',
        'FAILED',
        'COMPENSATING',
        'COMPENSATED'
    )),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- CREATE INDEX idx_saga_state_status ON saga_state(status);
-- CREATE INDEX idx_saga_state_step ON saga_state(current_step);

-- +goose Down
SELECT 'down SQL query';

DROP TABLE IF EXISTS order_items;