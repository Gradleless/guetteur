-- +goose Up
ALTER TABLE series ADD COLUMN tmdb_id INTEGER;

-- +goose Down
-- SQLite does not support DROP COLUMN; column is nullable and harmless if left.
