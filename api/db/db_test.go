package db

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// openTest returns a migrated database in a fresh temporary file.
func openTest(t *testing.T) *sql.DB {
	t.Helper()
	conn, err := Open("file:"+filepath.Join(t.TempDir(), "test.db"), "")
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return conn
}

func TestMigrateIsRepeatable(t *testing.T) {
	conn := openTest(t)
	if err := Migrate(t.Context(), conn); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
}

func TestOpenRejectsRemoteWithoutToken(t *testing.T) {
	if _, err := Open("libsql://club.turso.io", ""); err == nil {
		t.Fatal("expected an error for a remote database without a token")
	}
}

func TestSnapshots(t *testing.T) {
	s := NewSnapshots(New(openTest(t)))
	ctx := t.Context()

	data, at, err := s.LoadEventsSnapshot(ctx)
	if err != nil || data != nil || !at.IsZero() {
		t.Fatalf("empty store: got %q, %v, %v; want nil, zero, nil", data, at, err)
	}

	first := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	second := first.Add(time.Hour)
	for _, step := range []struct {
		data string
		at   time.Time
	}{{`[{"id":"a"}]`, first}, {`[{"id":"b"}]`, second}} {
		if err := s.SaveEventsSnapshot(ctx, []byte(step.data), step.at); err != nil {
			t.Fatalf("save: %v", err)
		}
		gotData, gotAt, err := s.LoadEventsSnapshot(ctx)
		if err != nil {
			t.Fatalf("load: %v", err)
		}
		if string(gotData) != step.data || !gotAt.Equal(step.at) {
			t.Errorf("load = %s at %v, want %s at %v", gotData, gotAt, step.data, step.at)
		}
	}
}

func addPost(t *testing.T, q *Queries, id, now string) {
	t.Helper()
	err := q.CreatePost(t.Context(), CreatePostParams{
		ID: id, Title: "Title " + id, Description: "desc", BodyMd: "body", Now: now, UserID: "auth0|alice",
	})
	if err != nil {
		t.Fatalf("create %s: %v", id, err)
	}
}

func TestPostsLifecycle(t *testing.T) {
	q := New(openTest(t))
	ctx := t.Context()
	addPost(t, q, "p1", "2026-04-01T09:00:00Z")

	got, err := q.GetPost(ctx, "p1")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Status != "draft" || got.Slug.Valid || got.PublishedAt.Valid {
		t.Errorf("new post = %+v, want a draft with no slug or publish date", got)
	}

	// Update succeeds against the current version and bumps it.
	n, err := q.UpdatePost(ctx, UpdatePostParams{
		ID: "p1", ExpectedUpdatedAt: "2026-04-01T09:00:00Z", Now: "2026-04-01T10:00:00Z", UserID: "auth0|bob",
		Title: "New title", Description: "d", BodyMd: "b",
	})
	if err != nil || n != 1 {
		t.Fatalf("update: n=%d err=%v, want 1 row", n, err)
	}
	// Update against the old version changes nothing.
	n, err = q.UpdatePost(ctx, UpdatePostParams{
		ID: "p1", ExpectedUpdatedAt: "2026-04-01T09:00:00Z", Now: "2026-04-01T11:00:00Z", UserID: "auth0|bob", Title: "Lost",
	})
	if err != nil || n != 0 {
		t.Fatalf("stale update: n=%d err=%v, want 0 rows", n, err)
	}

	// Publishing sets the slug and date once; republishing keeps them.
	pub := func(slug, now string) {
		t.Helper()
		if n, err := q.PublishPost(ctx, PublishPostParams{ID: "p1", Slug: sql.NullString{String: slug, Valid: true}, Now: now, UserID: "auth0|bob"}); err != nil || n != 1 {
			t.Fatalf("publish: n=%d err=%v", n, err)
		}
	}
	pub("new-title", "2026-04-02T09:00:00Z")
	if _, err := q.UnpublishPost(ctx, UnpublishPostParams{ID: "p1", Now: "2026-04-03T09:00:00Z", UserID: "auth0|bob"}); err != nil {
		t.Fatalf("unpublish: %v", err)
	}
	pub("ignored-second-slug", "2026-04-04T09:00:00Z")

	got, _ = q.GetPost(ctx, "p1")
	if got.Slug.String != "new-title" || got.PublishedAt.String != "2026-04-02T09:00:00Z" || got.Status != "published" {
		t.Errorf("after republish: slug=%q published_at=%q status=%q; want the first slug and date kept",
			got.Slug.String, got.PublishedAt.String, got.Status)
	}

	published, err := q.ListPublishedPosts(ctx)
	if err != nil || len(published) != 1 {
		t.Fatalf("published list: %d posts, err %v; want 1", len(published), err)
	}

	// Soft delete hides the post everywhere but keeps its slug taken.
	if n, err := q.SoftDeletePost(ctx, SoftDeletePostParams{ID: "p1", Now: "2026-04-05T09:00:00Z", UserID: "auth0|bob"}); err != nil || n != 1 {
		t.Fatalf("delete: n=%d err=%v", n, err)
	}
	if _, err := q.GetPost(ctx, "p1"); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("get deleted: err = %v, want sql.ErrNoRows", err)
	}
	if all, _ := q.ListPosts(ctx); len(all) != 0 {
		t.Errorf("list after delete: %d posts, want 0", len(all))
	}
	if pubs, _ := q.ListPublishedPosts(ctx); len(pubs) != 0 {
		t.Errorf("published after delete: %d posts, want 0", len(pubs))
	}
	if taken, err := q.SlugTaken(ctx, sql.NullString{String: "new-title", Valid: true}); err != nil || !taken {
		t.Errorf("slug of deleted post: taken=%v err=%v, want true", taken, err)
	}

	// Restore brings it back; deleting twice or restoring a live post does nothing.
	if n, _ := q.RestorePost(ctx, RestorePostParams{ID: "p1", Now: "2026-04-06T09:00:00Z", UserID: "auth0|bob"}); n != 1 {
		t.Fatalf("restore: %d rows, want 1", n)
	}
	if n, _ := q.RestorePost(ctx, RestorePostParams{ID: "p1", Now: "2026-04-06T10:00:00Z", UserID: "auth0|bob"}); n != 0 {
		t.Errorf("restore live post: %d rows, want 0", n)
	}
	if _, err := q.GetPost(ctx, "p1"); err != nil {
		t.Errorf("get restored: %v", err)
	}
}

func TestListPublishedNewestFirst(t *testing.T) {
	q := New(openTest(t))
	ctx := context.Background()
	for i, id := range []string{"old", "new"} {
		addPost(t, q, id, "2026-04-01T09:00:00Z")
		now := []string{"2026-04-02T09:00:00Z", "2026-04-09T09:00:00Z"}[i]
		if _, err := q.PublishPost(ctx, PublishPostParams{ID: id, Slug: sql.NullString{String: id, Valid: true}, Now: now, UserID: "u"}); err != nil {
			t.Fatal(err)
		}
	}
	got, err := q.ListPublishedPosts(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].ID != "new" || got[1].ID != "old" {
		t.Errorf("order = %v, want new then old", []string{got[0].ID, got[1].ID})
	}
}

// A rejected token must surface Turso's answer, not goose's "connection is
// already closed" after its retries.
func TestMigrateReportsUnreachableDatabase(t *testing.T) {
	turso := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = io.WriteString(w, `{"error":"Unauthorized: invalid token"}`)
	}))
	t.Cleanup(turso.Close)

	conn, err := Open(turso.URL, "bad-token")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })

	err = Migrate(t.Context(), conn)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "reaching database") || !strings.Contains(err.Error(), "invalid token") {
		t.Errorf("err = %v, want it to name the database and Turso's reason", err)
	}
}
