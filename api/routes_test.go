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
		{"health ok", "GET", "/healthz", http.StatusOK},
		{"health wrong method", "POST", "/healthz", http.StatusMethodNotAllowed},
		{"unknown path", "GET", "/nope", http.StatusNotFound},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))
			if rec.Code != tt.want {
				t.Fatalf("got %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

func TestCIFail(t *testing.T) {
	want := 1
	got := 2

	if want != got {
		t.Fatalf("got %d, want %d", got, want)
	}
}
