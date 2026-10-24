package posts

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"

	"github.com/ras251093/hgn-bowls/api/db"
)

// Store is the post storage the service needs (*db.Queries in production).
type Store interface {
	CreatePost(ctx context.Context, arg db.CreatePostParams) error
	GetPost(ctx context.Context, id string) (db.Post, error)
	ListPosts(ctx context.Context) ([]db.Post, error)
	ListPublishedPosts(ctx context.Context) ([]db.Post, error)
	UpdatePost(ctx context.Context, arg db.UpdatePostParams) (int64, error)
	PublishPost(ctx context.Context, arg db.PublishPostParams) (int64, error)
	UnpublishPost(ctx context.Context, arg db.UnpublishPostParams) (int64, error)
	SoftDeletePost(ctx context.Context, arg db.SoftDeletePostParams) (int64, error)
	RestorePost(ctx context.Context, arg db.RestorePostParams) (int64, error)
	SlugTaken(ctx context.Context, slug sql.NullString) (bool, error)
}

// Publisher starts a rebuild of the public site (site.Pages in production).
type Publisher interface {
	Rebuild(ctx context.Context) error
}

// ImageSigner gives the site build a short-lived link to an image (media.Bucket in production).
type ImageSigner interface {
	SignedReadURL(ctx context.Context, key string) (string, error)
}

// Service holds the rules for changing posts.
type Service struct {
	store Store
	now   func() time.Time
	newID func() string
}

// Option changes how a Service is built.
type Option func(*Service)

// WithClock sets the clock used for timestamps (tests use a fixed one).
func WithClock(now func() time.Time) Option { return func(s *Service) { s.now = now } }

// WithIDs sets how new post IDs are made (tests use predictable ones).
func WithIDs(newID func() string) Option { return func(s *Service) { s.newID = newID } }

// NewService returns a Service storing posts in store.
func NewService(store Store, opts ...Option) *Service {
	s := &Service{store: store, now: time.Now, newID: func() string { return ulid.Make().String() }}
	for _, o := range opts {
		o(s)
	}
	return s
}

// List returns every live post, drafts included, most recently changed first.
func (s *Service) List(ctx context.Context) ([]Post, error) {
	rows, err := s.store.ListPosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list posts: %w", err)
	}
	out := make([]Post, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(r))
	}
	return out, nil
}

// Get returns one live post.
func (s *Service) Get(ctx context.Context, id string) (Post, error) {
	row, err := s.store.GetPost(ctx, id)
	if errors.Is(err, sql.ErrNoRows) {
		return Post{}, ErrNotFound
	}
	if err != nil {
		return Post{}, fmt.Errorf("get post %s: %w", id, err)
	}
	return fromRow(row), nil
}

// Create saves a new draft.
func (s *Service) Create(ctx context.Context, d Draft, userID string) (Post, error) {
	if err := d.Validate(); err != nil {
		return Post{}, err
	}
	id := s.newID()
	err := s.store.CreatePost(ctx, db.CreatePostParams{
		ID: id, Title: d.Title, Description: d.Description, BodyMd: d.BodyMd,
		CoverImageKey: null(d.CoverImageKey), CoverAlt: null(d.CoverAlt),
		Now: s.timestamp(), UserID: userID,
	})
	if err != nil {
		return Post{}, fmt.Errorf("create post: %w", err)
	}
	return s.Get(ctx, id)
}

// Update saves an edit. version is the updatedAt the editor loaded; if the
// post has changed since, nothing is saved and ErrConflict is returned.
func (s *Service) Update(ctx context.Context, id, version string, d Draft, userID string) (Post, error) {
	if err := d.Validate(); err != nil {
		return Post{}, err
	}
	n, err := s.store.UpdatePost(ctx, db.UpdatePostParams{
		ID: id, ExpectedUpdatedAt: version,
		Title: d.Title, Description: d.Description, BodyMd: d.BodyMd,
		CoverImageKey: null(d.CoverImageKey), CoverAlt: null(d.CoverAlt),
		PublishedAt: null(d.PublishedAt),
		Now:         s.timestamp(), UserID: userID,
	})
	if err != nil {
		return Post{}, fmt.Errorf("update post %s: %w", id, err)
	}
	if n == 0 {
		// Either the post is gone or the version is stale; Get tells which.
		if _, err := s.Get(ctx, id); err != nil {
			return Post{}, err
		}
		return Post{}, ErrConflict
	}
	return s.Get(ctx, id)
}

// Publish makes a post live. The first publish fixes its slug and date;
// publishing again changes nothing.
func (s *Service) Publish(ctx context.Context, id, userID string) (Post, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return Post{}, err
	}
	if p.Status == "published" {
		return p, nil
	}

	slug := sql.NullString{}
	if p.Slug == nil {
		free, err := s.freeSlug(ctx, slugify(p.Title))
		if err != nil {
			return Post{}, err
		}
		slug = sql.NullString{String: free, Valid: true}
	}
	n, err := s.store.PublishPost(ctx, db.PublishPostParams{ID: id, Slug: slug, Now: s.timestamp(), UserID: userID})
	if err != nil {
		return Post{}, fmt.Errorf("publish post %s: %w", id, err)
	}
	if n == 0 {
		return Post{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// freeSlug returns base, or base-2, base-3… if it is already used.
func (s *Service) freeSlug(ctx context.Context, base string) (string, error) {
	for i := 1; i <= 100; i++ {
		candidate := base
		if i > 1 {
			candidate = base + "-" + strconv.Itoa(i)
		}
		taken, err := s.store.SlugTaken(ctx, sql.NullString{String: candidate, Valid: true})
		if err != nil {
			return "", fmt.Errorf("check slug %q: %w", candidate, err)
		}
		if !taken {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("no free slug for %q", base)
}

// Unpublish takes a post off the site, keeping its slug and date for a
// later republish. Unpublishing a draft changes nothing, and changed
// reports false.
func (s *Service) Unpublish(ctx context.Context, id, userID string) (p Post, changed bool, err error) {
	p, err = s.Get(ctx, id)
	if err != nil {
		return Post{}, false, err
	}
	if p.Status != "published" {
		return p, false, nil
	}
	n, err := s.store.UnpublishPost(ctx, db.UnpublishPostParams{ID: id, Now: s.timestamp(), UserID: userID})
	if err != nil {
		return Post{}, false, fmt.Errorf("unpublish post %s: %w", id, err)
	}
	if n == 0 {
		return Post{}, false, ErrNotFound
	}
	p, err = s.Get(ctx, id)
	return p, err == nil, err
}

// Delete hides a post; Restore undoes it. It returns the post as it was.
func (s *Service) Delete(ctx context.Context, id, userID string) (Post, error) {
	p, err := s.Get(ctx, id)
	if err != nil {
		return Post{}, err
	}
	n, err := s.store.SoftDeletePost(ctx, db.SoftDeletePostParams{ID: id, Now: s.timestamp(), UserID: userID})
	if err != nil {
		return Post{}, fmt.Errorf("delete post %s: %w", id, err)
	}
	if n == 0 {
		return Post{}, ErrNotFound
	}
	return p, nil
}

// Restore brings back a deleted post.
func (s *Service) Restore(ctx context.Context, id, userID string) (Post, error) {
	n, err := s.store.RestorePost(ctx, db.RestorePostParams{ID: id, Now: s.timestamp(), UserID: userID})
	if err != nil {
		return Post{}, fmt.Errorf("restore post %s: %w", id, err)
	}
	if n == 0 {
		return Post{}, ErrNotFound
	}
	return s.Get(ctx, id)
}

// Published returns live published posts, newest first.
func (s *Service) Published(ctx context.Context) ([]Post, error) {
	rows, err := s.store.ListPublishedPosts(ctx)
	if err != nil {
		return nil, fmt.Errorf("list published posts: %w", err)
	}
	out := make([]Post, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(r))
	}
	return out, nil
}

func (s *Service) timestamp() string { return formatTime(s.now()) }

// etag and parseETag turn a post's version into an HTTP entity tag and back.
func etag(p Post) string { return `"` + p.UpdatedAt + `"` }

func parseETag(h string) string {
	return strings.Trim(strings.TrimPrefix(strings.TrimSpace(h), "W/"), `"`)
}
