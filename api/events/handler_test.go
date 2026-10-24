package events

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/go-cmp/cmp"
)

type fakeUpcoming struct {
	gotLimit int
	result   Result
	err      error
}

func (f *fakeUpcoming) Upcoming(ctx context.Context, limit int) (Result, error) {
	f.gotLimit = limit
	return f.result, f.err
}

func TestHandler(t *testing.T) {
	events := []Event{{ID: "a", Title: "Open day"}}

	tests := []struct {
		name       string
		query      string
		result     Result
		err        error
		wantStatus int
		wantLimit  int // 0 means the source must not be called
		wantBody   string
	}{
		{name: "default limit", query: "", result: Result{Events: events}, wantStatus: 200, wantLimit: 10,
			wantBody: `{"events":[{"id":"a","title":"Open day","start":"","end":"","allDay":false,"location":"","description":""}],"stale":false}`},
		{name: "explicit limit", query: "?limit=4", result: Result{Events: events}, wantStatus: 200, wantLimit: 4},
		{name: "limit capped at 50", query: "?limit=500", result: Result{Events: events}, wantStatus: 200, wantLimit: 50},
		{name: "stale snapshot flagged", query: "", result: Result{Events: events, Stale: true}, wantStatus: 200, wantLimit: 10,
			wantBody: `{"events":[{"id":"a","title":"Open day","start":"","end":"","allDay":false,"location":"","description":""}],"stale":true}`},
		{name: "no events is an empty list", query: "", result: Result{}, wantStatus: 200, wantLimit: 10,
			wantBody: `{"events":[],"stale":false}`},
		{name: "non-numeric limit", query: "?limit=lots", wantStatus: 400},
		{name: "zero limit", query: "?limit=0", wantStatus: 400},
		{name: "unavailable", query: "", err: fmt.Errorf("fetching: %w", ErrUnavailable), wantStatus: 503, wantLimit: 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			src := &fakeUpcoming{result: tt.result, err: tt.err}
			h := NewHandler(src, slog.New(slog.DiscardHandler))

			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/api/events"+tt.query, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d (body %s)", rec.Code, tt.wantStatus, rec.Body)
			}
			if src.gotLimit != tt.wantLimit {
				t.Errorf("limit passed = %d, want %d", src.gotLimit, tt.wantLimit)
			}
			if tt.wantBody != "" {
				if diff := cmp.Diff(tt.wantBody+"\n", rec.Body.String()); diff != "" {
					t.Errorf("body mismatch (-want +got):\n%s", diff)
				}
			}
			if rec.Code == 200 {
				if got := rec.Header().Get("Cache-Control"); got != "public, max-age=60" {
					t.Errorf("Cache-Control = %q", got)
				}
				var v map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
					t.Errorf("body is not JSON: %v", err)
				}
			}
		})
	}
}

func TestHandlerErrorBody(t *testing.T) {
	h := NewHandler(&fakeUpcoming{err: ErrUnavailable}, slog.New(slog.DiscardHandler))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), "GET", "/api/events", nil))

	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q, want no-store so errors aren't cached", got)
	}
	if rec.Code != http.StatusServiceUnavailable {
		t.Errorf("status = %d", rec.Code)
	}
}
