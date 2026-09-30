-- +goose Up
SELECT 'up SQL query';

CREATE TABLE orders (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL,
    idempotency_key UUID NOT NULL UNIQUE,
    status TEXT NOT NULL CHECK (status IN (
        'NEW',
        'RESERVED',
        'PAID',
        'SHIPPED',
        'COMPLETED',
        'CANCELLED'
    )),
    total_price BIGINT NOT NULL CHECK (total_price >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_user_id ON orders(user_id);
-- CREATE INDEX idx_orders_created_at ON orders(created_at DESC);

-- +goose Down
SELECT 'down SQL query';

DROP TABLE IF EXISTS orders;