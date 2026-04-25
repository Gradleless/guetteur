-- name: UpsertAiringSchedule :exec
INSERT INTO airing_schedule (series_id, episode, airing_at)
VALUES (?, ?, ?)
ON CONFLICT(series_id, episode) DO UPDATE SET airing_at = excluded.airing_at;

-- name: ListAiringScheduleForSeries :many
SELECT * FROM airing_schedule WHERE series_id = ? ORDER BY episode;

-- name: DeleteAiringScheduleForSeries :exec
DELETE FROM airing_schedule WHERE series_id = ?;

-- name: GetNextAiringForSeries :one
SELECT * FROM airing_schedule
WHERE series_id = ? AND airing_at > ?
ORDER BY airing_at ASC
LIMIT 1;

-- name: ListScheduleInWindow :many
SELECT
    a.series_id,
    a.episode,
    a.airing_at,
    s.title_english,
    s.title_romaji,
    s.cover_url,
    s.follow_state
FROM airing_schedule a
JOIN series s ON s.id = a.series_id
WHERE a.airing_at >= ? AND a.airing_at <= ?
ORDER BY a.airing_at ASC;
