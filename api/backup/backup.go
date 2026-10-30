// Package backup exports every post to JSON and restores an export into
// an empty database. The weekly backup job ("api backup") writes exports to
// a private bucket; "api restore" loads one back.
package backup

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/ras251093/hgn-bowls/api/db"
)

// version is the export format. Bump it if Post changes incompatibly.
const version = 1

// File is an export: every post, deleted ones included.
type File struct {
	Version    int    `json:"version"`
	ExportedAt string `json:"exportedAt"`
	Posts      []Post `json:"posts"`
}

// Post is one row of the posts table, with NULLs as JSON null.
type Post struct {
	ID            string  `json:"id"`
	Slug          *string `json:"slug"`
	Title         string  `json:"title"`
	Description   string  `json:"description"`
	BodyMd        string  `json:"bodyMd"`
	CoverImageKey *string `json:"coverImageKey"`
	CoverAlt      *string `json:"coverAlt"`
	Status        string  `json:"status"`
	PublishedAt   *string `json:"publishedAt"`
	CreatedAt     string  `json:"createdAt"`
	UpdatedAt     string  `json:"updatedAt"`
	CreatedBy     string  `json:"createdBy"`
	UpdatedBy     string  `json:"updatedBy"`
	DeletedAt     *string `json:"deletedAt"`
}

// Lister reads every post (*db.Queries in production).
type Lister interface {
	AllPosts(ctx context.Context) ([]db.Post, error)
}

// ObjectName is where an export taken at t is stored; names sort by time.
func ObjectName(t time.Time) string {
	return "posts/" + t.UTC().Format("2006-01-02T150405Z") + ".json"
}

// Export writes every post to w as JSON.
func Export(ctx context.Context, store Lister, w io.Writer, now time.Time) error {
	rows, err := store.AllPosts(ctx)
	if err != nil {
		return fmt.Errorf("export posts: %w", err)
	}
	f := File{Version: version, ExportedAt: now.UTC().Format(time.RFC3339), Posts: make([]Post, 0, len(rows))}
	for _, r := range rows {
		f.Posts = append(f.Posts, Post{
			ID: r.ID, Slug: ptr(r.Slug), Title: r.Title, Description: r.Description, BodyMd: r.BodyMd,
			CoverImageKey: ptr(r.CoverImageKey), CoverAlt: ptr(r.CoverAlt), Status: r.Status,
			PublishedAt: ptr(r.PublishedAt), CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
			CreatedBy: r.CreatedBy, UpdatedBy: r.UpdatedBy, DeletedAt: ptr(r.DeletedAt),
		})
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(f); err != nil {
		return fmt.Errorf("export posts: %w", err)
	}
	return nil
}

// ErrNotEmpty means the database already has posts, so a restore would mix
// the backup with them.
var ErrNotEmpty = errors.New("the database already has posts; restore into an empty one")

// Restore loads an export into conn, all or nothing, and returns how many
// posts it restored. The database must be migrated and have no posts.
func Restore(ctx context.Context, conn *sql.DB, r io.Reader) (int, error) {
	var f File
	if err := json.NewDecoder(r).Decode(&f); err != nil {
		return 0, fmt.Errorf("restore: read backup: %w", err)
	}
	if f.Version != version {
		return 0, fmt.Errorf("restore: backup is version %d; this program reads version %d", f.Version, version)
	}

	tx, err := conn.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("restore: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := db.New(tx)
	n, err := q.CountPosts(ctx)
	if err != nil {
		return 0, fmt.Errorf("restore: %w", err)
	}
	if n > 0 {
		return 0, ErrNotEmpty
	}
	for _, p := range f.Posts {
		err := q.RestorePostRow(ctx, db.RestorePostRowParams{
			ID: p.ID, Slug: null(p.Slug), Title: p.Title, Description: p.Description, BodyMd: p.BodyMd,
			CoverImageKey: null(p.CoverImageKey), CoverAlt: null(p.CoverAlt), Status: p.Status,
			PublishedAt: null(p.PublishedAt), CreatedAt: p.CreatedAt, UpdatedAt: p.UpdatedAt,
			CreatedBy: p.CreatedBy, UpdatedBy: p.UpdatedBy, DeletedAt: null(p.DeletedAt),
		})
		if err != nil {
			return 0, fmt.Errorf("restore post %s: %w", p.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("restore: %w", err)
	}
	return len(f.Posts), nil
}

func ptr(s sql.NullString) *string {
	if !s.Valid {
		return nil
	}
	return &s.String
}

func null(s *string) sql.NullString {
	if s == nil {
		return sql.NullString{}
	}
	return sql.NullString{String: *s, Valid: true}
}
