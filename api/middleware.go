package main

import (
	"log/slog"
	"net/http"
	"slices"
	"strings"
	"time"
)

// withCORS lets the listed origins (the website) call the API from the
// browser. It wraps the whole mux so preflight OPTIONS requests are answered
// here instead of reaching method-specific routes and getting 405.
//
// An entry like "https://*.hgn-bowls.pages.dev" allows any one subdomain of
// that host over that scheme, for Cloudflare Pages previews.
func withCORS(allowed []string, next http.Handler) http.Handler {
	isAllowed := originMatcher(allowed)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		ok := origin != "" && isAllowed(origin)

		if ok {
			h := w.Header()
			h.Set("Access-Control-Allow-Origin", origin)
			h.Set("Access-Control-Expose-Headers", "ETag")
		}

		if r.Method == http.MethodOptions && r.Header.Get("Access-Control-Request-Method") != "" {
			if !ok {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			h := w.Header()
			h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE")
			h.Set("Access-Control-Allow-Headers", "Authorization, Content-Type, If-Match")
			h.Set("Access-Control-Max-Age", "600")
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// originMatcher splits the allowed list into exact origins and subdomain
// patterns ("scheme://*.host"), and returns whether an origin matches one.
func originMatcher(allowed []string) func(string) bool {
	var exact []string
	type pattern struct{ scheme, suffix string }
	var patterns []pattern
	for _, a := range allowed {
		if scheme, host, ok := strings.Cut(a, "://*."); ok {
			patterns = append(patterns, pattern{scheme, "." + host})
			continue
		}
		exact = append(exact, a)
	}
	return func(origin string) bool {
		if slices.Contains(exact, origin) {
			return true
		}
		scheme, host, ok := strings.Cut(origin, "://")
		if !ok {
			return false
		}
		for _, p := range patterns {
			label, isSub := strings.CutSuffix(host, p.suffix)
			if scheme == p.scheme && isSub && isLabel(label) {
				return true
			}
		}
		return false
	}
}

// isLabel reports whether s is one DNS label: letters, digits and hyphens.
// That rules out deeper subdomains, ports, paths and the "*" itself.
func isLabel(s string) bool {
	if s == "" || len(s) > 63 {
		return false
	}
	for _, c := range s {
		letter := c >= 'a' && c <= 'z'
		digit := c >= '0' && c <= '9'
		if !letter && !digit && c != '-' {
			return false
		}
	}
	return true
}

// withRequestLog writes one log line per request. On Cloud Run the trace ID
// is attached so Cloud Logging groups a request's lines together.
func withRequestLog(logger *slog.Logger, project string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)

		attrs := []any{
			"method", r.Method,
			"path", r.URL.Path,
			"status", rec.status,
			"duration", time.Since(start).String(),
		}
		if trace, _, _ := strings.Cut(r.Header.Get("X-Cloud-Trace-Context"), "/"); trace != "" && project != "" {
			attrs = append(attrs, "logging.googleapis.com/trace", "projects/"+project+"/traces/"+trace)
		}
		logger.Info("request", attrs...)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }
