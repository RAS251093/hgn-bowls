package main

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCORS(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusTeapot) })
	h := withCORS([]string{"https://hgnbowls.example", "http://localhost:4321"}, ok)

	tests := []struct {
		name        string
		method      string
		origin      string
		wantStatus  int
		wantAllowed string // expected Access-Control-Allow-Origin ("" = header absent)
	}{
		{"allowed origin echoed", "GET", "https://hgnbowls.example", http.StatusTeapot, "https://hgnbowls.example"},
		{"second allowed origin", "GET", "http://localhost:4321", http.StatusTeapot, "http://localhost:4321"},
		{"other origin gets no CORS header", "GET", "https://evil.example", http.StatusTeapot, ""},
		{"no origin (server to server) passes through", "GET", "", http.StatusTeapot, ""},
		{"preflight from allowed origin", "OPTIONS", "https://hgnbowls.example", http.StatusNoContent, "https://hgnbowls.example"},
		{"preflight from other origin", "OPTIONS", "https://evil.example", http.StatusForbidden, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), tt.method, "/api/events", nil)
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			if tt.method == "OPTIONS" {
				req.Header.Set("Access-Control-Request-Method", "PUT")
			}
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if got := rec.Header().Get("Access-Control-Allow-Origin"); got != tt.wantAllowed {
				t.Errorf("Allow-Origin = %q, want %q", got, tt.wantAllowed)
			}
			if got := rec.Header().Get("Vary"); got != "Origin" {
				t.Errorf("Vary = %q, want Origin", got)
			}
		})
	}
}

// Pages gives every preview its own subdomain, so an entry may allow any one
// subdomain of a host. Nothing broader, and nothing that only looks similar.
func TestCORSSubdomainPattern(t *testing.T) {
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})
	h := withCORS([]string{"https://hgnbowls.example", "https://*.hgn-bowls.pages.dev"}, ok)

	tests := []struct {
		origin string
		want   bool
	}{
		{"https://hgb-08-whats-on.hgn-bowls.pages.dev", true},
		{"https://4f3a9c1e.hgn-bowls.pages.dev", true},
		{"https://hgn-bowls.pages.dev", false},              // the bare host isn't a subdomain; list it separately
		{"https://a.b.hgn-bowls.pages.dev", false},          // only one level
		{"http://preview.hgn-bowls.pages.dev", false},       // scheme must match
		{"https://preview.hgn-bowls.pages.dev:8443", false}, // so must the port
		{"https://previewhgn-bowls.pages.dev", false},       // not a subdomain at all
		{"https://preview.hgn-bowls.pages.dev.evil.example", false},
		{"https://evil-hgn-bowls.pages.dev", false},
		{"https://.hgn-bowls.pages.dev", false},  // empty label
		{"https://*.hgn-bowls.pages.dev", false}, // the pattern itself isn't an origin
		{"https://hgnbowls.example", true},       // exact entries still work
	}
	for _, tt := range tests {
		t.Run(tt.origin, func(t *testing.T) {
			req := httptest.NewRequestWithContext(t.Context(), "OPTIONS", "/api/events", nil)
			req.Header.Set("Origin", tt.origin)
			req.Header.Set("Access-Control-Request-Method", "GET")
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)

			got := rec.Header().Get("Access-Control-Allow-Origin")
			if tt.want && (rec.Code != http.StatusNoContent || got != tt.origin) {
				t.Errorf("status %d, Allow-Origin %q; want 204 echoing the origin", rec.Code, got)
			}
			if !tt.want && (rec.Code != http.StatusForbidden || got != "") {
				t.Errorf("status %d, Allow-Origin %q; want 403 and no header", rec.Code, got)
			}
		})
	}
}

func TestCORSPreflightAllowsAdminHeaders(t *testing.T) {
	h := withCORS([]string{"https://hgnbowls.example"}, http.NotFoundHandler())
	req := httptest.NewRequestWithContext(t.Context(), "OPTIONS", "/api/admin/posts/1", nil)
	req.Header.Set("Origin", "https://hgnbowls.example")
	req.Header.Set("Access-Control-Request-Method", "PUT")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	for header, want := range map[string]string{
		"Access-Control-Allow-Methods":  "GET, POST, PUT, DELETE",
		"Access-Control-Allow-Headers":  "Authorization, Content-Type, If-Match",
		"Access-Control-Expose-Headers": "ETag",
		"Access-Control-Max-Age":        "600",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
}

func TestRequestLog(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, nil))
	h := withRequestLog(logger, "my-project", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
	}))

	req := httptest.NewRequestWithContext(t.Context(), "POST", "/api/admin/posts", nil)
	req.Header.Set("X-Cloud-Trace-Context", "abc123/456;o=1")
	h.ServeHTTP(httptest.NewRecorder(), req)

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("log line is not JSON: %v (%s)", err, buf.String())
	}
	want := map[string]any{
		"method":                       "POST",
		"path":                         "/api/admin/posts",
		"status":                       float64(201),
		"logging.googleapis.com/trace": "projects/my-project/traces/abc123",
	}
	for k, v := range want {
		if line[k] != v {
			t.Errorf("log %s = %v, want %v", k, line[k], v)
		}
	}
	if _, ok := line["duration"]; !ok {
		t.Error("log line has no duration")
	}
}
