-- name: CreatePost :exec
INSERT INTO posts (
    id, title, description, body_md, cover_image_key, cover_alt,
    status, created_at, updated_at, created_by, updated_by
) VALUES (
    sqlc.arg(id), sqlc.arg(title), sqlc.arg(description), sqlc.arg(body_md),
    sqlc.narg(cover_image_key), sqlc.narg(cover_alt),
    'draft', sqlc.arg(now), sqlc.arg(now), sqlc.arg(user_id), sqlc.arg(user_id)
);

-- name: GetPost :one
SELECT * FROM posts WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: ListPosts :many
SELECT * FROM posts WHERE deleted_at IS NULL ORDER BY updated_at DESC;

-- name: ListPublishedPosts :many
SELECT * FROM posts
WHERE status = 'published' AND deleted_at IS NULL
ORDER BY published_at DESC;

-- name: UpdatePost :execrows
UPDATE posts SET
    title = sqlc.arg(title),
    description = sqlc.arg(description),
    body_md = sqlc.arg(body_md),
    cover_image_key = sqlc.narg(cover_image_key),
    cover_alt = sqlc.narg(cover_alt),
    published_at = COALESCE(sqlc.narg(published_at), published_at),
    updated_at = sqlc.arg(now),
    updated_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(id) AND updated_at = sqlc.arg(expected_updated_at) AND deleted_at IS NULL;

-- name: PublishPost :execrows
UPDATE posts SET
    status = 'published',
    slug = COALESCE(slug, sqlc.arg(slug)),
    published_at = COALESCE(published_at, sqlc.arg(now)),
    updated_at = sqlc.arg(now),
    updated_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: UnpublishPost :execrows
UPDATE posts SET status = 'draft', updated_at = sqlc.arg(now), updated_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: SoftDeletePost :execrows
UPDATE posts SET updated_at = sqlc.arg(now), deleted_at = sqlc.arg(now), updated_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(id) AND deleted_at IS NULL;

-- name: RestorePost :execrows
UPDATE posts SET deleted_at = NULL, updated_at = sqlc.arg(now), updated_by = sqlc.arg(user_id)
WHERE id = sqlc.arg(id) AND deleted_at IS NOT NULL;

-- name: SlugTaken :one
-- Deleted posts keep their slug, so they count.
SELECT EXISTS (SELECT 1 FROM posts WHERE slug = sqlc.arg(slug)) AS taken;

-- name: ImportPost :execrows
-- Adds an already-published post from the old site, keeping its slug and
-- date. A slug that is already taken is left alone, so reruns are safe.
INSERT INTO posts (
    id, slug, title, description, body_md, cover_image_key, cover_alt,
    status, published_at, created_at, updated_at, created_by, updated_by
) VALUES (
    sqlc.arg(id), sqlc.arg(slug), sqlc.arg(title), sqlc.arg(description), sqlc.arg(body_md),
    sqlc.narg(cover_image_key), sqlc.narg(cover_alt),
    'published', sqlc.arg(published_at), sqlc.arg(now), sqlc.arg(now), sqlc.arg(user_id), sqlc.arg(user_id)
)
ON CONFLICT (slug) DO NOTHING;

-- name: AllPosts :many
-- Every post, deleted ones included, for backups.
SELECT * FROM posts ORDER BY created_at, id;

-- name: CountPosts :one
SELECT COUNT(*) FROM posts;

-- name: RestorePostRow :exec
-- Puts back a post exactly as a backup holds it.
INSERT INTO posts (
    id, slug, title, description, body_md, cover_image_key, cover_alt, status,
    published_at, created_at, updated_at, created_by, updated_by, deleted_at
) VALUES (
    sqlc.arg(id), sqlc.narg(slug), sqlc.arg(title), sqlc.arg(description), sqlc.arg(body_md),
    sqlc.narg(cover_image_key), sqlc.narg(cover_alt), sqlc.arg(status),
    sqlc.narg(published_at), sqlc.arg(created_at), sqlc.arg(updated_at),
    sqlc.arg(created_by), sqlc.arg(updated_by), sqlc.narg(deleted_at)
);
