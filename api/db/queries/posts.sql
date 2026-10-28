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
