package admins

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
	"unicode"

	"github.com/google/go-cmp/cmp"
)

// fakeAuth0 fakes the tenant: its token endpoint, plus whatever Management
// and Authentication API routes a test adds to mux.
type fakeAuth0 struct {
	t   *testing.T
	mux *http.ServeMux
	srv *httptest.Server

	mu         sync.Mutex
	tokenCalls int
	tokenBody  map[string]string
	expiresIn  int
	tokenSeq   int
}

func newFakeAuth0(t *testing.T) *fakeAuth0 {
	t.Helper()
	f := &fakeAuth0{t: t, mux: http.NewServeMux(), expiresIn: 86400}
	f.mux.HandleFunc("POST /oauth/token", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		f.tokenCalls++
		f.tokenSeq++
		f.tokenBody = decodeBody[map[string]string](t, r)
		if f.tokenBody["client_secret"] != "mgmt-secret" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = io.WriteString(w, `{"error":"access_denied","error_description":"Unauthorized"}`)
			return
		}
		writeTestJSON(w, 200, map[string]any{
			"access_token": "tok" + strconv.Itoa(f.tokenSeq),
			"token_type":   "Bearer",
			"expires_in":   f.expiresIn,
		})
	})
	f.srv = httptest.NewServer(f.mux)
	t.Cleanup(f.srv.Close)
	return f
}

// client returns an Auth0 client for the fake, with a clock the test moves.
func (f *fakeAuth0) client(now *time.Time) *Auth0 {
	return NewAuth0(f.srv.Client(), Settings{
		Domain:       "club.uk.auth0.com",
		ClientID:     "mgmt-id",
		ClientSecret: "mgmt-secret",
		RoleID:       "rol_admin",
		AppClientID:  "spa-id",
	}, WithBaseURL(f.srv.URL), WithClock(func() time.Time { return *now }))
}

// handle adds a Management API route that insists on a bearer token.
func (f *fakeAuth0) handle(pattern string, h http.HandlerFunc) {
	f.mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer tok") {
			f.t.Errorf("%s %s: Authorization = %q", r.Method, r.URL.Path, r.Header.Get("Authorization"))
		}
		h(w, r)
	})
}

func decodeBody[T any](t *testing.T, r *http.Request) T {
	t.Helper()
	var v T
	if err := json.NewDecoder(r.Body).Decode(&v); err != nil {
		t.Errorf("decode %s %s body: %v", r.Method, r.URL.Path, err)
	}
	return v
}

func writeTestJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func auth0Error(w http.ResponseWriter, status int, code, msg string) {
	writeTestJSON(w, status, map[string]any{
		"statusCode": status, "error": http.StatusText(status), "message": msg, "errorCode": code,
	})
}

var start = time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)

func TestTokenIsCachedUntilShortlyBeforeExpiry(t *testing.T) {
	f := newFakeAuth0(t)
	f.expiresIn = 3600
	var gotAuth []string
	f.handle("GET /api/v2/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		writeTestJSON(w, 200, map[string]any{"user_id": r.PathValue("id")})
	})
	now := start
	c := f.client(&now)

	for range 3 {
		if _, err := c.User(t.Context(), "auth0|1"); err != nil {
			t.Fatal(err)
		}
	}
	if f.tokenCalls != 1 {
		t.Errorf("token fetched %d times, want 1", f.tokenCalls)
	}
	want := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     "mgmt-id",
		"client_secret": "mgmt-secret",
		"audience":      "https://club.uk.auth0.com/api/v2/",
	}
	if diff := cmp.Diff(want, f.tokenBody); diff != "" {
		t.Errorf("token request (-want +got):\n%s", diff)
	}

	now = start.Add(55 * time.Minute) // still well inside the hour
	if _, err := c.User(t.Context(), "auth0|1"); err != nil {
		t.Fatal(err)
	}
	if f.tokenCalls != 1 {
		t.Errorf("token fetched %d times before expiry, want 1", f.tokenCalls)
	}

	now = start.Add(59*time.Minute + 30*time.Second) // inside the safety margin
	if _, err := c.User(t.Context(), "auth0|1"); err != nil {
		t.Fatal(err)
	}
	if f.tokenCalls != 2 {
		t.Errorf("token fetched %d times after expiry, want 2", f.tokenCalls)
	}
	if got := gotAuth[len(gotAuth)-1]; got != "Bearer tok2" {
		t.Errorf("after refresh Authorization = %q, want the new token", got)
	}
}

func TestTokenIsSafeForConcurrentUse(t *testing.T) {
	f := newFakeAuth0(t)
	f.handle("GET /api/v2/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, 200, map[string]any{"user_id": r.PathValue("id")})
	})
	now := start
	c := f.client(&now)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if _, err := c.User(t.Context(), "auth0|1"); err != nil {
				t.Error(err)
			}
		})
	}
	wg.Wait()
	if f.tokenCalls != 1 {
		t.Errorf("token fetched %d times, want 1", f.tokenCalls)
	}
}

func TestTokenFailureDoesNotLeakSecret(t *testing.T) {
	f := newFakeAuth0(t)
	now := start
	c := NewAuth0(f.srv.Client(), Settings{
		Domain: "club.uk.auth0.com", ClientID: "mgmt-id", ClientSecret: "wrong-secret", RoleID: "rol_admin", AppClientID: "spa-id",
	}, WithBaseURL(f.srv.URL), WithClock(func() time.Time { return now }))
	_, err := c.User(t.Context(), "auth0|1")
	if err == nil {
		t.Fatal("expected an error")
	}
	if strings.Contains(err.Error(), "wrong-secret") {
		t.Errorf("error %q leaks the client secret", err)
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("error %q doesn't say what Auth0 answered", err)
	}
}

func TestUser(t *testing.T) {
	f := newFakeAuth0(t)
	var gotPath, gotQuery string
	f.handle("GET /api/v2/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.EscapedPath(), r.URL.Query().Get("fields")
		if r.PathValue("id") == "auth0|gone" {
			auth0Error(w, 404, "inexistent_user", "The user does not exist.")
			return
		}
		writeTestJSON(w, 200, map[string]any{
			"user_id": "auth0|1", "name": "Ann", "email": "ann@example.com",
			"last_login": "2026-09-30T18:04:05.123Z", "logins_count": 3,
		})
	})
	now := start
	c := f.client(&now)

	got, err := c.User(t.Context(), "auth0|1")
	if err != nil {
		t.Fatal(err)
	}
	want := User{ID: "auth0|1", Name: "Ann", Email: "ann@example.com",
		LastLogin: time.Date(2026, 9, 30, 18, 4, 5, 123e6, time.UTC), LoginsCount: 3}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("user (-want +got):\n%s", diff)
	}
	if gotPath != "/api/v2/users/auth0%7C1" {
		t.Errorf("path = %q, want the id escaped", gotPath)
	}
	if gotQuery != "user_id,name,email,last_login,logins_count" {
		t.Errorf("fields = %q", gotQuery)
	}

	_, err = c.User(t.Context(), "auth0|gone")
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("missing user: err = %v, want ErrNotFound", err)
	}
	var apiErr *APIError
	if !errors.As(err, &apiErr) || apiErr.Status != 404 || apiErr.Code != "inexistent_user" {
		t.Errorf("missing user: err = %#v, want an APIError with Auth0's status and code", err)
	}
}

func TestAdminsPaginatesAndAddsDetails(t *testing.T) {
	f := newFakeAuth0(t)
	var froms []string
	f.handle("GET /api/v2/roles/rol_admin/users", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		froms = append(froms, q.Get("from"))
		if q.Get("take") == "" {
			t.Errorf("role users asked without take: %q", r.URL.RawQuery)
		}
		switch q.Get("from") {
		case "":
			writeTestJSON(w, 200, map[string]any{"next": "page2", "users": []map[string]any{
				{"user_id": "auth0|1", "name": "Ann", "email": "ann@example.com"},
				{"user_id": "auth0|2", "name": "Bob", "email": "bob@example.com"},
			}})
		case "page2":
			writeTestJSON(w, 200, map[string]any{"users": []map[string]any{
				// Just invited, so not in the search index yet.
				{"user_id": "auth0|3", "name": "Cat", "email": "cat@example.com"},
			}})
		default:
			t.Errorf("unexpected from %q", q.Get("from"))
		}
	})
	var gotSearch string
	f.handle("GET /api/v2/users", func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		gotSearch = q.Get("q")
		if q.Get("search_engine") != "v3" {
			t.Errorf("search_engine = %q", q.Get("search_engine"))
		}
		writeTestJSON(w, 200, []map[string]any{
			{"user_id": "auth0|1", "name": "Ann", "email": "ann@example.com", "last_login": "2026-09-30T18:04:05.000Z", "logins_count": 3},
			{"user_id": "auth0|2", "name": "Bob", "email": "bob@example.com", "logins_count": 0},
		})
	})
	now := start
	c := f.client(&now)

	got, err := c.Admins(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := []User{
		{ID: "auth0|1", Name: "Ann", Email: "ann@example.com", LastLogin: time.Date(2026, 9, 30, 18, 4, 5, 0, time.UTC), LoginsCount: 3},
		{ID: "auth0|2", Name: "Bob", Email: "bob@example.com"},
		{ID: "auth0|3", Name: "Cat", Email: "cat@example.com"},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("admins (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"", "page2"}, froms); diff != "" {
		t.Errorf("pages asked for (-want +got):\n%s", diff)
	}
	if want := `user_id:("auth0|1" OR "auth0|2" OR "auth0|3")`; gotSearch != want {
		t.Errorf("q = %q, want %q", gotSearch, want)
	}
}

func TestAdminsWithNoneSkipsSearch(t *testing.T) {
	f := newFakeAuth0(t)
	f.handle("GET /api/v2/roles/rol_admin/users", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, 200, map[string]any{"users": []any{}})
	})
	f.handle("GET /api/v2/users", func(w http.ResponseWriter, r *http.Request) {
		t.Error("searched for no users")
	})
	now := start
	got, err := f.client(&now).Admins(t.Context())
	if err != nil || len(got) != 0 {
		t.Errorf("Admins() = %v, %v; want none", got, err)
	}
}

func TestAdminsRefusesEndlessPages(t *testing.T) {
	f := newFakeAuth0(t)
	f.handle("GET /api/v2/roles/rol_admin/users", func(w http.ResponseWriter, r *http.Request) {
		writeTestJSON(w, 200, map[string]any{"next": "more", "users": []map[string]any{{"user_id": "auth0|x"}}})
	})
	now := start
	if _, err := f.client(&now).Admins(t.Context()); err == nil {
		t.Error("expected an error rather than a truncated list")
	}
}

// passwordOK checks Auth0's "good" policy at length 12 or more: at least
// three of lower, upper, digits and symbols. It insists on all four.
func passwordOK(pw string) bool {
	var lower, upper, digit, symbol bool
	for _, c := range pw {
		switch {
		case unicode.IsLower(c):
			lower = true
		case unicode.IsUpper(c):
			upper = true
		case unicode.IsDigit(c):
			digit = true
		default:
			symbol = true
		}
	}
	return len(pw) >= 32 && lower && upper && digit && symbol
}

func TestCreateUser(t *testing.T) {
	f := newFakeAuth0(t)
	var got map[string]any
	f.handle("POST /api/v2/users", func(w http.ResponseWriter, r *http.Request) {
		got = decodeBody[map[string]any](t, r)
		writeTestJSON(w, 201, map[string]any{"user_id": "auth0|new", "name": got["name"], "email": got["email"], "logins_count": 0})
	})
	now := start
	u, err := f.client(&now).CreateUser(t.Context(), "Dee", "dee@example.com")
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(User{ID: "auth0|new", Name: "Dee", Email: "dee@example.com"}, u); diff != "" {
		t.Errorf("user (-want +got):\n%s", diff)
	}
	pw, _ := got["password"].(string)
	if !passwordOK(pw) {
		t.Errorf("password %q doesn't meet the policy", pw)
	}
	delete(got, "password")
	want := map[string]any{
		"connection": "admins", "email": "dee@example.com", "name": "Dee",
		"email_verified": false, "verify_email": false,
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("create body (-want +got):\n%s", diff)
	}
}

func TestCreateUserFindsExistingUser(t *testing.T) {
	f := newFakeAuth0(t)
	f.handle("POST /api/v2/users", func(w http.ResponseWriter, r *http.Request) {
		auth0Error(w, 409, "auth0_idp_error", "The user already exists.")
	})
	var gotEmail string
	f.handle("GET /api/v2/users-by-email", func(w http.ResponseWriter, r *http.Request) {
		gotEmail = r.URL.Query().Get("email")
		writeTestJSON(w, 200, []map[string]any{
			{"user_id": "google-oauth2|9", "email": "dee@example.com", "identities": []map[string]any{{"connection": "google-oauth2"}}},
			{"user_id": "auth0|old", "name": "Dee", "email": "dee@example.com", "logins_count": 7,
				"last_login": "2026-01-02T03:04:05.000Z", "identities": []map[string]any{{"connection": "admins"}}},
		})
	})
	now := start
	u, err := f.client(&now).CreateUser(t.Context(), "Dee", "Dee@Example.com")
	if err != nil {
		t.Fatal(err)
	}
	want := User{ID: "auth0|old", Name: "Dee", Email: "dee@example.com", LoginsCount: 7,
		LastLogin: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)}
	if diff := cmp.Diff(want, u); diff != "" {
		t.Errorf("user (-want +got):\n%s", diff)
	}
	if gotEmail != "dee@example.com" {
		t.Errorf("looked up %q, want the email in lower case", gotEmail)
	}
}

func TestCreateUserErrorNeverShowsPassword(t *testing.T) {
	f := newFakeAuth0(t)
	var pw string
	f.handle("POST /api/v2/users", func(w http.ResponseWriter, r *http.Request) {
		pw, _ = decodeBody[map[string]any](t, r)["password"].(string)
		auth0Error(w, 400, "invalid_body", "Password "+pw+" is too weak")
	})
	now := start
	_, err := f.client(&now).CreateUser(t.Context(), "Dee", "dee@example.com")
	if err == nil {
		t.Fatal("expected an error")
	}
	if pw == "" || strings.Contains(err.Error(), pw) {
		t.Errorf("error %q shows the password", err)
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error %q doesn't say what Auth0 answered", err)
	}
}

func TestNewPasswordMeetsPolicy(t *testing.T) {
	seen := map[string]bool{}
	for range 200 {
		pw, err := newPassword()
		if err != nil {
			t.Fatal(err)
		}
		if !passwordOK(pw) {
			t.Fatalf("password %q doesn't meet the policy", pw)
		}
		if seen[pw] {
			t.Fatalf("password %q repeated", pw)
		}
		seen[pw] = true
	}
}

func TestRoles(t *testing.T) {
	f := newFakeAuth0(t)
	var assigned, removed map[string][]string
	f.handle("POST /api/v2/roles/rol_admin/users", func(w http.ResponseWriter, r *http.Request) {
		assigned = decodeBody[map[string][]string](t, r)
		w.WriteHeader(200)
	})
	f.handle("DELETE /api/v2/users/{id}/roles", func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("id") == "auth0|gone" {
			auth0Error(w, 404, "inexistent_user", "The user does not exist.")
			return
		}
		removed = decodeBody[map[string][]string](t, r)
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(204)
	})
	f.handle("GET /api/v2/users/{id}/roles", func(w http.ResponseWriter, r *http.Request) {
		roles := []map[string]string{{"id": "rol_other", "name": "Other"}}
		if r.PathValue("id") == "auth0|admin" {
			roles = append(roles, map[string]string{"id": "rol_admin", "name": "Club admin"})
		}
		writeTestJSON(w, 200, roles)
	})
	now := start
	c := f.client(&now)

	if err := c.GrantAdmin(t.Context(), "auth0|1"); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string][]string{"users": {"auth0|1"}}, assigned); diff != "" {
		t.Errorf("assign body (-want +got):\n%s", diff)
	}
	if err := c.RevokeAdmin(t.Context(), "auth0|1"); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string][]string{"roles": {"rol_admin"}}, removed); diff != "" {
		t.Errorf("remove body (-want +got):\n%s", diff)
	}
	if err := c.RevokeAdmin(t.Context(), "auth0|gone"); !errors.Is(err, ErrNotFound) {
		t.Errorf("revoke missing user: err = %v, want ErrNotFound", err)
	}

	for id, want := range map[string]bool{"auth0|admin": true, "auth0|1": false} {
		got, err := c.IsAdmin(t.Context(), id)
		if err != nil || got != want {
			t.Errorf("IsAdmin(%q) = %v, %v; want %v", id, got, err, want)
		}
	}
}

func TestSendPasswordEmail(t *testing.T) {
	f := newFakeAuth0(t)
	var got map[string]string
	var gotAuth string
	fail := false
	f.mux.HandleFunc("POST /dbconnections/change_password", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		got = decodeBody[map[string]string](t, r)
		if fail {
			auth0Error(w, 429, "too_many_requests", "Too many requests")
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		_, _ = io.WriteString(w, "We've just sent you an email to reset your password.")
	})
	now := start
	c := f.client(&now)

	if err := c.SendPasswordEmail(t.Context(), "dee@example.com"); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"client_id": "spa-id", "email": "dee@example.com", "connection": "admins"}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("change_password body (-want +got):\n%s", diff)
	}
	if gotAuth != "" || f.tokenCalls != 0 {
		t.Errorf("sent a token (%q, %d fetched); the Authentication API doesn't need one", gotAuth, f.tokenCalls)
	}

	fail = true
	if err := c.SendPasswordEmail(t.Context(), "dee@example.com"); err == nil || !strings.Contains(err.Error(), "429") {
		t.Errorf("err = %v, want one naming Auth0's 429", err)
	}
}

func TestUnauthorisedDropsCachedToken(t *testing.T) {
	f := newFakeAuth0(t)
	calls := 0
	f.handle("GET /api/v2/users/{id}", func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			auth0Error(w, 401, "invalid_token", "Expired token")
			return
		}
		writeTestJSON(w, 200, map[string]any{"user_id": "auth0|1"})
	})
	now := start
	c := f.client(&now)
	if _, err := c.User(t.Context(), "auth0|1"); err == nil {
		t.Fatal("expected the 401 to come back as an error")
	}
	if _, err := c.User(t.Context(), "auth0|1"); err != nil {
		t.Fatal(err)
	}
	if f.tokenCalls != 2 {
		t.Errorf("token fetched %d times, want a fresh one after the 401", f.tokenCalls)
	}
}
