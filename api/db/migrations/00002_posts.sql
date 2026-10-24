-- +goose Up
CREATE TABLE posts (
    id              TEXT PRIMARY KEY,                -- ULID
    slug            TEXT UNIQUE,                     -- NULL until first publish, then fixed
    title           TEXT NOT NULL,
    description     TEXT NOT NULL DEFAULT '',
    body_md         TEXT NOT NULL DEFAULT '',
    cover_image_key TEXT,                            -- object name in Cloud Storage
    cover_alt       TEXT,
    status          TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    published_at    TEXT,                            -- RFC 3339; kept through unpublish
    created_at      TEXT NOT NULL,
    updated_at      TEXT NOT NULL,                   -- also the If-Match version
    created_by      TEXT NOT NULL,                   -- Auth0 sub
    updated_by      TEXT NOT NULL,
    deleted_at      TEXT                             -- soft delete
);

CREATE INDEX posts_published ON posts (status, published_at) WHERE deleted_at IS NULL;

-- +goose Down
DROP TABLE posts;
