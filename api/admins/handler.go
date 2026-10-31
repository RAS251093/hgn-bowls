package admins

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/mail"
	"slices"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	// maxRequestBody caps an invite, which is just a name and an email.
	maxRequestBody = 4 << 10
	maxNameLength  = 100
	maxEmailLength = 254
)

// Directory is what the handlers need from Auth0 (*Auth0).
type Directory interface {
	Admins(ctx context.Context) ([]User, error)
	User(ctx context.Context, id string) (User, error)
	IsAdmin(ctx context.Context, id string) (bool, error)
	CreateUser(ctx context.Context, name, email string) (User, error)
	GrantAdmin(ctx context.Context, id string) error
	RevokeAdmin(ctx context.Context, id string) error
	SendPasswordEmail(ctx context.Context, email string) error
}

// Handler serves the admins endpoints.
type Handler struct {
	dir      Directory
	callerID func(*http.Request) string
	logger   *slog.Logger
}

// NewHandler returns the admins HTTP handlers. dir may be nil when Auth0
// management isn't set up, and then every endpoint says so. callerID
// returns the signed-in admin's user ID for a request.
func NewHandler(dir Directory, callerID func(*http.Request) string, logger *slog.Logger) *Handler {
	return &Handler{dir: dir, callerID: callerID, logger: logger}
}

// Admin is an admin as the admin area shows them.
type Admin struct {
	ID        string  `json:"id"`
	Name      string  `json:"name"`
	Email     string  `json:"email"`
	Status    string  `json:"status"`    // invited until they first sign in, then active
	LastLogin *string `json:"lastLogin"` // RFC 3339, or null if they never have
}

func adminOf(u User) Admin {
	a := Admin{ID: u.ID, Name: u.Name, Email: u.Email, Status: "active"}
	if u.LoginsCount == 0 {
		a.Status = "invited"
	}
	if !u.LastLogin.IsZero() {
		s := u.LastLogin.UTC().Format(time.RFC3339)
		a.LastLogin = &s
	}
	return a
}

type emailStatus struct {
	Status string `json:"status"` // sent or failed
}

// List serves GET /api/admin/admins.
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	users, err := h.dir.Admins(r.Context())
	if err != nil {
		h.auth0Failed(w, err, "listing admins", "Auth0 didn't respond, so the admins can't be shown. Try again in a moment.")
		return
	}
	out := make([]Admin, 0, len(users))
	for _, u := range users {
		out = append(out, adminOf(u))
	}
	slices.SortFunc(out, func(a, b Admin) int {
		return cmp.Or(
			cmp.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)),
			cmp.Compare(a.Email, b.Email),
		)
	})
	writeJSON(w, http.StatusOK, map[string][]Admin{"admins": out})
}

type invite struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

// validate tidies an invite and says what's wrong with it, by field.
func (in *invite) validate() map[string]string {
	fields := map[string]string{}
	in.Name = strings.TrimSpace(in.Name)
	switch {
	case in.Name == "":
		fields["name"] = "Add their name"
	case utf8.RuneCountInString(in.Name) > maxNameLength:
		fields["name"] = "Shorten their name to 100 characters or fewer"
	}
	// Auth0 keeps emails in lower case anyway.
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	if !plausibleEmail(in.Email) {
		fields["email"] = "Enter their email address, like name@example.com"
	}
	return fields
}

// plausibleEmail accepts a bare address, without a display name or <>.
func plausibleEmail(s string) bool {
	if s == "" || len(s) > maxEmailLength {
		return false
	}
	addr, err := mail.ParseAddress(s)
	return err == nil && addr.Name == "" && addr.Address == s
}

// Invite serves POST /api/admin/admins: it creates the user in Auth0 (or
// finds them, if they were an admin before), gives them the club-admin
// role and emails them a link to choose a password.
func (h *Handler) Invite(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	var in invite
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		writeError(w, http.StatusBadRequest, "the request body isn't valid JSON for an invite")
		return
	}
	if fields := in.validate(); len(fields) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, map[string]any{
			"error":  "some fields need fixing",
			"fields": fields,
		})
		return
	}

	const notInvited = "Auth0 didn't respond, so they haven't been invited. Try again in a moment."
	// Once started, an invite shouldn't stop halfway because the browser
	// went away.
	ctx := context.WithoutCancel(r.Context())
	u, err := h.dir.CreateUser(ctx, in.Name, in.Email)
	if err != nil {
		h.auth0Failed(w, err, "creating admin", notInvited)
		return
	}
	already, err := h.dir.IsAdmin(ctx, u.ID)
	if err != nil {
		h.auth0Failed(w, err, "checking admin role", notInvited)
		return
	}
	if already {
		writeError(w, http.StatusConflict, "They're already an admin")
		return
	}
	if err := h.dir.GrantAdmin(ctx, u.ID); err != nil {
		h.auth0Failed(w, err, "assigning admin role", notInvited)
		return
	}

	// They're an admin now, so a failed email still answers 201: the admin
	// area offers to send it again.
	email := emailStatus{Status: "sent"}
	if err := h.dir.SendPasswordEmail(ctx, u.Email); err != nil {
		h.logger.Error("sending password email", "err", err, "user", u.ID)
		email.Status = "failed"
	}
	writeJSON(w, http.StatusCreated, struct {
		Admin Admin       `json:"admin"`
		Email emailStatus `json:"email"`
	}{adminOf(u), email})
}

// Resend serves POST /api/admin/admins/{id}/invite, sending the password
// email again.
func (h *Handler) Resend(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	const notSent = "Auth0 didn't respond, so the email hasn't been sent. Try again in a moment."
	id := r.PathValue("id")
	isAdmin, err := h.dir.IsAdmin(r.Context(), id)
	if err == nil && !isAdmin {
		err = ErrNotFound
	}
	var u User
	if err == nil {
		u, err = h.dir.User(r.Context(), id)
	}
	if err == nil {
		err = h.dir.SendPasswordEmail(context.WithoutCancel(r.Context()), u.Email)
	}
	if err != nil {
		h.auth0Failed(w, err, "resending password email", notSent)
		return
	}
	writeJSON(w, http.StatusOK, map[string]emailStatus{"email": {Status: "sent"}})
}

// Remove serves DELETE /api/admin/admins/{id}, taking away the club-admin
// role. Their Auth0 user stays, so Restore can undo it.
func (h *Handler) Remove(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	id := r.PathValue("id")
	if id == h.callerID(r) {
		writeError(w, http.StatusConflict, "You can't remove your own access")
		return
	}
	if err := h.dir.RevokeAdmin(context.WithoutCancel(r.Context()), id); err != nil {
		h.auth0Failed(w, err, "removing admin role", "Auth0 didn't respond, so they're still an admin. Try again in a moment.")
		return
	}
	writeJSON(w, http.StatusOK, struct{}{})
}

// Restore serves PUT /api/admin/admins/{id}, giving the club-admin role
// back without another email: the admin area's Undo after Remove.
func (h *Handler) Restore(w http.ResponseWriter, r *http.Request) {
	if !h.ready(w) {
		return
	}
	const notRestored = "Auth0 didn't respond, so they're still removed. Try again in a moment."
	u, err := h.dir.User(r.Context(), r.PathValue("id"))
	if err == nil {
		err = h.dir.GrantAdmin(context.WithoutCancel(r.Context()), u.ID)
	}
	if err != nil {
		h.auth0Failed(w, err, "restoring admin role", notRestored)
		return
	}
	writeJSON(w, http.StatusOK, map[string]Admin{"admin": adminOf(u)})
}

// ready answers 503 when there's no Auth0 management client.
func (h *Handler) ready(w http.ResponseWriter) bool {
	if h.dir == nil {
		writeError(w, http.StatusServiceUnavailable, "Inviting admins isn't set up yet")
		return false
	}
	return true
}

// auth0Failed answers 404 for a user Auth0 doesn't have, or else logs the
// failure and answers 502 with msg.
func (h *Handler) auth0Failed(w http.ResponseWriter, err error, doing, msg string) {
	if errors.Is(err, ErrNotFound) {
		writeError(w, http.StatusNotFound, "We couldn't find that admin")
		return
	}
	h.logger.Error(doing, "err", err)
	writeError(w, http.StatusBadGateway, msg)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
