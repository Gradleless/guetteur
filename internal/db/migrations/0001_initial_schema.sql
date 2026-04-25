-- +goose Up

CREATE TABLE series (
    id                    INTEGER PRIMARY KEY,
    title_romaji          TEXT NOT NULL,
    title_english         TEXT,
    title_native          TEXT,
    aliases_json          TEXT NOT NULL DEFAULT '[]',
    season_number         INTEGER DEFAULT 1,
    total_episodes        INTEGER,
    status                TEXT NOT NULL,
    follow_state          INTEGER NOT NULL DEFAULT 0,
    preferred_groups_json TEXT NOT NULL DEFAULT '[]',
    cover_url             TEXT,
    anilist_url           TEXT,
    updated_at            TIMESTAMP NOT NULL
);

CREATE INDEX idx_series_follow_state ON series(follow_state);

CREATE TABLE airing_schedule (
    series_id INTEGER NOT NULL REFERENCES series(id) ON DELETE CASCADE,
    episode   INTEGER NOT NULL,
    airing_at TIMESTAMP NOT NULL,
    PRIMARY KEY (series_id, episode)
);

CREATE TABLE releases (
    info_hash     TEXT PRIMARY KEY,
    series_id     INTEGER REFERENCES series(id) ON DELETE SET NULL,
    raw_title     TEXT NOT NULL,
    episode       INTEGER,
    episode_end   INTEGER,
    group_name    TEXT,
    resolution    TEXT,
    magnet        TEXT NOT NULL,
    nyaa_url      TEXT,
    seeders       INTEGER,
    leechers      INTEGER,
    size_bytes    INTEGER,
    detected_at   TIMESTAMP NOT NULL,
    status        TEXT NOT NULL,
    download_path TEXT,
    media_path    TEXT,
    progress      REAL NOT NULL DEFAULT 0,
    error_msg     TEXT
);

CREATE INDEX idx_releases_series_ep ON releases(series_id, episode);
CREATE INDEX idx_releases_status ON releases(status);

CREATE TABLE kv (
    k TEXT PRIMARY KEY,
    v TEXT NOT NULL
);

CREATE TABLE app_settings (
    id                    INTEGER PRIMARY KEY CHECK (id = 1),
    poll_interval_seconds INTEGER
);

-- +goose Down

DROP TABLE IF EXISTS app_settings;
DROP TABLE IF EXISTS kv;
DROP INDEX IF EXISTS idx_releases_status;
DROP INDEX IF EXISTS idx_releases_series_ep;
DROP TABLE IF EXISTS releases;
DROP TABLE IF EXISTS airing_schedule;
DROP INDEX IF EXISTS idx_series_follow_state;
DROP TABLE IF EXISTS series;
