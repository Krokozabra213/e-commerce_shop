-- +goose Up
SELECT 'up SQL query';

CREATE TABLE stocks (
    product_id VARCHAR PRIMARY KEY,
    available_quantity INT NOT NULL DEFAULT 0 CHECK (available_quantity >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
SELECT 'down SQL query';

DROP TABLE IF EXISTS stock;