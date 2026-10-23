package main

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRoutes(t *testing.T) {
	h := newHandler()
	tests := []struct {
		name   string
		method string
		path   string
		want   int
	}{
		{"health ok", "GET", "/health", http.StatusOK},
		{"health wrong method", "POST", "/health", http.StatusMethodNotAllowed},
		{"healthz 404", "GET", "/healthz", http.StatusNotFound},
		{"unknown path", "GET", "/nope", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), tt.method, tt.path, nil))
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d", rec.Code, tt.want)
			}
		})
	}
}
