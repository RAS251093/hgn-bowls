package admins

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

// fakeDirectory keeps users in memory. fail makes the named method return
// an error, standing in for Auth0 being down.
type fakeDirectory struct {
	users  map[string]User
	admins map[string]bool
	sent   []string
	fail   map[string]error
	nextID string
}

func newFakeDirectory() *fakeDirectory {
	return &fakeDirectory{
		users: map[string]User{
			"auth0|me":  {ID: "auth0|me", Name: "Rob", Email: "rob@example.com", LoginsCount: 5, LastLogin: time.Date(2026, 10, 30, 8, 0, 0, 0, time.UTC)},
			"auth0|ann": {ID: "auth0|ann", Name: "ann", Email: "ann@example.com"},
			"auth0|zed": {ID: "auth0|zed", Name: "Zed", Email: "zed@example.com", LoginsCount: 1, LastLogin: time.Date(2026, 9, 1, 12, 30, 0, 0, time.UTC)},
			"auth0|old": {ID: "auth0|old", Name: "Old", Email: "old@example.com", LoginsCount: 2, LastLogin: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		},
		admins: map[string]bool{"auth0|me": true, "auth0|ann": true, "auth0|zed": true},
		fail:   map[string]error{},
		nextID: "auth0|new",
	}
}

var errDown = errors.New("auth0: connection refused")

func (f *fakeDirectory) Admins(ctx context.Context) ([]User, error) {
	if err := f.fail["Admins"]; err != nil {
		return nil, err
	}
	var out []User
	for id := range f.admins {
		out = append(out, f.users[id])
	}
	return out, nil
}

func (f *fakeDirectory) User(ctx context.Context, id string) (User, error) {
	if err := f.fail["User"]; err != nil {
		return User{}, err
	}
	u, ok := f.users[id]
	if !ok {
		return User{}, ErrNotFound
	}
	return u, nil
}

func (f *fakeDirectory) IsAdmin(ctx context.Context, id string) (bool, error) {
	if err := f.fail["IsAdmin"]; err != nil {
		return false, err
	}
	if _, ok := f.users[id]; !ok {
		return false, ErrNotFound
	}
	return f.admins[id], nil
}

func (f *fakeDirectory) CreateUser(ctx context.Context, name, email string) (User, error) {
	if err := f.fail["CreateUser"]; err != nil {
		return User{}, err
	}
	for _, u := range f.users {
		if u.Email == email {
			return u, nil
		}
	}
	u := User{ID: f.nextID, Name: name, Email: email}
	f.users[u.ID] = u
	return u, nil
}

func (f *fakeDirectory) GrantAdmin(ctx context.Context, id string) error {
	if err := f.fail["GrantAdmin"]; err != nil {
		return err
	}
	f.admins[id] = true
	return nil
}

func (f *fakeDirectory) RevokeAdmin(ctx context.Context, id string) error {
	if err := f.fail["RevokeAdmin"]; err != nil {
		return err
	}
	if _, ok := f.users[id]; !ok {
		return ErrNotFound
	}
	delete(f.admins, id)
	return nil
}

func (f *fakeDirectory) SendPasswordEmail(ctx context.Context, email string) error {
	if err := f.fail["SendPasswordEmail"]; err != nil {
		return err
	}
	f.sent = append(f.sent, email)
	return nil
}

func serve(dir Directory) http.Handler {
	h := NewHandler(dir, func(*http.Request) string { return "auth0|me" }, slog.New(slog.DiscardHandler))
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/admin/admins", h.List)
	mux.HandleFunc("POST /api/admin/admins", h.Invite)
	mux.HandleFunc("POST /api/admin/admins/{id}/invite", h.Resend)
	mux.HandleFunc("DELETE /api/admin/admins/{id}", h.Remove)
	mux.HandleFunc("PUT /api/admin/admins/{id}", h.Restore)
	return mux
}

type reply struct {
	status int
	header http.Header
	body   string
}

func do(t *testing.T, h http.Handler, method, path, body string) reply {
	t.Helper()
	req := httptest.NewRequestWithContext(t.Context(), method, path, strings.NewReader(body))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	b, _ := io.ReadAll(rec.Body)
	r := reply{rec.Code, rec.Header(), string(b)}
	if got := r.header.Get("Cache-Control"); got != "no-store" {
		t.Errorf("%s %s: Cache-Control = %q, want no-store", method, path, got)
	}
	if got := r.header.Get("Content-Type"); got != "application/json" {
		t.Errorf("%s %s: Content-Type = %q", method, path, got)
	}
	return r
}

// asJSON decodes a reply generically, so tests compare the exact shape the
// admin area sees.
func asJSON(t *testing.T, r reply) any {
	t.Helper()
	var v any
	if err := json.Unmarshal([]byte(r.body), &v); err != nil {
		t.Fatalf("decode %q: %v", r.body, err)
	}
	return v
}

func expect(t *testing.T, r reply, status int, body string) {
	t.Helper()
	if r.status != status {
		t.Errorf("status = %d, want %d (%s)", r.status, status, r.body)
	}
	var want any
	if err := json.Unmarshal([]byte(body), &want); err != nil {
		t.Fatalf("bad want %q: %v", body, err)
	}
	if diff := cmp.Diff(want, asJSON(t, r)); diff != "" {
		t.Errorf("body (-want +got):\n%s", diff)
	}
}

func TestList(t *testing.T) {
	r := do(t, serve(newFakeDirectory()), "GET", "/api/admin/admins", "")
	expect(t, r, 200, `{"admins":[
		{"id":"auth0|ann","name":"ann","email":"ann@example.com","status":"invited","lastLogin":null},
		{"id":"auth0|me","name":"Rob","email":"rob@example.com","status":"active","lastLogin":"2026-10-30T08:00:00Z"},
		{"id":"auth0|zed","name":"Zed","email":"zed@example.com","status":"active","lastLogin":"2026-09-01T12:30:00Z"}
	]}`)
}

func TestListEmptyAndFailing(t *testing.T) {
	dir := newFakeDirectory()
	dir.admins = map[string]bool{}
	expect(t, do(t, serve(dir), "GET", "/api/admin/admins", ""), 200, `{"admins":[]}`)

	dir.fail["Admins"] = errDown
	r := do(t, serve(dir), "GET", "/api/admin/admins", "")
	if r.status != http.StatusBadGateway {
		t.Errorf("status = %d, want 502", r.status)
	}
}

func TestInvite(t *testing.T) {
	dir := newFakeDirectory()
	r := do(t, serve(dir), "POST", "/api/admin/admins", `{"name":"  Dee Jones ","email":" Dee@Example.com "}`)
	expect(t, r, 201, `{
		"admin":{"id":"auth0|new","name":"Dee Jones","email":"dee@example.com","status":"invited","lastLogin":null},
		"email":{"status":"sent"}
	}`)
	if !dir.admins["auth0|new"] {
		t.Error("new user wasn't given the admin role")
	}
	if diff := cmp.Diff([]string{"dee@example.com"}, dir.sent); diff != "" {
		t.Errorf("emails sent (-want +got):\n%s", diff)
	}
}

func TestInviteFormerAdmin(t *testing.T) {
	dir := newFakeDirectory()
	r := do(t, serve(dir), "POST", "/api/admin/admins", `{"name":"Old","email":"old@example.com"}`)
	expect(t, r, 201, `{
		"admin":{"id":"auth0|old","name":"Old","email":"old@example.com","status":"active","lastLogin":"2026-01-01T00:00:00Z"},
		"email":{"status":"sent"}
	}`)
	if !dir.admins["auth0|old"] {
		t.Error("former admin wasn't given the role back")
	}
}

func TestInviteValidation(t *testing.T) {
	const fix = "some fields need fixing"
	tests := []struct {
		name, body string
		status     int
		fields     map[string]string
	}{
		{"not json", `{`, 400, nil},
		{"unknown field", `{"name":"a","email":"a@example.com","role":"owner"}`, 400, nil},
		{"empty", `{}`, 422, map[string]string{
			"name":  "Add their name",
			"email": "Enter their email address, like name@example.com",
		}},
		{"blank name", `{"name":"   ","email":"a@example.com"}`, 422, map[string]string{"name": "Add their name"}},
		{"long name", `{"name":"` + strings.Repeat("é", 101) + `","email":"a@example.com"}`, 422, map[string]string{
			"name": "Shorten their name to 100 characters or fewer",
		}},
		{"not an email", `{"name":"Dee","email":"dee"}`, 422, map[string]string{
			"email": "Enter their email address, like name@example.com",
		}},
		{"display name", `{"name":"Dee","email":"Dee <dee@example.com>"}`, 422, map[string]string{
			"email": "Enter their email address, like name@example.com",
		}},
		{"two addresses", `{"name":"Dee","email":"a@example.com, b@example.com"}`, 422, map[string]string{
			"email": "Enter their email address, like name@example.com",
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := newFakeDirectory()
			r := do(t, serve(dir), "POST", "/api/admin/admins", tt.body)
			if r.status != tt.status {
				t.Fatalf("status = %d, want %d (%s)", r.status, tt.status, r.body)
			}
			if tt.fields != nil {
				got := asJSON(t, r).(map[string]any)
				if got["error"] != fix {
					t.Errorf("error = %v, want %q", got["error"], fix)
				}
				wantFields := map[string]any{}
				for k, v := range tt.fields {
					wantFields[k] = v
				}
				if diff := cmp.Diff(wantFields, got["fields"]); diff != "" {
					t.Errorf("fields (-want +got):\n%s", diff)
				}
			}
			if len(dir.sent) != 0 || dir.admins["auth0|new"] {
				t.Error("a rejected invite still reached Auth0")
			}
		})
	}
}

func TestInviteAlreadyAdmin(t *testing.T) {
	dir := newFakeDirectory()
	r := do(t, serve(dir), "POST", "/api/admin/admins", `{"name":"Zed","email":"zed@example.com"}`)
	expect(t, r, 409, `{"error":"They're already an admin"}`)
	if len(dir.sent) != 0 {
		t.Error("sent an email to someone who's already an admin")
	}
}

func TestInviteEmailFails(t *testing.T) {
	dir := newFakeDirectory()
	dir.fail["SendPasswordEmail"] = errDown
	r := do(t, serve(dir), "POST", "/api/admin/admins", `{"name":"Dee","email":"dee@example.com"}`)
	expect(t, r, 201, `{
		"admin":{"id":"auth0|new","name":"Dee","email":"dee@example.com","status":"invited","lastLogin":null},
		"email":{"status":"failed"}
	}`)
	if !dir.admins["auth0|new"] {
		t.Error("user should keep the role when only the email fails")
	}
}

func TestInviteAuth0Fails(t *testing.T) {
	for _, step := range []string{"CreateUser", "IsAdmin", "GrantAdmin"} {
		t.Run(step, func(t *testing.T) {
			dir := newFakeDirectory()
			dir.fail[step] = errDown
			r := do(t, serve(dir), "POST", "/api/admin/admins", `{"name":"Dee","email":"dee@example.com"}`)
			expect(t, r, 502, `{"error":"Auth0 didn't respond, so they haven't been invited. Try again in a moment."}`)
			if len(dir.sent) != 0 {
				t.Error("sent an email though the invite failed")
			}
		})
	}
}

func TestResend(t *testing.T) {
	dir := newFakeDirectory()
	h := serve(dir)
	expect(t, do(t, h, "POST", "/api/admin/admins/auth0%7Cann/invite", ""), 200, `{"email":{"status":"sent"}}`)
	if diff := cmp.Diff([]string{"ann@example.com"}, dir.sent); diff != "" {
		t.Errorf("emails sent (-want +got):\n%s", diff)
	}

	for _, id := range []string{"auth0%7Cold", "auth0%7Cnobody"} {
		r := do(t, h, "POST", "/api/admin/admins/"+id+"/invite", "")
		if r.status != http.StatusNotFound {
			t.Errorf("resend to %s: status = %d, want 404", id, r.status)
		}
	}
	if len(dir.sent) != 1 {
		t.Errorf("sent %d emails, want only the first", len(dir.sent))
	}

	dir.fail["SendPasswordEmail"] = errDown
	if r := do(t, h, "POST", "/api/admin/admins/auth0%7Cann/invite", ""); r.status != http.StatusBadGateway {
		t.Errorf("failed resend: status = %d, want 502", r.status)
	}
}

func TestRemove(t *testing.T) {
	dir := newFakeDirectory()
	h := serve(dir)
	expect(t, do(t, h, "DELETE", "/api/admin/admins/auth0%7Czed", ""), 200, `{}`)
	if dir.admins["auth0|zed"] {
		t.Error("role wasn't removed")
	}

	expect(t, do(t, h, "DELETE", "/api/admin/admins/auth0%7Cme", ""), 409, `{"error":"You can't remove your own access"}`)
	if !dir.admins["auth0|me"] {
		t.Error("removed the caller's own role")
	}

	if r := do(t, h, "DELETE", "/api/admin/admins/auth0%7Cnobody", ""); r.status != http.StatusNotFound {
		t.Errorf("remove unknown: status = %d, want 404", r.status)
	}
	dir.fail["RevokeAdmin"] = errDown
	if r := do(t, h, "DELETE", "/api/admin/admins/auth0%7Cann", ""); r.status != http.StatusBadGateway {
		t.Errorf("failed remove: status = %d, want 502", r.status)
	}
}

func TestRestore(t *testing.T) {
	dir := newFakeDirectory()
	h := serve(dir)
	r := do(t, h, "PUT", "/api/admin/admins/auth0%7Cold", "")
	expect(t, r, 200, `{"admin":{"id":"auth0|old","name":"Old","email":"old@example.com","status":"active","lastLogin":"2026-01-01T00:00:00Z"}}`)
	if !dir.admins["auth0|old"] {
		t.Error("role wasn't given back")
	}
	if len(dir.sent) != 0 {
		t.Error("restoring sent an email")
	}

	if r := do(t, h, "PUT", "/api/admin/admins/auth0%7Cnobody", ""); r.status != http.StatusNotFound {
		t.Errorf("restore unknown: status = %d, want 404", r.status)
	}
	dir.fail["GrantAdmin"] = errDown
	if r := do(t, h, "PUT", "/api/admin/admins/auth0%7Cann", ""); r.status != http.StatusBadGateway {
		t.Errorf("failed restore: status = %d, want 502", r.status)
	}
}

func TestNotConfigured(t *testing.T) {
	h := serve(nil)
	for _, r := range []struct{ method, path, body string }{
		{"GET", "/api/admin/admins", ""},
		{"POST", "/api/admin/admins", `{"name":"Dee","email":"dee@example.com"}`},
		{"POST", "/api/admin/admins/auth0%7Cann/invite", ""},
		{"DELETE", "/api/admin/admins/auth0%7Cann", ""},
		{"PUT", "/api/admin/admins/auth0%7Cann", ""},
	} {
		t.Run(r.method+" "+r.path, func(t *testing.T) {
			expect(t, do(t, h, r.method, r.path, r.body), 503, `{"error":"Inviting admins isn't set up yet"}`)
		})
	}
}
