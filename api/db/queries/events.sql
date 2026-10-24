-- name: SaveEventsSnapshot :exec
INSERT INTO events_snapshot (id, data, fetched_at)
VALUES (1, sqlc.arg(data), sqlc.arg(fetched_at))
ON CONFLICT (id) DO UPDATE SET data = excluded.data, fetched_at = excluded.fetched_at;

-- name: LoadEventsSnapshot :one
SELECT data, fetched_at FROM events_snapshot WHERE id = 1;
