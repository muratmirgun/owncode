-- +goose Up
ALTER TABLE messages ADD COLUMN reasoning_effort TEXT;

-- +goose Down
ALTER TABLE messages DROP COLUMN reasoning_effort;