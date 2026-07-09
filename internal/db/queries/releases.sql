-- name: GetReleaseByInfoHash :one
SELECT * FROM releases WHERE info_hash = ? LIMIT 1;

-- name: GetReleaseByStreamToken :one
SELECT * FROM releases WHERE stream_token = ? LIMIT 1;

-- name: ListReleasesBySeriesID :many
SELECT * FROM releases WHERE series_id = ? ORDER BY episode;

-- name: ListReleasesByStatus :many
SELECT * FROM releases WHERE status = ? ORDER BY detected_at;

-- name: InsertRelease :exec
INSERT INTO releases (
    info_hash, series_id, raw_title, episode, episode_end,
    group_name, resolution, magnet, nyaa_url,
    seeders, leechers, size_bytes, detected_at,
    status, download_path, media_path, progress, error_msg, stream_token
) VALUES (
    ?, ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?,
    ?, ?, ?, ?, ?, ?
)
ON CONFLICT(info_hash) DO NOTHING;

-- name: UpdateReleaseStatus :exec
UPDATE releases SET status = ?, error_msg = ? WHERE info_hash = ?;

-- name: UpdateReleaseProgress :exec
UPDATE releases SET progress = ? WHERE info_hash = ?;

-- name: UpdateReleaseDownloadPath :exec
UPDATE releases SET download_path = ? WHERE info_hash = ?;

-- name: UpdateReleaseMediaPath :exec
UPDATE releases SET media_path = ?, status = 'completed' WHERE info_hash = ?;

-- name: UpdateReleaseForRedownload :exec
UPDATE releases
SET status = 'queued', progress = 0, media_path = NULL, error_msg = NULL
WHERE info_hash = ?;

-- name: DeleteReleasesBySeriesID :exec
DELETE FROM releases WHERE series_id = ?;

-- name: ListReleasesWithSeriesByStatus :many
SELECT r.info_hash, r.series_id, r.raw_title, r.episode, r.episode_end,
       r.group_name, r.resolution, r.magnet, r.nyaa_url,
       r.seeders, r.leechers, r.size_bytes, r.detected_at,
       r.status, r.download_path, r.media_path, r.progress, r.error_msg, r.stream_token,
       s.title_english, s.title_romaji, s.cover_url
FROM releases r
LEFT JOIN series s ON s.id = r.series_id
WHERE r.status = ?
ORDER BY r.detected_at DESC;
