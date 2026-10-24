package posts

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ras251093/hgn-bowls/api/db"
)

type fakeSite struct {
	calls int
	err   error
}

func (f *fakeSite) Rebuild(ctx context.Context) error {
	f.calls++
	return f.err
}

type fakeImages struct{ err error }

func (f fakeImages) SignedReadURL(ctx context.Context, key string) (string, error) {
	return "https://storage.example/" + key + "?sig", f.err
}

// failingStore is a real store whose reads can be made to fail.
type failingStore struct {
	Store
	err error
}

func (f failingStore) ListPosts(ctx context.Context) ([]db.Post, error) { return nil, f.err }

type env struct {
	t      *testing.T
	srv    http.Handler
	site   *fakeSite
	images *fakeImages
	store  Store
}

// newEnv wires a Handler to a fresh database, a clock that ticks a second
// per call and IDs p1, p2, ….
func newEnv(t *testing.T) *env {
	t.Helper()
	conn, err := db.Open("file:"+filepath.Join(t.TempDir(), "posts.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if err := db.Migrate(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	e := &env{t: t, site: &fakeSite{}, images: &fakeImages{}, store: db.New(conn)}
	e.build(e.store)
	return e
}

func (e *env) build(store Store) {
	clock := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	ids := 0
	svc := NewService(store,
		WithClock(func() time.Time { clock = clock.Add(time.Second); return clock }),
		WithIDs(func() string { ids++; return fmt.Sprintf("p%d", ids) }),
	)
	h := NewHandler(svc, e.site, e.images, func(*http.Request) string { return "auth0|alice" }, slog.New(slog.DiscardHandler))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/posts", h.Published)
	mux.HandleFunc("GET /api/admin/posts", h.List)
	mux.HandleFunc("POST /api/admin/posts", h.Create)
	mux.HandleFunc("GET /api/admin/posts/{id}", h.Get)
	mux.HandleFunc("PUT /api/admin/posts/{id}", h.Update)
	mux.HandleFunc("DELETE /api/admin/posts/{id}", h.Delete)
	mux.HandleFunc("POST /api/admin/posts/{id}/publish", h.Publish)
	mux.HandleFunc("POST /api/admin/posts/{id}/unpublish", h.Unpublish)
	mux.HandleFunc("POST /api/admin/posts/{id}/restore", h.Restore)
	e.srv = mux
}

type reply struct {
	status int
	header http.Header
	body   string
}

func (e *env) do(method, path, body string, headers ...string) reply {
	e.t.Helper()
	req := httptest.NewRequestWithContext(e.t.Context(), method, path, strings.NewReader(body))
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	rec := httptest.NewRecorder()
	e.srv.ServeHTTP(rec, req)
	b, _ := io.ReadAll(rec.Body)
	return reply{rec.Code, rec.Header(), string(b)}
}

func decodeAs[T any](t *testing.T, r reply) T {
	t.Helper()
	var v T
	if err := json.Unmarshal([]byte(r.body), &v); err != nil {
		t.Fatalf("decode %q: %v", r.body, err)
	}
	return v
}

func (e *env) create(title string) Post {
	e.t.Helper()
	r := e.do("POST", "/api/admin/posts", `{"title":"`+title+`","bodyMd":"Hello"}`)
	if r.status != http.StatusCreated {
		e.t.Fatalf("create: %d %s", r.status, r.body)
	}
	return decodeAs[Post](e.t, r)
}

func TestCreateAndGet(t *testing.T) {
	e := newEnv(t)
	r := e.do("POST", "/api/admin/posts", `{"title":"  Open day ","description":"Come along","bodyMd":"**Hi**"}`)
	if r.status != http.StatusCreated {
		t.Fatalf("status = %d, body %s", r.status, r.body)
	}
	want := Post{
		ID: "p1", Title: "Open day", Description: "Come along", BodyMd: "**Hi**", Status: "draft",
		CreatedAt: "2026-04-01T09:00:01.000000Z", UpdatedAt: "2026-04-01T09:00:01.000000Z",
		CreatedBy: "auth0|alice", UpdatedBy: "auth0|alice",
	}
	if diff := cmp.Diff(want, decodeAs[Post](t, r)); diff != "" {
		t.Errorf("created (-want +got):\n%s", diff)
	}
	if got := r.header.Get("Location"); got != "/api/admin/posts/p1" {
		t.Errorf("Location = %q", got)
	}
	if got := r.header.Get("ETag"); got != `"2026-04-01T09:00:01.000000Z"` {
		t.Errorf("ETag = %q", got)
	}

	g := e.do("GET", "/api/admin/posts/p1", "")
	if g.status != 200 || g.header.Get("ETag") != r.header.Get("ETag") {
		t.Errorf("get: %d ETag %q", g.status, g.header.Get("ETag"))
	}
	if r := e.do("GET", "/api/admin/posts/nope", ""); r.status != http.StatusNotFound {
		t.Errorf("get missing: %d, want 404", r.status)
	}
	if e.site.calls != 0 {
		t.Errorf("creating a draft started %d rebuilds", e.site.calls)
	}
}

func TestCreateRejectsBadInput(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name string
		body string
		want int
	}{
		{"not json", `{`, 400},
		{"unknown field", `{"title":"t","status":"published"}`, 400},
		{"invalid post", `{"title":""}`, 422},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := e.do("POST", "/api/admin/posts", tt.body)
			if r.status != tt.want {
				t.Errorf("status = %d, want %d (%s)", r.status, tt.want, r.body)
			}
		})
	}
	r := e.do("POST", "/api/admin/posts", `{"title":""}`)
	got := decodeAs[struct{ Fields map[string]string }](t, r)
	if _, ok := got.Fields["title"]; !ok {
		t.Errorf("422 body %s does not name the title field", r.body)
	}
}

func TestUpdate(t *testing.T) {
	e := newEnv(t)
	p := e.create("First")
	body := `{"title":"Second","bodyMd":"b","coverImageKey":"posts/2026/04/x.jpg","coverAlt":"A green"}`

	if r := e.do("PUT", "/api/admin/posts/p1", body); r.status != http.StatusPreconditionRequired {
		t.Errorf("no If-Match: %d, want 428", r.status)
	}

	r := e.do("PUT", "/api/admin/posts/p1", body, "If-Match", etag(p))
	if r.status != 200 {
		t.Fatalf("update: %d %s", r.status, r.body)
	}
	updated := decodeAs[Post](t, r)
	if updated.Title != "Second" || *updated.CoverAlt != "A green" || updated.UpdatedAt == p.UpdatedAt {
		t.Errorf("updated = %+v", updated)
	}
	if r.header.Get("ETag") != etag(updated) {
		t.Errorf("ETag %q does not match the new version", r.header.Get("ETag"))
	}

	// A second editor still holding the old version loses.
	if r := e.do("PUT", "/api/admin/posts/p1", `{"title":"Third"}`, "If-Match", etag(p)); r.status != http.StatusConflict {
		t.Errorf("stale save: %d, want 409", r.status)
	}
	if r := e.do("PUT", "/api/admin/posts/nope", `{"title":"Third"}`, "If-Match", etag(p)); r.status != http.StatusNotFound {
		t.Errorf("missing post: %d, want 404", r.status)
	}
	if r := e.do("PUT", "/api/admin/posts/p1", `{"title":""}`, "If-Match", etag(updated)); r.status != 422 {
		t.Errorf("invalid save: %d, want 422", r.status)
	}
	if e.site.calls != 0 {
		t.Errorf("saving started %d rebuilds", e.site.calls)
	}
}

type change struct {
	Post    *Post
	Rebuild struct{ Status string }
}

func TestPublishLifecycle(t *testing.T) {
	e := newEnv(t)
	e.create("Open Day")

	r := e.do("POST", "/api/admin/posts/p1/publish", "")
	got := decodeAs[change](t, r)
	if r.status != 200 || got.Post.Status != "published" || *got.Post.Slug != "open-day" || got.Post.PublishedAt == nil {
		t.Fatalf("publish: %d %s", r.status, r.body)
	}
	if got.Rebuild.Status != "started" || e.site.calls != 1 {
		t.Errorf("rebuild = %q after %d calls, want started after 1", got.Rebuild.Status, e.site.calls)
	}
	firstPublished := *got.Post.PublishedAt

	// Publishing again changes nothing but still rebuilds, to push edits live.
	again := decodeAs[change](t, e.do("POST", "/api/admin/posts/p1/publish", ""))
	if *again.Post.PublishedAt != firstPublished || again.Post.UpdatedAt != got.Post.UpdatedAt || e.site.calls != 2 {
		t.Errorf("republish changed the post or skipped the rebuild: %+v, %d calls", again.Post, e.site.calls)
	}

	un := decodeAs[change](t, e.do("POST", "/api/admin/posts/p1/unpublish", ""))
	if un.Post.Status != "draft" || un.Rebuild.Status != "started" || e.site.calls != 3 {
		t.Errorf("unpublish: %+v rebuild %q", un.Post, un.Rebuild.Status)
	}
	un2 := decodeAs[change](t, e.do("POST", "/api/admin/posts/p1/unpublish", ""))
	if un2.Rebuild.Status != "skipped" || e.site.calls != 3 {
		t.Errorf("unpublishing a draft: rebuild %q, %d calls", un2.Rebuild.Status, e.site.calls)
	}

	// Republishing keeps the first slug and date even after a retitle.
	cur := decodeAs[Post](t, e.do("GET", "/api/admin/posts/p1", ""))
	e.do("PUT", "/api/admin/posts/p1", `{"title":"Renamed"}`, "If-Match", etag(cur))
	re := decodeAs[change](t, e.do("POST", "/api/admin/posts/p1/publish", ""))
	if *re.Post.Slug != "open-day" || *re.Post.PublishedAt != firstPublished {
		t.Errorf("republish: slug %q date %q", *re.Post.Slug, *re.Post.PublishedAt)
	}

	if r := e.do("POST", "/api/admin/posts/nope/publish", ""); r.status != 404 {
		t.Errorf("publish missing: %d", r.status)
	}
}

func TestPublishSlugClash(t *testing.T) {
	e := newEnv(t)
	for range 3 {
		e.create("Open Day")
	}
	var slugs []string
	for _, id := range []string{"p1", "p2", "p3"} {
		c := decodeAs[change](t, e.do("POST", "/api/admin/posts/"+id+"/publish", ""))
		slugs = append(slugs, *c.Post.Slug)
	}
	if diff := cmp.Diff([]string{"open-day", "open-day-2", "open-day-3"}, slugs); diff != "" {
		t.Errorf("slugs (-want +got):\n%s", diff)
	}
}

func TestRebuildFailureStillSaves(t *testing.T) {
	e := newEnv(t)
	e.site.err = errors.New("cloudflare is down")
	e.create("News")
	r := e.do("POST", "/api/admin/posts/p1/publish", "")
	got := decodeAs[change](t, r)
	if r.status != 200 || got.Post.Status != "published" || got.Rebuild.Status != "failed" {
		t.Errorf("publish with failing rebuild: %d %s", r.status, r.body)
	}
}

func TestDeleteAndRestore(t *testing.T) {
	e := newEnv(t)
	e.create("Draft")
	e.create("Live")
	e.do("POST", "/api/admin/posts/p2/publish", "")
	calls := e.site.calls

	draftDel := decodeAs[change](t, e.do("DELETE", "/api/admin/posts/p1", ""))
	if draftDel.Rebuild.Status != "skipped" || draftDel.Post != nil {
		t.Errorf("deleting a draft: %+v", draftDel)
	}
	liveDel := decodeAs[change](t, e.do("DELETE", "/api/admin/posts/p2", ""))
	if liveDel.Rebuild.Status != "started" || e.site.calls != calls+1 {
		t.Errorf("deleting a live post: rebuild %q", liveDel.Rebuild.Status)
	}
	if r := e.do("GET", "/api/admin/posts/p2", ""); r.status != 404 {
		t.Errorf("get deleted: %d", r.status)
	}
	if r := e.do("DELETE", "/api/admin/posts/p2", ""); r.status != 404 {
		t.Errorf("delete twice: %d", r.status)
	}

	restored := decodeAs[change](t, e.do("POST", "/api/admin/posts/p2/restore", ""))
	if restored.Post == nil || restored.Post.ID != "p2" || restored.Rebuild.Status != "started" {
		t.Errorf("restore live post: %+v", restored)
	}
	if r := e.do("POST", "/api/admin/posts/p2/restore", ""); r.status != 404 {
		t.Errorf("restore a live post: %d, want 404", r.status)
	}
	draftRestore := decodeAs[change](t, e.do("POST", "/api/admin/posts/p1/restore", ""))
	if draftRestore.Rebuild.Status != "skipped" {
		t.Errorf("restoring a draft rebuilt the site")
	}
}

func TestListAndPublished(t *testing.T) {
	e := newEnv(t)
	e.create("Draft")
	e.do("POST", "/api/admin/posts", `{"title":"With cover","coverImageKey":"posts/2026/04/x.jpg","coverAlt":"The green"}`)
	e.do("POST", "/api/admin/posts/p2/publish", "")

	all := decodeAs[struct{ Posts []Post }](t, e.do("GET", "/api/admin/posts", ""))
	if len(all.Posts) != 2 || all.Posts[0].ID != "p2" {
		t.Errorf("admin list: %+v", all.Posts)
	}

	r := e.do("GET", "/api/posts", "")
	pub := decodeAs[struct{ Posts []PublicPost }](t, r)
	want := []PublicPost{{
		ID: "p2", Slug: "with-cover", Title: "With cover",
		Cover:       &Cover{Key: "posts/2026/04/x.jpg", URL: "https://storage.example/posts/2026/04/x.jpg?sig", Alt: "The green"},
		PublishedAt: "2026-04-01T09:00:03.000000Z", UpdatedAt: "2026-04-01T09:00:03.000000Z",
	}}
	if diff := cmp.Diff(want, pub.Posts); diff != "" {
		t.Errorf("published (-want +got):\n%s", diff)
	}

	empty := newEnv(t)
	if r := empty.do("GET", "/api/posts", ""); r.body != "{\"posts\":[]}\n" {
		t.Errorf("no posts: body %q, want an empty list", r.body)
	}
}

func TestPublishedFailsWhenSigningFails(t *testing.T) {
	e := newEnv(t)
	e.do("POST", "/api/admin/posts", `{"title":"t","coverImageKey":"posts/x.jpg","coverAlt":"a"}`)
	e.do("POST", "/api/admin/posts/p1/publish", "")
	e.images.err = errors.New("no signing rights")
	if r := e.do("GET", "/api/posts", ""); r.status != 500 {
		t.Errorf("status = %d, want 500 so the build keeps the current site", r.status)
	}
}

func TestStoreErrorIs500(t *testing.T) {
	e := newEnv(t)
	e.build(failingStore{Store: e.store, err: sql.ErrConnDone})
	r := e.do("GET", "/api/admin/posts", "")
	if r.status != 500 || strings.Contains(r.body, "sql") {
		t.Errorf("store failure: %d %s; want 500 without internals", r.status, r.body)
	}
}
