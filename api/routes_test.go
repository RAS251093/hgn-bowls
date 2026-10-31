package main

import (
	"net/http"
	"net/http/httptest"
	"slices"
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

type fakeAdmins struct{}

func (fakeAdmins) List(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("admins.List")) }
func (fakeAdmins) Invite(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("admins.Invite"))
}
func (fakeAdmins) Resend(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("admins.Resend " + r.PathValue("id")))
}
func (fakeAdmins) Remove(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("admins.Remove " + r.PathValue("id")))
}
func (fakeAdmins) Restore(w http.ResponseWriter, r *http.Request) {
	_, _ = w.Write([]byte("admins.Restore " + r.PathValue("id")))
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

// permission stands in for Verifier.RequirePermission: X-Permissions lists
// the caller's permissions. None at all gets 401, the wrong ones 403.
func permission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			have := r.Header.Get("X-Permissions")
			if have == "" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			if !slices.Contains(strings.Split(have, " "), perm) {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

// Credentials the route tests send: the build token, or an Auth0 token
// carrying the named permissions.
const (
	buildToken = "build"
	postsPerm  = "manage:posts"
	adminsPerm = "manage:admins"
	bothPerms  = "manage:posts manage:admins"
)

func withCredentials(r *http.Request, as string) {
	switch as {
	case "":
	case buildToken:
		r.Header.Set("X-Build", "yes")
	default:
		r.Header.Set("X-Permissions", as)
	}
}

func testRoutes() http.Handler {
	return newHandler(routes{
		events:       named("events"),
		posts:        fakePosts{},
		uploads:      named("uploads"),
		latestDeploy: named("deploys.Latest"),
		rebuild:      named("rebuild"),
		admins:       fakeAdmins{},
		admin:        permission("manage:posts"),
		manageAdmins: permission("manage:admins"),
		build:        gate("X-Build"),
	})
}

func TestRoutes(t *testing.T) {
	h := testRoutes()
	tests := []struct {
		method, path string
		as           string // credentials sent, if any
		want         int
		wantBody     string
	}{
		{"GET", "/health", "", 200, ""},
		{"POST", "/health", "", http.StatusMethodNotAllowed, ""},
		{"GET", "/healthz", "", 404, ""},
		{"GET", "/nope", "", 404, ""},
		{"GET", "/api/events", "", 200, "events"},

		{"GET", "/api/posts", buildToken, 200, "posts.Published"},
		{"GET", "/api/admin/posts", postsPerm, 200, "posts.List"},
		{"POST", "/api/admin/posts", postsPerm, 200, "posts.Create"},
		{"GET", "/api/admin/posts/p1", postsPerm, 200, "posts.Get p1"},
		{"PUT", "/api/admin/posts/p1", postsPerm, 200, "posts.Update p1"},
		{"DELETE", "/api/admin/posts/p1", postsPerm, 200, "posts.Delete p1"},
		{"POST", "/api/admin/posts/p1/publish", postsPerm, 200, "posts.Publish p1"},
		{"POST", "/api/admin/posts/p1/unpublish", postsPerm, 200, "posts.Unpublish p1"},
		{"POST", "/api/admin/posts/p1/restore", postsPerm, 200, "posts.Restore p1"},
		{"POST", "/api/admin/uploads", postsPerm, 200, "uploads"},
		{"GET", "/api/admin/deploys/latest", postsPerm, 200, "deploys.Latest"},
		{"POST", "/api/admin/rebuild", postsPerm, 200, "rebuild"},

		{"GET", "/api/admin/admins", adminsPerm, 200, "admins.List"},
		{"POST", "/api/admin/admins", adminsPerm, 200, "admins.Invite"},
		{"POST", "/api/admin/admins/auth0%7C1/invite", adminsPerm, 200, "admins.Resend auth0|1"},
		{"DELETE", "/api/admin/admins/auth0%7C1", adminsPerm, 200, "admins.Remove auth0|1"},
		{"PUT", "/api/admin/admins/auth0%7C1", adminsPerm, 200, "admins.Restore auth0|1"},
		{"GET", "/api/admin/admins", bothPerms, 200, "admins.List"},
		{"GET", "/api/admin/posts", bothPerms, 200, "posts.List"},

		// Managing posts doesn't let you manage admins, nor the other way round.
		{"GET", "/api/admin/admins", postsPerm, 403, ""},
		{"POST", "/api/admin/admins", postsPerm, 403, ""},
		{"POST", "/api/admin/admins/auth0%7C1/invite", postsPerm, 403, ""},
		{"DELETE", "/api/admin/admins/auth0%7C1", postsPerm, 403, ""},
		{"PUT", "/api/admin/admins/auth0%7C1", postsPerm, 403, ""},
		{"GET", "/api/admin/posts", adminsPerm, 403, ""},
		{"POST", "/api/admin/posts/p1/publish", adminsPerm, 403, ""},
		{"POST", "/api/admin/uploads", adminsPerm, 403, ""},
		{"POST", "/api/admin/rebuild", adminsPerm, 403, ""},

		// The build token must not open the admin area, nor an admin token the build list.
		{"GET", "/api/admin/posts", buildToken, 401, ""},
		{"GET", "/api/admin/admins", buildToken, 401, ""},
		{"GET", "/api/posts", postsPerm, 401, ""},
	}
	for _, tt := range tests {
		t.Run(tt.method+" "+tt.path+" as "+tt.as, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil)
			withCredentials(req, tt.as)
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
		{"GET", "/api/admin/admins"},
		{"POST", "/api/admin/admins"},
		{"POST", "/api/admin/admins/a1/invite"},
		{"DELETE", "/api/admin/admins/a1"},
		{"PUT", "/api/admin/admins/a1"},
		{"GET", "/api/admin/anything-new"},
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), r.method, r.path, nil))
		if rec.Code != http.StatusUnauthorized || strings.Contains(rec.Body.String(), ".") {
			t.Errorf("%s %s without credentials: %d %q, want 401", r.method, r.path, rec.Code, rec.Body)
		}
	}
}
