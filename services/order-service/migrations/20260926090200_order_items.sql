-- +goose Up
SELECT 'up SQL query';

CREATE TABLE order_items (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    order_id UUID NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id VARCHAR NOT NULL,
    quantity INT NOT NULL CHECK (quantity > 0),
    price BIGINT NOT NULL CHECK (price >= 0),

    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_order_items_order_id ON order_items(order_id);
-- CREATE INDEX idx_order_items_product_id ON order_items(product_id);

-- +goose Down
SELECT 'down SQL query';

DROP TABLE IF EXISTS order_items;