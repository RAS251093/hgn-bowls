package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ras251093/hgn-bowls/api/db"
)

// Smallest files that sniff as each type.
var (
	jpeg = []byte("\xff\xd8\xff\xe0 a jpeg")
	png  = []byte("\x89PNG\r\n\x1a\n a png")
)

// oldSite serves testdata/posts.json as Hygraph would, with its image
// addresses pointing back at the server.
type oldSite struct {
	*httptest.Server
	mu       sync.Mutex
	requests []*http.Request
	queries  []string
	images   map[string][]byte
}

func newOldSite(t *testing.T) *oldSite {
	t.Helper()
	fixture, err := os.ReadFile(filepath.Join("testdata", "posts.json"))
	if err != nil {
		t.Fatal(err)
	}
	s := &oldSite{images: map[string][]byte{"/green.jpg": jpeg, "/extra.png": png, "/team.jpg": jpeg, "/page.html": []byte("<!doctype html><p>not an image")}}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		s.requests = append(s.requests, r)
		s.mu.Unlock()
		if r.Method == http.MethodPost && r.URL.Path == "/graphql" {
			var body struct{ Query string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			s.mu.Lock()
			s.queries = append(s.queries, body.Query)
			s.mu.Unlock()
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(bytes.ReplaceAll(fixture, []byte("IMAGES"), []byte(s.URL)))
			return
		}
		img, ok := s.images[r.URL.Path]
		if !ok {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(img)
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *oldSite) imageRequests() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for _, r := range s.requests {
		if r.Method == http.MethodGet {
			n++
		}
	}
	return n
}

// memBucket keeps uploaded images in memory.
type memBucket struct {
	objects map[string]object
}

type object struct {
	contentType string
	data        []byte
}

func (b *memBucket) Put(_ context.Context, key, contentType string, data []byte) error {
	if b.objects == nil {
		b.objects = map[string]object{}
	}
	b.objects[key] = object{contentType, data}
	return nil
}

func openDB(t *testing.T) *db.Queries {
	t.Helper()
	conn, err := db.Open("file:"+filepath.Join(t.TempDir(), "test.db"), "")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(t.Context(), conn); err != nil {
		t.Fatal(err)
	}
	return db.New(conn)
}

func newTestImporter(store *db.Queries, bucket *memBucket) *importer {
	n := 0
	return &importer{
		store:  store,
		images: bucket,
		client: http.DefaultClient,
		now:    func() time.Time { return time.Date(2026, 10, 30, 9, 0, 0, 0, time.UTC) },
		newID: func() string {
			n++
			return fmt.Sprintf("ID%d", n)
		},
		log: io.Discard,
	}
}

func TestFetchPostsAsksForEveryPost(t *testing.T) {
	site := newOldSite(t)
	got, err := fetchPosts(t.Context(), http.DefaultClient, site.URL+"/graphql", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].Slug != "opening-day" || got[1].Slug != "winter-work" {
		t.Fatalf("posts = %+v, want opening-day and winter-work", got)
	}
	if got[0].Body.Markdown == "" || len(got[0].Assets) != 2 || got[0].AltText[0] == "" {
		t.Errorf("first post = %+v, want its body, assets and alt text", got[0])
	}
	// Hygraph returns 10 posts unless asked for more.
	if q := site.queries[0]; !strings.Contains(q, "posts(first: 1000") {
		t.Errorf("query %q doesn't ask for every post", q)
	}
	if h := site.requests[0].Header.Get("Authorization"); h != "" {
		t.Errorf("sent Authorization %q without a token", h)
	}
}

func TestFetchPostsSendsTheToken(t *testing.T) {
	site := newOldSite(t)
	if _, err := fetchPosts(t.Context(), http.DefaultClient, site.URL+"/graphql", "secret"); err != nil {
		t.Fatal(err)
	}
	if h := site.requests[0].Header.Get("Authorization"); h != "Bearer secret" {
		t.Errorf("Authorization = %q, want Bearer secret", h)
	}
}

func TestFetchPostsReportsFailures(t *testing.T) {
	tests := map[string]http.HandlerFunc{
		"http error": func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, "nope", http.StatusUnauthorized)
		},
		"graphql error": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = io.WriteString(w, `{"data":null,"errors":[{"message":"field 'pubDate' not found"}]}`)
		},
	}
	for name, h := range tests {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(h)
			defer srv.Close()
			_, err := fetchPosts(t.Context(), http.DefaultClient, srv.URL, "")
			if err == nil {
				t.Fatal("expected an error")
			}
			if name == "graphql error" && !strings.Contains(err.Error(), "pubDate") {
				t.Errorf("error %q doesn't pass on Hygraph's message", err)
			}
		})
	}
}

func TestImportKeepsSlugsDatesAndCovers(t *testing.T) {
	site := newOldSite(t)
	store, bucket := openDB(t), &memBucket{}
	posts, err := fetchPosts(t.Context(), http.DefaultClient, site.URL+"/graphql", "")
	if err != nil {
		t.Fatal(err)
	}

	rep, err := newTestImporter(store, bucket).run(t.Context(), posts)
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(rep.imported, []string{"opening-day", "winter-work"}) {
		t.Errorf("imported %v, want both posts", rep.imported)
	}

	got, err := store.ListPublishedPosts(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d published posts, want 2", len(got))
	}
	winter, opening := got[0], got[1]

	if opening.Slug.String != "opening-day" || opening.Title != "Opening day" ||
		opening.Description != "The green opens for the season." || opening.Status != "published" {
		t.Errorf("opening day = %+v", opening)
	}
	// A date on its own means that day in the UK: midnight BST is 23:00 UTC the day before.
	if opening.PublishedAt.String != "2025-04-04T23:00:00.000000Z" {
		t.Errorf("opening day published %q, want midnight UK time on 5 April", opening.PublishedAt.String)
	}
	if winter.PublishedAt.String != "2025-11-14T18:30:00.000000Z" {
		t.Errorf("winter work published %q, want the original time", winter.PublishedAt.String)
	}
	if opening.CreatedAt != "2026-10-30T09:00:00.000000Z" || opening.CreatedBy != "import:hygraph" {
		t.Errorf("created %q by %q, want now by import:hygraph", opening.CreatedAt, opening.CreatedBy)
	}
	if !strings.Contains(opening.BodyMd, "<iframe") {
		t.Error("body lost its video; the site turns the iframe into a player")
	}

	if opening.CoverImageKey.String != "posts/ID1.jpg" || opening.CoverAlt.String != "Bowlers on the green in the sunshine" {
		t.Errorf("cover = %q (%q), want posts/ID1.jpg with its alt text", opening.CoverImageKey.String, opening.CoverAlt.String)
	}
	if obj := bucket.objects["posts/ID1.jpg"]; obj.contentType != "image/jpeg" || !bytes.Equal(obj.data, jpeg) {
		t.Errorf("uploaded cover = %+v, want the jpeg", obj)
	}
	if winter.CoverImageKey.Valid || winter.CoverAlt.Valid {
		t.Errorf("winter work has cover %+v; it had no image, so it gets the club photo", winter.CoverImageKey)
	}
	if len(bucket.objects) != 1 {
		t.Errorf("uploaded %d images, want only the cover", len(bucket.objects))
	}
}

func TestImportWarnsAboutWhatItLeavesBehind(t *testing.T) {
	site := newOldSite(t)
	posts, err := fetchPosts(t.Context(), http.DefaultClient, site.URL+"/graphql", "")
	if err != nil {
		t.Fatal(err)
	}
	rep, err := newTestImporter(openDB(t), &memBucket{}).run(t.Context(), posts)
	if err != nil {
		t.Fatal(err)
	}
	all := strings.Join(rep.warnings, "\n")
	for _, want := range []string{
		"opening-day: 1 more image the old site never showed",
		"opening-day: body links to an image on the old site: " + site.URL + "/team.jpg",
	} {
		if !strings.Contains(all, want) {
			t.Errorf("warnings %q\nmissing %q", all, want)
		}
	}
}

func TestImportSkipsPostsAlreadyThere(t *testing.T) {
	site := newOldSite(t)
	store, bucket := openDB(t), &memBucket{}
	posts, err := fetchPosts(t.Context(), http.DefaultClient, site.URL+"/graphql", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := newTestImporter(store, bucket).run(t.Context(), posts); err != nil {
		t.Fatal(err)
	}
	downloads := site.imageRequests()

	rep, err := newTestImporter(store, bucket).run(t.Context(), posts)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.imported) != 0 || !slices.Equal(rep.skipped, []string{"opening-day", "winter-work"}) {
		t.Errorf("second run imported %v, skipped %v; want both skipped", rep.imported, rep.skipped)
	}
	if site.imageRequests() != downloads {
		t.Error("second run downloaded images for posts it skipped")
	}
}

func TestDryRunWritesNothing(t *testing.T) {
	site := newOldSite(t)
	store, bucket := openDB(t), &memBucket{}
	posts, err := fetchPosts(t.Context(), http.DefaultClient, site.URL+"/graphql", "")
	if err != nil {
		t.Fatal(err)
	}
	im := newTestImporter(store, bucket)
	im.dryRun = true
	var log bytes.Buffer
	im.log = &log

	rep, err := im.run(t.Context(), posts)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.imported) != 2 {
		t.Errorf("dry run would import %v, want both posts", rep.imported)
	}
	if got, _ := store.ListPosts(t.Context()); len(got) != 0 {
		t.Errorf("dry run saved %d posts", len(got))
	}
	if len(bucket.objects) != 0 {
		t.Errorf("dry run uploaded %d images", len(bucket.objects))
	}
	if !strings.Contains(log.String(), "would import opening-day") {
		t.Errorf("log %q doesn't say what it would do", log.String())
	}
}

func TestImportStopsOnABadPost(t *testing.T) {
	tests := map[string]func(*hygraphPost){
		"slug not usable in an address": func(p *hygraphPost) { p.Slug = "Opening Day!" },
		"no title":                      func(p *hygraphPost) { p.Title = " " },
		"unreadable date":               func(p *hygraphPost) { p.PubDate = "5th April" },
		"cover missing":                 func(p *hygraphPost) { p.Assets[0].URL += "-gone" },
		"cover not an image":            func(p *hygraphPost) { p.Assets[0].URL = strings.Replace(p.Assets[0].URL, "green.jpg", "page.html", 1) },
	}
	for name, spoil := range tests {
		t.Run(name, func(t *testing.T) {
			site := newOldSite(t)
			store := openDB(t)
			posts, err := fetchPosts(t.Context(), http.DefaultClient, site.URL+"/graphql", "")
			if err != nil {
				t.Fatal(err)
			}
			spoil(&posts[0])
			if _, err := newTestImporter(store, &memBucket{}).run(t.Context(), posts); err == nil {
				t.Fatal("expected an error")
			}
			if taken, _ := store.SlugTaken(t.Context(), sql.NullString{String: "opening-day", Valid: true}); taken {
				t.Error("saved the bad post")
			}
		})
	}
}
