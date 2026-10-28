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
func withCORS(allowed []string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Vary", "Origin")
		origin := r.Header.Get("Origin")
		ok := origin != "" && slices.Contains(allowed, origin)

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
