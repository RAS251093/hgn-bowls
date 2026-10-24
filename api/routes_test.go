package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// named answers 200 with its own name, so tests can see which handler ran.
type named string

func (n named) ServeHTTP(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte(n)) }

type fakePosts struct{}

func (fakePosts) List(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("posts.List")) }
func (fakePosts) Create(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("posts.Create"))
}
func (fakePosts) Get(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("posts.Get " + r.PathValue("id")))
}
func (fakePosts) Update(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("posts.Update " + r.PathValue("id")))
}
func (fakePosts) Delete(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("posts.Delete " + r.PathValue("id")))
}
func (fakePosts) Publish(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("posts.Publish " + r.PathValue("id")))
}
func (fakePosts) Unpublish(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("posts.Unpublish " + r.PathValue("id")))
}
func (fakePosts) Restore(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("posts.Restore " + r.PathValue("id")))
}
func (fakePosts) Published(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("posts.Published"))
}

// gate lets a request through only with the given header, standing in for
// the real token checks.
func gate(header string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(header) == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func testRoutes() http.Handler {
	return newHandler(routes{
		events:       named("events"),
		posts:        fakePosts{},
		uploads:      named("uploads"),
		latestDeploy: named("deploys.Latest"),
		rebuild:      named("rebuild"),
		admin:        gate("X-Admin"),
		build:        gate("X-Build"),
	})
}

func TestRoutes(t *testing.T) {
	h := testRoutes()
	tests := []struct {
		method, path string
		as           string // header that passes the gate, if any
		want         int
		wantBody     string
	}{
		{"GET", "/health", "", 200, ""},
		{"POST", "/health", "", http.StatusMethodNotAllowed, ""},
		{"GET", "/healthz", "", 404, ""},
		{"GET", "/nope", "", 404, ""},
		{"GET", "/api/events", "", 200, "events"},

		{"GET", "/api/posts", "X-Build", 200, "posts.Published"},
		{"GET", "/api/admin/posts", "X-Admin", 200, "posts.List"},
		{"POST", "/api/admin/posts", "X-Admin", 200, "posts.Create"},
		{"GET", "/api/admin/posts/p1", "X-Admin", 200, "posts.Get p1"},
		{"PUT", "/api/admin/posts/p1", "X-Admin", 200, "posts.Update p1"},
		{"DELETE", "/api/admin/posts/p1", "X-Admin", 200, "posts.Delete p1"},
		{"POST", "/api/admin/posts/p1/publish", "X-Admin", 200, "posts.Publish p1"},
		{"POST", "/api/admin/posts/p1/unpublish", "X-Admin", 200, "posts.Unpublish p1"},
		{"POST", "/api/admin/posts/p1/restore", "X-Admin", 200, "posts.Restore p1"},
		{"POST", "/api/admin/uploads", "X-Admin", 200, "uploads"},
		{"GET", "/api/admin/deploys/latest", "X-Admin", 200, "deploys.Latest"},
		{"POST", "/api/admin/rebuild", "X-Admin", 200, "rebuild"},

		// The build token must not open the admin area, nor an admin token the build list.
		{"GET", "/api/admin/posts", "X-Build", 401, ""},
		{"GET", "/api/posts", "X-Admin", 401, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path+" as "+tt.as, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			if tt.as != "" {
				req.Header.Set(tt.as, "yes")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Fatalf("status = %d, want %d", rec.Code, tt.want)
			}
			if tt.want == http.StatusOK && rec.Body.String() != tt.wantBody {
				t.Errorf("body = %q, want %q", rec.Body, tt.wantBody)
			}
		})
	}
}

// Every route under /api/admin/ and the build list must refuse a request
// with no credentials, so a new route can't be added unprotected.
func TestProtectedRoutesNeedCredentials(t *testing.T) {
	h := testRoutes()
	for _, r := range []struct{ method, path string }{
		{"GET", "/api/posts"},
		{"GET", "/api/admin/posts"},
		{"POST", "/api/admin/posts"},
		{"GET", "/api/admin/posts/p1"},
		{"PUT", "/api/admin/posts/p1"},
		{"DELETE", "/api/admin/posts/p1"},
		{"POST", "/api/admin/posts/p1/publish"},
		{"POST", "/api/admin/posts/p1/unpublish"},
		{"POST", "/api/admin/posts/p1/restore"},
		{"POST", "/api/admin/uploads"},
		{"GET", "/api/admin/deploys/latest"},
		{"POST", "/api/admin/rebuild"},
		{"GET", "/api/admin/anything-new"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), r.method, r.path, nil))
		if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), "posts.") {
			t.Errorf("%s %s without credentials: %d %q, want 401", r.method, r.path, rec.Code, rec.Body)
		}
	}
}
