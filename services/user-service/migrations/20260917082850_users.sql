-- +goose Up
SELECT 'up SQL query';

CREATE TABLE users (
    id                UUID PRIMARY KEY,
    email             TEXT NOT NULL UNIQUE,
    first_name        TEXT,
    last_name         TEXT,
    avatar_url        TEXT,
    phone             TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);

CREATE INDEX idx_users_deleted_at ON users(deleted_at) WHERE deleted_at IS NULL;

-- +goose Down
SELECT 'down SQL query';

DROP TABLE IF EXISTS users;