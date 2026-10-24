package events

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
	_ "time/tzdata" // embed time-zone data so Europe/London always loads
)

const defaultBaseURL = "https://www.googleapis.com/calendar/v3"

var london = mustLoadLocation("Europe/London")

type Event struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	Start       string `json:"start"`
	End         string `json:"end"`
	AllDay      bool   `json:"allDay"`
	Location    string `json:"location"`
	Description string `json:"description"`
}

type Calendar struct {
	client     *http.Client
	baseURL    string
	calendarID string
	apiKey     string
}

type Option func(*Calendar)

func WithBaseURL(u string) Option {
	return func(c *Calendar) { c.baseURL = u }
}

func NewCalendar(client *http.Client, calendarID, apiKey string, opts ...Option) *Calendar {
	c := &Calendar{
		client:     client,
		baseURL:    defaultBaseURL,
		calendarID: calendarID,
		apiKey:     apiKey,
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Calendar) Upcoming(ctx context.Context, now time.Time, max int) ([]Event, error) {
	q := url.Values{
		"key":          {c.apiKey},
		"singleEvents": {"true"},
		"orderBy":      {"startTime"},
		"timeMin":      {now.UTC().Format(time.RFC3339)},
		"maxResults":   {strconv.Itoa(max)},
	}
	u := c.baseURL + "/calendars/" + url.PathEscape(c.calendarID) + "/events?" + q.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, fmt.Errorf("building calendar request: %w", err)
	}
	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling google calendar: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return nil, fmt.Errorf("google calendar returned %d: %s", resp.StatusCode, body)
	}

	var page struct {
		Items []googleEvent `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&page); err != nil {
		return nil, fmt.Errorf("decoding calendar response: %w", err)
	}

	events := make([]Event, 0, len(page.Items))
	for _, item := range page.Items {
		ev, err := item.toEvent()
		if err != nil {
			return nil, fmt.Errorf("event %s: %w", item.ID, err)
		}
		events = append(events, ev)
	}
	return events, nil
}

type googleEvent struct {
	ID          string    `json:"id"`
	Summary     string    `json:"summary"`
	Location    string    `json:"location"`
	Description string    `json:"description"`
	Start       eventTime `json:"start"`
	End         eventTime `json:"end"`
}

type eventTime struct {
	DateTime string `json:"dateTime"`
	Date     string `json:"date"`
}

func (g *googleEvent) toEvent() (Event, error) {
	ev := Event{
		ID:          g.ID,
		Title:       g.Summary,
		Location:    g.Location,
		Description: plainText(g.Description),
	}

	if g.Start.Date != "" { // all-day
		end, err := time.Parse(time.DateOnly, g.End.Date)
		if err != nil {
			return Event{}, fmt.Errorf("parsing end date: %w", err)
		}
		ev.AllDay = true
		ev.Start = g.Start.Date
		ev.End = end.AddDate(0, 0, -1).Format(time.DateOnly) // Google's end date is exclusive
		return ev, nil
	}

	start, err := time.Parse(time.RFC3339, g.Start.DateTime)
	if err != nil {
		return Event{}, fmt.Errorf("parsing start: %w", err)
	}
	end, err := time.Parse(time.RFC3339, g.End.DateTime)
	if err != nil {
		return Event{}, fmt.Errorf("parsing end: %w", err)
	}
	ev.Start = start.In(london).Format(time.RFC3339)
	ev.End = end.In(london).Format(time.RFC3339)
	return ev, nil
}

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		panic(err) // can't happen: time/tzdata is embedded
	}
	return loc
}
