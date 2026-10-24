// api/events/calendar_test.go
package events

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
)

const (
	testCalendarID = "club@group.calendar.google.com"
	testAPIKey     = "test-key"
)

func TestCalendarUpcoming(t *testing.T) {
	now := time.Date(2026, 4, 1, 9, 0, 0, 0, time.UTC)
	fixture, err := os.ReadFile("testdata/events.json")
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got, want := r.URL.EscapedPath(), "/calendars/club@group.calendar.google.com/events"; got != want {
			t.Errorf("path = %q, want %q", got, want)
		}
		q := r.URL.Query()
		wantQuery := map[string]string{
			"key":          testAPIKey,
			"singleEvents": "true",
			"orderBy":      "startTime",
			"timeMin":      "2026-04-01T09:00:00Z",
			"maxResults":   "10",
		}
		for k, want := range wantQuery {
			if got := q.Get(k); got != want {
				t.Errorf("query %s = %q, want %q", k, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write(fixture)
	}))
	defer srv.Close()

	cal := NewCalendar(srv.Client(), testCalendarID, testAPIKey, WithBaseURL(srv.URL))
	got, err := cal.Upcoming(t.Context(), now, 10)
	if err != nil {
		t.Fatalf("Upcoming: %v", err)
	}

	want := []Event{
		{
			ID:          "timed1",
			Title:       "Open bowling day",
			Start:       "2026-04-18T14:00:00+01:00", // 13:00 UTC is 14:00 BST
			End:         "2026-04-18T17:00:00+01:00",
			Location:    "Main green",
			Description: "Bowls provided.",
		},
		{
			ID:     "allday1",
			Title:  "Club competition",
			Start:  "2026-04-25",
			End:    "2026-04-25", // Google's exclusive end (the 26th) becomes the last day
			AllDay: true,
		},
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Errorf("events mismatch (-want +got):\n%s", diff)
	}
}

func TestCalendarUpcomingGoogleError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "quota exceeded", http.StatusForbidden)
	}))
	defer srv.Close()

	cal := NewCalendar(srv.Client(), testCalendarID, testAPIKey, WithBaseURL(srv.URL))
	if _, err := cal.Upcoming(t.Context(), time.Now(), 10); err == nil {
		t.Fatal("expected an error for a 403 from Google")
	}
}
