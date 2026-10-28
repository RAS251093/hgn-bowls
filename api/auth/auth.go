// Package auth checks who is calling the API: Auth0 access tokens for the
// admin area, and a shared build token for the site build.
package auth

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/lestrrat-go/httprc/v3"
	"github.com/lestrrat-go/jwx/v3/jwk"
	"github.com/lestrrat-go/jwx/v3/jwt"
)

const (
	// Auth0 publishes its next signing key before rotating to it, so a
	// refresh every few minutes picks it up in time.
	keysMinRefresh = 5 * time.Minute
	keysMaxRefresh = time.Hour
	// keysWait bounds how long a request waits for the first key fetch
	// after a cold start.
	keysWait  = 5 * time.Second
	clockSkew = 30 * time.Second
)

// Verifier checks Auth0 access tokens.
type Verifier struct {
	keys     *jwk.Cache
	jwksURL  string
	issuer   string
	audience string
	keysWait time.Duration
	logger   *slog.Logger
}

// NewVerifier returns a Verifier for tokens from issuerURL (for example
// "https://club.eu.auth0.com/") meant for audience. Signing keys are fetched
// from the tenant's JWKS with client, which should have a timeout, and
// refreshed in the background until ctx ends.
func NewVerifier(ctx context.Context, issuerURL, audience string, client *http.Client, logger *slog.Logger) (*Verifier, error) {
	if issuerURL == "" || audience == "" {
		return nil, errors.New("auth: issuer and audience are required")
	}
	if !strings.HasSuffix(issuerURL, "/") {
		return nil, errors.New("auth: issuer must end in /, as Auth0 issues it")
	}
	// Auth0 always serves its keys here, so there's no need for OpenID discovery.
	jwksURL, err := url.JoinPath(issuerURL, ".well-known/jwks.json")
	if err != nil {
		return nil, fmt.Errorf("auth: issuer: %w", err)
	}

	keys, err := jwk.NewCache(ctx, httprc.NewClient())
	if err != nil {
		return nil, fmt.Errorf("auth: key cache: %w", err)
	}
	// Don't wait for the first fetch: the API should start even if Auth0 is slow.
	if err := keys.Register(ctx, jwksURL,
		jwk.WithHTTPClient(client),
		jwk.WithMinInterval(keysMinRefresh),
		jwk.WithMaxInterval(keysMaxRefresh),
		jwk.WithWaitReady(false),
	); err != nil {
		return nil, fmt.Errorf("auth: register keys: %w", err)
	}
	return &Verifier{keys: keys, jwksURL: jwksURL, issuer: issuerURL, audience: audience, keysWait: keysWait, logger: logger}, nil
}

// errKeysUnavailable means Auth0's signing keys couldn't be fetched.
var errKeysUnavailable = errors.New("signing keys unavailable")

// verify checks a raw token's signature, issuer, audience and dates.
func (v *Verifier) verify(ctx context.Context, raw string) (jwt.Token, error) {
	res, err := v.keys.LookupResource(ctx, v.jwksURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", errKeysUnavailable, err)
	}
	wait, cancel := context.WithTimeout(ctx, v.keysWait)
	defer cancel()
	if err := res.Ready(wait); err != nil {
		return nil, fmt.Errorf("%w: %w", errKeysUnavailable, err)
	}
	set := res.Resource()
	if set == nil {
		return nil, errKeysUnavailable
	}
	// WithKeySet only trusts the algorithm on the key (RS256 for Auth0),
	// never the one the token claims, and requires a matching kid.
	return jwt.ParseString(raw,
		jwt.WithKeySet(set),
		jwt.WithValidate(true),
		jwt.WithIssuer(v.issuer),
		jwt.WithAudience(v.audience),
		jwt.WithAcceptableSkew(clockSkew),
	)
}

type callerKey struct{}

// RequirePermission lets a request through only with a valid token that
// carries perm. A missing or bad token gets 401; a good token without the
// permission gets 403.
func (v *Verifier) RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			raw, ok := bearer(r)
			if !ok {
				unauthorized(w)
				return
			}
			tok, err := v.verify(r.Context(), raw)
			if errors.Is(err, errKeysUnavailable) {
				v.logger.Error("checking token", "err", err)
				writeError(w, http.StatusServiceUnavailable, "sign-in checks are unavailable right now")
				return
			}
			if err != nil {
				v.logger.Info("rejected token", "err", err)
				unauthorized(w)
				return
			}
			sub, _ := tok.Subject()
			if sub == "" {
				v.logger.Info("rejected token", "err", "no subject")
				unauthorized(w)
				return
			}
			if !slices.Contains(permissions(tok), perm) {
				writeError(w, http.StatusForbidden, "you do not have permission to do that")
				return
			}
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), callerKey{}, sub)))
		})
	}
}

// permissions reads the permissions claim Auth0 adds when RBAC is on.
func permissions(tok jwt.Token) []string {
	var raw []any
	if err := tok.Get("permissions", &raw); err != nil {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		if s, ok := p.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// CallerID returns the Auth0 user ID of the caller, or "" when the request
// has not been through RequirePermission.
func CallerID(r *http.Request) string {
	sub, _ := r.Context().Value(callerKey{}).(string)
	return sub
}

func bearer(r *http.Request) (string, bool) {
	tok, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
	return tok, ok && tok != ""
}

// BuildToken returns middleware that lets a request through only when it
// carries token as a bearer token. The site build uses it to read posts.
func BuildToken(token string) (func(http.Handler) http.Handler, error) {
	if token == "" {
		return nil, errors.New("auth: build token is empty")
	}
	want := []byte(token)
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			got, ok := bearer(r)
			if !ok || subtle.ConstantTimeCompare([]byte(got), want) != 1 {
				unauthorized(w)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}

func unauthorized(w http.ResponseWriter) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	writeError(w, http.StatusUnauthorized, "sign in to do that")
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
