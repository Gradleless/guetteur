-- name: GetSeriesByID :one
SELECT * FROM series WHERE id = ? LIMIT 1;

-- name: ListAllSeries :many
SELECT * FROM series ORDER BY title_romaji;

-- name: ListFollowedSeries :many
SELECT * FROM series WHERE follow_state = 1 ORDER BY title_romaji;

-- name: UpsertSeries :exec
INSERT INTO series (
    id, title_romaji, title_english, title_native,
    aliases_json, season_number, total_episodes, status,
    follow_state, preferred_groups_json, cover_url, anilist_url, tmdb_id,
    description, studio, score_anilist, genres_json, source,
    season_year, season_name, characters_json, relations_json,
    updated_at
) VALUES (
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?
)
ON CONFLICT(id) DO UPDATE SET
    title_romaji          = excluded.title_romaji,
    title_english         = excluded.title_english,
    title_native          = excluded.title_native,
    aliases_json          = excluded.aliases_json,
    season_number         = excluded.season_number,
    total_episodes        = excluded.total_episodes,
    status                = excluded.status,
    preferred_groups_json = excluded.preferred_groups_json,
    cover_url             = excluded.cover_url,
    anilist_url           = excluded.anilist_url,
    tmdb_id               = excluded.tmdb_id,
    description           = excluded.description,
    studio                = excluded.studio,
    score_anilist         = excluded.score_anilist,
    genres_json           = excluded.genres_json,
    source                = excluded.source,
    season_year           = excluded.season_year,
    season_name           = excluded.season_name,
    characters_json       = excluded.characters_json,
    relations_json        = excluded.relations_json,
    updated_at            = excluded.updated_at;

-- name: ListSeriesByFollowState :many
SELECT * FROM series WHERE follow_state = ? ORDER BY title_romaji;

-- name: UpdateSeriesFollowState :exec
UPDATE series SET follow_state = ? WHERE id = ?;

-- name: UpdateSeriesTmdbID :exec
UPDATE series SET tmdb_id = ? WHERE id = ?;

-- name: UpdateSeriesUserFields :exec
UPDATE series
SET preferred_groups_json = ?,
    aliases_json          = ?,
    season_number         = ?
WHERE id = ?;

-- name: GetSeriesStats :one
SELECT
    COUNT(*)                                    AS dl_count,
    COALESCE(SUM(size_bytes), 0)               AS total_bytes,
    MIN(detected_at)                            AS first_dl,
    MAX(detected_at)                            AS last_dl
FROM releases
WHERE series_id = ? AND status = 'completed';

-- name: ListNextAiringAllSeries :many
SELECT series_id, episode, airing_at FROM (
    SELECT series_id, episode, airing_at,
           ROW_NUMBER() OVER (PARTITION BY series_id ORDER BY airing_at ASC) AS rn
    FROM airing_schedule WHERE airing_at > ?
) WHERE rn = 1;

-- name: ListSeriesDownloadCounts :many
SELECT series_id, COUNT(*) AS dl_count
FROM releases
WHERE status = 'completed' AND series_id IS NOT NULL
GROUP BY series_id;
