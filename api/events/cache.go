package events

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"golang.org/x/sync/singleflight"
)

// fetchMax is how many events are fetched from Google per refresh; callers
// asking for fewer get a slice of the same cached list.
const fetchMax = 50

// ErrUnavailable means no events could be served: Google failed and there
// is no saved snapshot to fall back on.
var ErrUnavailable = errors.New("events unavailable")

// source is where the cache gets events from (a *Calendar in production).
type source interface {
	Upcoming(ctx context.Context, now time.Time, max int) ([]Event, error)
}

// SnapshotStore keeps the last good list of events so they can be served
// when Google is unavailable. LoadEventsSnapshot returns nil data when
// nothing has been saved yet.
type SnapshotStore interface {
	SaveEventsSnapshot(ctx context.Context, data []byte, fetchedAt time.Time) error
	LoadEventsSnapshot(ctx context.Context) (data []byte, fetchedAt time.Time, err error)
}

// Result is a list of events and whether it came from an old snapshot.
type Result struct {
	Events []Event
	Stale  bool
}

// Cache keeps upcoming events in memory for a short time, so a burst of
// visitors makes one call to Google, and falls back to a saved snapshot.
type Cache struct {
	src    source
	ttl    time.Duration
	snaps  SnapshotStore
	logger *slog.Logger
	group  singleflight.Group

	mu        sync.Mutex
	events    []Event
	fetchedAt time.Time
	lastSaved []byte
}

// CacheOption changes how a Cache is built.
type CacheOption func(*Cache)

// WithSnapshots saves each new list of events and serves the last one when
// Google fails.
func WithSnapshots(s SnapshotStore) CacheOption {
	return func(c *Cache) { c.snaps = s }
}

// WithLogger sets where the cache reports problems it recovers from.
func WithLogger(l *slog.Logger) CacheOption {
	return func(c *Cache) { c.logger = l }
}

// NewCache returns a cache in front of src that refreshes after ttl.
func NewCache(src source, ttl time.Duration, opts ...CacheOption) *Cache {
	c := &Cache{src: src, ttl: ttl, logger: slog.New(slog.DiscardHandler)}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// Upcoming returns up to limit upcoming events.
func (c *Cache) Upcoming(ctx context.Context, limit int) (Result, error) {
	if events, ok := c.fresh(); ok {
		return Result{Events: first(events, limit)}, nil
	}

	// One refresh at a time; simultaneous callers share its result. The
	// refresh is detached from this caller's cancellation so one visitor
	// closing their tab doesn't fail everyone waiting on the same call.
	v, err, _ := c.group.Do("refresh", func() (any, error) {
		return c.refresh(context.WithoutCancel(ctx))
	})
	if err == nil {
		return Result{Events: first(v.([]Event), limit)}, nil
	}

	events, ok := c.fromSnapshot(ctx)
	if !ok {
		return Result{}, fmt.Errorf("%w: %w", ErrUnavailable, err)
	}
	c.logger.Warn("serving events snapshot", "err", err)
	return Result{Events: first(events, limit), Stale: true}, nil
}

func (c *Cache) fresh() ([]Event, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fetchedAt.IsZero() || time.Since(c.fetchedAt) >= c.ttl {
		return nil, false
	}
	return c.events, true
}

func (c *Cache) refresh(ctx context.Context) ([]Event, error) {
	now := time.Now()
	events, err := c.src.Upcoming(ctx, now, fetchMax)
	if err != nil {
		return nil, err
	}

	c.mu.Lock()
	c.events, c.fetchedAt = events, now
	c.mu.Unlock()

	c.save(ctx, events, now)
	return events, nil
}

// save writes a snapshot, but only when the events have changed.
func (c *Cache) save(ctx context.Context, events []Event, at time.Time) {
	if c.snaps == nil {
		return
	}
	data, err := json.Marshal(events)
	if err != nil {
		c.logger.Error("encoding events snapshot", "err", err)
		return
	}

	c.mu.Lock()
	unchanged := bytes.Equal(data, c.lastSaved)
	c.mu.Unlock()
	if unchanged {
		return
	}

	if err := c.snaps.SaveEventsSnapshot(ctx, data, at); err != nil {
		c.logger.Error("saving events snapshot", "err", err)
		return
	}
	c.mu.Lock()
	c.lastSaved = data
	c.mu.Unlock()
}

// fromSnapshot returns the saved events that haven't finished yet.
func (c *Cache) fromSnapshot(ctx context.Context) ([]Event, bool) {
	if c.snaps == nil {
		return nil, false
	}
	data, _, err := c.snaps.LoadEventsSnapshot(ctx)
	if err != nil {
		c.logger.Error("loading events snapshot", "err", err)
		return nil, false
	}
	if data == nil {
		return nil, false
	}
	var saved []Event
	if err := json.Unmarshal(data, &saved); err != nil {
		c.logger.Error("decoding events snapshot", "err", err)
		return nil, false
	}

	now := time.Now()
	upcoming := make([]Event, 0, len(saved))
	for _, ev := range saved {
		if end, ok := endsAt(ev); !ok || end.After(now) {
			upcoming = append(upcoming, ev)
		}
	}
	return upcoming, true
}

// endsAt is when an event finishes. All-day events finish at the end of
// their last day in London.
func endsAt(ev Event) (time.Time, bool) {
	if ev.AllDay {
		day, err := time.ParseInLocation(time.DateOnly, ev.End, london)
		if err != nil {
			return time.Time{}, false
		}
		return day.AddDate(0, 0, 1), true
	}
	end, err := time.Parse(time.RFC3339, ev.End)
	if err != nil {
		return time.Time{}, false
	}
	return end, true
}

func first(events []Event, n int) []Event {
	if len(events) > n {
		return events[:n]
	}
	return events
}
