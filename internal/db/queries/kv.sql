-- name: GetKV :one
SELECT * FROM kv WHERE k = ? LIMIT 1;

-- name: SetKV :exec
INSERT INTO kv (k, v) VALUES (?, ?)
ON CONFLICT(k) DO UPDATE SET v = excluded.v;

-- name: DeleteKV :exec
DELETE FROM kv WHERE k = ?;
