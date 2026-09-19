-- +goose Up
SELECT 'up SQL query';

CREATE TABLE user_roles (
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    role    TEXT NOT NULL CHECK (role IN ('ROLE_USER', 'ROLE_MANAGER', 'ROLE_ADMIN')),
    PRIMARY KEY (user_id, role)
);

-- +goose Down
SELECT 'down SQL query';
