-- +goose Up
ALTER TABLE series ADD COLUMN description    TEXT;
ALTER TABLE series ADD COLUMN studio         TEXT;
ALTER TABLE series ADD COLUMN score_anilist  REAL;
ALTER TABLE series ADD COLUMN genres_json    TEXT NOT NULL DEFAULT '[]';
ALTER TABLE series ADD COLUMN source         TEXT;
ALTER TABLE series ADD COLUMN season_year    INTEGER;
ALTER TABLE series ADD COLUMN season_name    TEXT;
ALTER TABLE series ADD COLUMN characters_json TEXT NOT NULL DEFAULT '[]';
ALTER TABLE series ADD COLUMN relations_json  TEXT NOT NULL DEFAULT '[]';

-- +goose Down
-- SQLite does not support DROP COLUMN reliably; columns are nullable/defaulted and harmless if left.
