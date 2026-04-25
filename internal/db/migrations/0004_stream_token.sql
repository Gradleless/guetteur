-- +goose Up
ALTER TABLE releases ADD COLUMN stream_token TEXT;
UPDATE releases SET stream_token = lower(hex(randomblob(16))) WHERE stream_token IS NULL;
CREATE UNIQUE INDEX idx_releases_stream_token ON releases(stream_token);

-- +goose Down
DROP INDEX IF EXISTS idx_releases_stream_token;
ALTER TABLE releases DROP COLUMN stream_token;
