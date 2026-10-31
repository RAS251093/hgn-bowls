// Package admins lets club admins see, invite and remove each other. An
// admin is an Auth0 user on the "admins" database connection who has the
// club-admin role; this package keeps those users in Auth0 through its
// Management API and serves the admin area's endpoints for them.
package admins

import (
	"bytes"
	"cmp"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// connection is the Auth0 database connection admins sign in with.
	connection = "admins"
	// tokenMargin is how long before expiry a Management API token is
	// replaced, so one never runs out mid-request.
	tokenMargin = time.Minute
	// pageSize is how many users to ask Auth0 for at once (its maximum
	// is 100), and maxPages stops a list going on for ever.
	pageSize = 100
	maxPages = 50
	// searchBatch keeps each user search's query to a sensible length.
	searchBatch = 50
	// userFields are the user details the admin area shows.
	userFields = "user_id,name,email,last_login,logins_count"
)

// ErrNotFound means Auth0 has no such user.
var ErrNotFound = errors.New("no such user")

// errUserExists means a user with that email is already on the connection.
var errUserExists = errors.New("user already exists")

// User is an Auth0 user as the admin area sees them.
type User struct {
	ID          string
	Name        string
	Email       string
	LastLogin   time.Time // zero if they've never signed in
	LoginsCount int
}

// APIError is an error answer from Auth0. Its message is Auth0's own, and
// never includes a password or secret.
type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("Auth0 answered %d %s: %s", e.Status, e.Code, e.Message)
}

// Is lets errors.Is match ErrNotFound and errUserExists.
func (e *APIError) Is(target error) bool {
	switch target {
	case ErrNotFound:
		return e.Status == http.StatusNotFound
	case errUserExists:
		return e.Status == http.StatusConflict
	}
	return false
}

// Settings are what the client needs to reach the tenant.
type Settings struct {
	Domain       string // for example "club.uk.auth0.com"
	ClientID     string // the machine-to-machine app allowed to manage users
	ClientSecret string
	RoleID       string // the club-admin role
	AppClientID  string // the admin area's app, which password emails send people back to
}

// Auth0 manages admins through Auth0's Management API. It's safe for
// concurrent use.
type Auth0 struct {
	client   *http.Client
	settings Settings
	baseURL  string
	now      func() time.Time

	mu        sync.Mutex
	token     string
	expiresAt time.Time
}

// Option changes how Auth0 is built.
type Option func(*Auth0)

// WithBaseURL points at a different server than the tenant (tests use a fake).
func WithBaseURL(u string) Option { return func(a *Auth0) { a.baseURL = u } }

// WithClock sets where the time comes from, for token expiry.
func WithClock(now func() time.Time) Option { return func(a *Auth0) { a.now = now } }

// NewAuth0 returns a client for the tenant in s. client should have a timeout.
func NewAuth0(client *http.Client, s Settings, opts ...Option) *Auth0 {
	a := &Auth0{client: client, settings: s, baseURL: "https://" + s.Domain, now: time.Now}
	for _, o := range opts {
		o(a)
	}
	return a
}

// auth0User is a user as the Management API sends one.
type auth0User struct {
	UserID      string `json:"user_id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	LastLogin   string `json:"last_login"`
	LoginsCount int    `json:"logins_count"`
	Identities  []struct {
		Connection string `json:"connection"`
	} `json:"identities"`
}

func (u auth0User) user() User {
	last, _ := time.Parse(time.RFC3339Nano, u.LastLogin) // zero when they've never signed in
	return User{ID: u.UserID, Name: u.Name, Email: u.Email, LastLogin: last, LoginsCount: u.LoginsCount}
}

// Admins returns every user with the club-admin role.
func (a *Auth0) Admins(ctx context.Context) ([]User, error) {
	var members []auth0User
	from := ""
	for page := 0; ; page++ {
		if page == maxPages {
			return nil, fmt.Errorf("list admins: more than %d pages of them", maxPages)
		}
		q := url.Values{"take": {strconv.Itoa(pageSize)}}
		if from != "" {
			q.Set("from", from)
		}
		var res struct {
			Users []auth0User `json:"users"`
			Next  string      `json:"next"`
		}
		if err := a.call(ctx, http.MethodGet, "/roles/"+url.PathEscape(a.settings.RoleID)+"/users", q, nil, &res); err != nil {
			return nil, fmt.Errorf("list admins: %w", err)
		}
		members = append(members, res.Users...)
		if res.Next == "" || len(res.Users) == 0 {
			break
		}
		from = res.Next
	}

	// The role's list only has names and emails, so look up the rest.
	details := map[string]auth0User{}
	for batch := range chunks(members, searchBatch) {
		found, err := a.search(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("list admins: %w", err)
		}
		for _, u := range found {
			details[u.UserID] = u
		}
	}
	out := make([]User, 0, len(members))
	for _, m := range members {
		// Search lags a little behind, so someone just invited may not be
		// found yet. They haven't signed in, so the role's list will do.
		if d, ok := details[m.UserID]; ok {
			m = d
		}
		out = append(out, m.user())
	}
	return out, nil
}

// search finds users by ID with Auth0's user search.
func (a *Auth0) search(ctx context.Context, users []auth0User) ([]auth0User, error) {
	ids := make([]string, len(users))
	for i, u := range users {
		ids[i] = phrase(u.UserID)
	}
	q := url.Values{
		"q":              {"user_id:(" + strings.Join(ids, " OR ") + ")"},
		"search_engine":  {"v3"},
		"fields":         {userFields},
		"include_fields": {"true"},
		"per_page":       {strconv.Itoa(pageSize)},
	}
	var found []auth0User
	if err := a.call(ctx, http.MethodGet, "/users", q, nil, &found); err != nil {
		return nil, fmt.Errorf("search users: %w", err)
	}
	return found, nil
}

// phrase quotes s as a Lucene phrase for Auth0's user search.
func phrase(s string) string {
	return `"` + strings.NewReplacer(`\`, `\\`, `"`, `\"`).Replace(s) + `"`
}

func chunks[T any](s []T, n int) func(func([]T) bool) {
	return func(yield func([]T) bool) {
		for len(s) > 0 {
			k := min(n, len(s))
			if !yield(s[:k]) {
				return
			}
			s = s[k:]
		}
	}
}

// User returns one user's details.
func (a *Auth0) User(ctx context.Context, id string) (User, error) {
	q := url.Values{"fields": {userFields}, "include_fields": {"true"}}
	var u auth0User
	if err := a.call(ctx, http.MethodGet, "/users/"+url.PathEscape(id), q, nil, &u); err != nil {
		return User{}, fmt.Errorf("get user: %w", err)
	}
	return u.user(), nil
}

// IsAdmin reports whether the user has the club-admin role.
func (a *Auth0) IsAdmin(ctx context.Context, id string) (bool, error) {
	var roles []struct {
		ID string `json:"id"`
	}
	q := url.Values{"per_page": {strconv.Itoa(pageSize)}}
	if err := a.call(ctx, http.MethodGet, "/users/"+url.PathEscape(id)+"/roles", q, nil, &roles); err != nil {
		return false, fmt.Errorf("get user's roles: %w", err)
	}
	for _, r := range roles {
		if r.ID == a.settings.RoleID {
			return true, nil
		}
	}
	return false, nil
}

// CreateUser adds a user to the admins connection with a random password
// nobody knows; they choose their own from the password email. If the
// email is already on the connection, that user is returned instead.
func (a *Auth0) CreateUser(ctx context.Context, name, email string) (User, error) {
	password, err := newPassword()
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	body := map[string]any{
		"connection":     connection,
		"email":          email,
		"name":           name,
		"password":       password,
		"email_verified": false,
		"verify_email":   false,
	}
	var u auth0User
	err = a.call(ctx, http.MethodPost, "/users", nil, body, &u)
	if errors.Is(err, errUserExists) {
		return a.userByEmail(ctx, email)
	}
	if apiErr, ok := errors.AsType[*APIError](err); ok {
		// Auth0 shouldn't repeat the password back, but make sure.
		apiErr.Message = strings.ReplaceAll(apiErr.Message, password, "[password]")
	}
	if err != nil {
		return User{}, fmt.Errorf("create user: %w", err)
	}
	return u.user(), nil
}

// userByEmail finds the admins-connection user with email.
func (a *Auth0) userByEmail(ctx context.Context, email string) (User, error) {
	// Auth0 keeps emails in lower case, and this lookup is exact.
	q := url.Values{"email": {strings.ToLower(email)}}
	var users []auth0User
	if err := a.call(ctx, http.MethodGet, "/users-by-email", q, nil, &users); err != nil {
		return User{}, fmt.Errorf("find existing user: %w", err)
	}
	for _, u := range users {
		for _, id := range u.Identities {
			if id.Connection == connection {
				return u.user(), nil
			}
		}
	}
	return User{}, fmt.Errorf("find existing user: Auth0 said they exist but has no %s user with that email", connection)
}

// GrantAdmin gives the user the club-admin role. Giving it twice is fine.
func (a *Auth0) GrantAdmin(ctx context.Context, id string) error {
	body := map[string][]string{"users": {id}}
	if err := a.call(ctx, http.MethodPost, "/roles/"+url.PathEscape(a.settings.RoleID)+"/users", nil, body, nil); err != nil {
		return fmt.Errorf("assign admin role: %w", err)
	}
	return nil
}

// RevokeAdmin takes the club-admin role away. The user stays in Auth0, so
// GrantAdmin can undo it.
func (a *Auth0) RevokeAdmin(ctx context.Context, id string) error {
	body := map[string][]string{"roles": {a.settings.RoleID}}
	if err := a.call(ctx, http.MethodDelete, "/users/"+url.PathEscape(id)+"/roles", nil, body, nil); err != nil {
		return fmt.Errorf("remove admin role: %w", err)
	}
	return nil
}

// SendPasswordEmail asks Auth0 to email a link for choosing a new
// password. It uses the Authentication API, which needs no token.
func (a *Auth0) SendPasswordEmail(ctx context.Context, email string) error {
	body := map[string]string{
		"client_id":  a.settings.AppClientID,
		"email":      email,
		"connection": connection,
	}
	if err := a.send(ctx, http.MethodPost, a.baseURL+"/dbconnections/change_password", "", body, nil); err != nil {
		return fmt.Errorf("send password email: %w", err)
	}
	return nil
}

// call sends a Management API request with a token, decoding the answer
// into out if it isn't nil.
func (a *Auth0) call(ctx context.Context, method, path string, q url.Values, in, out any) error {
	token, err := a.accessToken(ctx)
	if err != nil {
		return err
	}
	u := a.baseURL + "/api/v2" + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	err = a.send(ctx, method, u, token, in, out)
	if apiErr, ok := errors.AsType[*APIError](err); ok && apiErr.Status == http.StatusUnauthorized {
		a.forgetToken(token) // revoked or expired early: fetch a new one next time
	}
	return err
}

// accessToken returns a Management API token, fetching one with the
// client-credentials grant when the last has nearly run out. Holding the
// lock while fetching means concurrent requests share one fetch.
func (a *Auth0) accessToken(ctx context.Context) (string, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token != "" && a.now().Before(a.expiresAt) {
		return a.token, nil
	}
	body := map[string]string{
		"grant_type":    "client_credentials",
		"client_id":     a.settings.ClientID,
		"client_secret": a.settings.ClientSecret,
		"audience":      "https://" + a.settings.Domain + "/api/v2/",
	}
	var res struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"` // seconds
	}
	if err := a.send(ctx, http.MethodPost, a.baseURL+"/oauth/token", "", body, &res); err != nil {
		return "", fmt.Errorf("get management token: %w", err)
	}
	if res.AccessToken == "" {
		return "", errors.New("get management token: Auth0 sent no token")
	}
	life := time.Duration(res.ExpiresIn) * time.Second
	a.token = res.AccessToken
	a.expiresAt = a.now().Add(life - min(tokenMargin, life/2))
	return a.token, nil
}

func (a *Auth0) forgetToken(token string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.token == token {
		a.token = ""
	}
}

// send makes one JSON request to Auth0. A non-2xx answer becomes an
// *APIError; out, if not nil, gets the decoded body of a good one.
func (a *Auth0) send(ctx context.Context, method, u, token string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}
	// The URL is built from the configured tenant, not user input.
	req, err := http.NewRequestWithContext(ctx, method, u, body) //nolint:gosec // G704
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	resp, err := a.client.Do(req) //nolint:gosec // G704, as above
	if err != nil {
		// url.Error would print the URL, which can carry an email address.
		if uerr, ok := errors.AsType[*url.Error](err); ok {
			err = uerr.Err
		}
		return fmt.Errorf("call Auth0: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode/100 != 2 {
		// Management API errors say errorCode and message; the
		// Authentication API says error and error_description.
		var e struct {
			ErrorCode   string `json:"errorCode"`
			Error       string `json:"error"`
			Message     string `json:"message"`
			Description string `json:"error_description"`
		}
		_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&e)
		return &APIError{
			Status:  resp.StatusCode,
			Code:    cmp.Or(e.ErrorCode, e.Error, http.StatusText(resp.StatusCode)),
			Message: truncate(cmp.Or(e.Message, e.Description), 200),
		}
	}
	if out == nil {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<16))
		return nil
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(out); err != nil {
		return fmt.Errorf("decode Auth0's answer: %w", err)
	}
	return nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

// Passwords mix all four kinds of character Auth0's policies count, so
// they pass any of its strength levels.
const (
	passwordLength = 32
	lowerChars     = "abcdefghijklmnopqrstuvwxyz"
	upperChars     = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	digitChars     = "0123456789"
	symbolChars    = "!#$%&*+-.=?@^_~"
	passwordChars  = lowerChars + upperChars + digitChars + symbolChars
)

// newPassword returns a random password nobody will ever type: the new
// admin replaces it from the password email. It is never logged, stored
// or sent anywhere but Auth0.
func newPassword() (string, error) {
	for {
		pw := make([]byte, passwordLength)
		buf := make([]byte, 1)
		for i := range pw {
			// Reject bytes past the last whole run of the alphabet, so
			// every character is equally likely.
			for {
				if _, err := rand.Read(buf); err != nil {
					return "", fmt.Errorf("random password: %w", err)
				}
				if int(buf[0]) < 256-256%len(passwordChars) {
					pw[i] = passwordChars[int(buf[0])%len(passwordChars)]
					break
				}
			}
		}
		s := string(pw)
		if strings.ContainsAny(s, lowerChars) && strings.ContainsAny(s, upperChars) &&
			strings.ContainsAny(s, digitChars) && strings.ContainsAny(s, symbolChars) {
			return s, nil
		}
	}
}
