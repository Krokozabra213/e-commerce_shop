-- +goose Up
SELECT 'up SQL query';

CREATE TABLE reservations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL,
    product_id VARCHAR NOT NULL REFERENCES stocks(product_id),
    quantity INT NOT NULL CHECK (quantity > 0),
    status TEXT NOT NULL DEFAULT 'ACTIVE'
        CHECK (status IN ('ACTIVE', 'RELEASED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    released_at TIMESTAMPTZ
);

CREATE INDEX idx_reservations_order
    ON reservations (order_id, status);

-- +goose Down
SELECT 'down SQL query';

DROP TABLE IF EXISTS reservations;