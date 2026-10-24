package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/lestrrat-go/jwx/v3/jwa"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const testAudience = "https://api.example.org"

// issuer is a fake Auth0 tenant: it publishes a signing key and signs tokens.
type issuer struct {
	srv *httptest.Server
	key *rsa.PrivateKey
}

func newIssuer(t *testing.T) *issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	iss := &issuer{key: key}
	pub, err := jwk.Import(&key.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range map[string]any{jwk.KeyIDKey: "test-key", jwk.AlgorithmKey: jwa.RS256(), jwk.KeyUsageKey: "sig"} {
		if err := pub.Set(k, v); err != nil {
			t.Fatal(err)
		}
	}
	set := jwk.NewSet()
	if err := set.AddKey(pub); err != nil {
		t.Fatal(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/jwks.json", func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(set)
	})
	iss.srv = httptest.NewServer(mux)
	t.Cleanup(iss.srv.Close)
	return iss
}

func (i *issuer) url() string { return i.srv.URL + "/" }

// token signs a token with the given key and claims.
func (i *issuer) token(t *testing.T, key *rsa.PrivateKey, issuerURL, audience string, expiry time.Time, permissions []string) string {
	t.Helper()
	signingKey, err := jwk.Import(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := signingKey.Set(jwk.KeyIDKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	tok, err := jwt.NewBuilder().
		Issuer(issuerURL).
		Subject("auth0|alice").
		Audience([]string{audience}).
		Expiration(expiry).
		IssuedAt(time.Now()).
		Claim("permissions", permissions).
		Build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), signingKey))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestRequirePermission(t *testing.T) {
	iss := newIssuer(t)
	otherKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	v, err := NewVerifier(t.Context(), iss.url(), testAudience, iss.srv.Client(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}

	var gotCaller string
	protected := v.RequirePermission("manage:posts")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCaller = CallerID(r)
	}))

	hour := time.Now().Add(time.Hour)
	tests := []struct {
		name       string
		header     string
		wantStatus int
		wantCaller string
	}{
		{"valid token with permission", "Bearer " + iss.token(t, iss.key, iss.url(), testAudience, hour, []string{"manage:posts"}), 200, "auth0|alice"},
		{"no token", "", 401, ""},
		{"not a bearer token", "Basic abc", 401, ""},
		{"garbage token", "Bearer not.a.jwt", 401, ""},
		{"expired", "Bearer " + iss.token(t, iss.key, iss.url(), testAudience, time.Now().Add(-time.Hour), []string{"manage:posts"}), 401, ""},
		{"wrong audience", "Bearer " + iss.token(t, iss.key, iss.url(), "https://other.example", hour, []string{"manage:posts"}), 401, ""},
		{"wrong issuer", "Bearer " + iss.token(t, iss.key, "https://evil.example/", testAudience, hour, []string{"manage:posts"}), 401, ""},
		{"signed by another key", "Bearer " + iss.token(t, otherKey, iss.url(), testAudience, hour, []string{"manage:posts"}), 401, ""},
		{"valid but missing permission", "Bearer " + iss.token(t, iss.key, iss.url(), testAudience, hour, []string{"read:things"}), 403, ""},
		{"HS256 token signed with a shared secret", "Bearer " + hmacToken(t, iss.url()), 401, ""},
		{"token without a key ID", "Bearer " + iss.tokenWithoutKid(t), 401, ""},
		{"empty bearer", "Bearer ", 401, ""},
		{"valid with no permissions at all", "Bearer " + iss.token(t, iss.key, iss.url(), testAudience, hour, nil), 403, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCaller = ""
			req := httptest.NewRequestWithContext(t.Context(), "POST", "/api/admin/posts", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			protected.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if gotCaller != tt.wantCaller {
				t.Errorf("caller = %q, want %q", gotCaller, tt.wantCaller)
			}
			if tt.wantStatus == 401 && rec.Header().Get("WWW-Authenticate") == "" {
				t.Error("401 without a WWW-Authenticate header")
			}
		})
	}
}

func TestNewVerifierNeedsSettings(t *testing.T) {
	logger := slog.New(slog.DiscardHandler)
	if _, err := NewVerifier(t.Context(), "", testAudience, http.DefaultClient, logger); err == nil {
		t.Error("expected an error with no issuer")
	}
	if _, err := NewVerifier(t.Context(), "https://club.eu.auth0.com/", "", http.DefaultClient, logger); err == nil {
		t.Error("expected an error with no audience")
	}
}

func TestCallerIDWithoutToken(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/", nil)
	if got := CallerID(req); got != "" {
		t.Errorf("CallerID on an unauthenticated request = %q, want empty", got)
	}
}

func TestBuildToken(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	mw, err := BuildToken("s3cret-build-token")
	if err != nil {
		t.Fatal(err)
	}
	h := mw(ok)

	tests := []struct {
		name   string
		header string
		want   int
	}{
		{"correct token", "Bearer s3cret-build-token", 200},
		{"wrong token", "Bearer nope", 401},
		{"prefix of the token", "Bearer s3cret", 401},
		{"empty bearer", "Bearer ", 401},
		{"no header", "", 401},
		{"token without Bearer", "s3cret-build-token", 401},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), "GET", "/api/posts", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestBuildTokenRefusesEmptyToken(t *testing.T) {
	if _, err := BuildToken(""); err == nil {
		t.Fatal("an empty build token must be refused, or an empty bearer would get in")
	}
}

// hmacToken tries algorithm confusion: an HS256 token naming the real key ID.
func hmacToken(t *testing.T, issuerURL string) string {
	t.Helper()
	key, err := jwk.Import([]byte("guessable-secret"))
	if err != nil {
		t.Fatal(err)
	}
	if err := key.Set(jwk.KeyIDKey, "test-key"); err != nil {
		t.Fatal(err)
	}
	tok, err := jwt.NewBuilder().Issuer(issuerURL).Subject("auth0|mallory").Audience([]string{testAudience}).
		Expiration(time.Now().Add(time.Hour)).Claim("permissions", []string{"manage:posts"}).Build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Sign(tok, jwt.WithKey(jwa.HS256(), key))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func (i *issuer) tokenWithoutKid(t *testing.T) string {
	t.Helper()
	tok, err := jwt.NewBuilder().Issuer(i.url()).Subject("auth0|alice").Audience([]string{testAudience}).
		Expiration(time.Now().Add(time.Hour)).Claim("permissions", []string{"manage:posts"}).Build()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := jwt.Sign(tok, jwt.WithKey(jwa.RS256(), i.key))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestKeysUnavailableIs503(t *testing.T) {
	down := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(down.Close)
	iss := newIssuer(t)
	v, err := NewVerifier(t.Context(), down.URL+"/", testAudience, down.Client(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	v.keysWait = 50 * time.Millisecond
	h := v.RequirePermission("manage:posts")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler ran without verified keys")
	}))
	req := httptest.NewRequestWithContext(t.Context(), "GET", "/api/admin/posts", nil)
	req.Header.Set("Authorization", "Bearer "+iss.token(t, iss.key, down.URL+"/", testAudience, time.Now().Add(time.Hour), []string{"manage:posts"}))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d, want 503 when Auth0's keys can't be fetched", rec.Code)
	}
}

func TestNewVerifierWantsAuth0StyleIssuer(t *testing.T) {
	if _, err := NewVerifier(t.Context(), "https://club.eu.auth0.com", testAudience, http.DefaultClient, slog.New(slog.DiscardHandler)); err == nil {
		t.Error("expected an error for an issuer without the trailing slash Auth0 puts in tokens")
	}
}
