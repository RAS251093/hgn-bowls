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

	jwtmiddleware "github.com/auth0/go-jwt-middleware/v3"
	"github.com/auth0/go-jwt-middleware/v3/jwks"
	"github.com/auth0/go-jwt-middleware/v3/validator"
)

// keysTTL is how long Auth0's signing keys are cached before refetching.
const keysTTL = 5 * time.Minute

// claims are the custom claims Auth0 adds when RBAC is on for the API.
type claims struct {
	Permissions []string `json:"permissions"`
}

// Validate is required by validator.CustomClaims; nothing extra to check.
func (c *claims) Validate(context.Context) error { return nil }

// Verifier checks Auth0 access tokens.
type Verifier struct {
	check  *jwtmiddleware.JWTMiddleware
	logger *slog.Logger
}

// NewVerifier returns a Verifier for tokens from issuerURL (for example
// "https://club.eu.auth0.com/") meant for audience. Signing keys are fetched
// with client, which should have a timeout.
func NewVerifier(issuerURL, audience string, client *http.Client, logger *slog.Logger) (*Verifier, error) {
	if issuerURL == "" || audience == "" {
		return nil, errors.New("auth: issuer and audience are required")
	}
	u, err := url.Parse(issuerURL)
	if err != nil {
		return nil, fmt.Errorf("auth: parse issuer: %w", err)
	}
	keys, err := jwks.NewCachingProvider(
		jwks.WithIssuerURL(u),
		jwks.WithCacheTTL(keysTTL),
		jwks.WithCustomClient(client),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	v, err := validator.New(
		validator.WithKeyFunc(keys.KeyFunc),
		validator.WithAlgorithm(validator.RS256),
		validator.WithIssuer(issuerURL),
		validator.WithAudience(audience),
		validator.WithCustomClaims(func() *claims { return &claims{} }),
		validator.WithAllowedClockSkew(30*time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}

	ver := &Verifier{logger: logger}
	ver.check, err = jwtmiddleware.New(
		jwtmiddleware.WithValidator(v),
		jwtmiddleware.WithValidateOnOptions(false),
		jwtmiddleware.WithErrorHandler(func(w http.ResponseWriter, r *http.Request, err error) {
			if !errors.Is(err, jwtmiddleware.ErrJWTMissing) {
				ver.logger.Info("rejected token", "err", err)
			}
			unauthorized(w)
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("auth: %w", err)
	}
	return ver, nil
}

// RequirePermission lets a request through only with a valid token that
// carries perm. A missing or bad token gets 401; a good token without the
// permission gets 403.
func (v *Verifier) RequirePermission(perm string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return v.check.CheckJWT(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var custom *claims
			if c := validated(r); c != nil {
				custom, _ = c.CustomClaims.(*claims)
			}
			if custom == nil || !slices.Contains(custom.Permissions, perm) {
				writeError(w, http.StatusForbidden, "you do not have permission to do that")
				return
			}
			next.ServeHTTP(w, r)
		}))
	}
}

// CallerID returns the Auth0 user ID of the caller, or "" when the request
// has not been through RequirePermission.
func CallerID(r *http.Request) string {
	if c := validated(r); c != nil {
		return c.RegisteredClaims.Subject
	}
	return ""
}

func validated(r *http.Request) *validator.ValidatedClaims {
	c, err := jwtmiddleware.GetClaims[*validator.ValidatedClaims](r.Context())
	if err != nil {
		return nil
	}
	return c
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
			got, ok := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
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
