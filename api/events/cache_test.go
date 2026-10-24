package events

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"
)

// fakeSource counts calls and returns fixed events, or an error.
type fakeSource struct {
	calls  atomic.Int32
	mu     sync.Mutex
	events []Event
	err    error
	delay  time.Duration
}

func (f *fakeSource) Upcoming(ctx context.Context, now time.Time, max int) ([]Event, error) {
	f.calls.Add(1)
	time.Sleep(f.delay) // fake time inside synctest: lets concurrent callers pile up
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return nil, f.err
	}
	return f.events, nil
}

func (f *fakeSource) fail(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.err = err
}

// fakeSnapshots is an in-memory SnapshotStore.
type fakeSnapshots struct {
	mu    sync.Mutex
	data  []byte
	saved time.Time
	saves int
}

func (f *fakeSnapshots) SaveEventsSnapshot(ctx context.Context, data []byte, fetchedAt time.Time) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.data, f.saved = data, fetchedAt
	f.saves++
	return nil
}

func (f *fakeSnapshots) LoadEventsSnapshot(ctx context.Context) ([]byte, time.Time, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.data, f.saved, nil
}

func threeEvents() []Event {
	return []Event{{ID: "a"}, {ID: "b"}, {ID: "c"}}
}

func TestCacheServesFromMemoryWithinTTL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{events: threeEvents()}
		c := NewCache(src, time.Minute)

		for range 3 {
			if _, err := c.Upcoming(t.Context(), 10); err != nil {
				t.Fatal(err)
			}
			time.Sleep(10 * time.Second)
		}
		if got := src.calls.Load(); got != 1 {
			t.Errorf("source called %d times within the TTL, want 1", got)
		}
	})
}

func TestCacheRefreshesAfterTTL(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{events: threeEvents()}
		c := NewCache(src, time.Minute)

		mustUpcoming(t, c)
		time.Sleep(61 * time.Second)
		mustUpcoming(t, c)

		if got := src.calls.Load(); got != 2 {
			t.Errorf("source called %d times, want 2 (one refresh after the TTL)", got)
		}
	})
}

func TestCacheConcurrentMissesShareOneCall(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{events: threeEvents(), delay: time.Second}
		c := NewCache(src, time.Minute)

		var wg sync.WaitGroup
		for range 10 {
			wg.Go(func() { mustUpcoming(t, c) })
		}
		wg.Wait()

		if got := src.calls.Load(); got != 1 {
			t.Errorf("source called %d times for 10 simultaneous misses, want 1", got)
		}
	})
}

func TestCacheDoesNotCacheErrors(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{err: errors.New("google is down")}
		c := NewCache(src, time.Minute)

		if _, err := c.Upcoming(t.Context(), 10); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("err = %v, want ErrUnavailable", err)
		}
		_, _ = c.Upcoming(t.Context(), 10) // fails again; only the call count matters

		if got := src.calls.Load(); got != 2 {
			t.Errorf("source called %d times, want 2 (an error must not be cached)", got)
		}
	})
}

func TestCacheAppliesLimit(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		c := NewCache(&fakeSource{events: threeEvents()}, time.Minute)

		got, err := c.Upcoming(t.Context(), 2)
		if err != nil {
			t.Fatal(err)
		}
		want := Result{Events: []Event{{ID: "a"}, {ID: "b"}}}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("result mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestCacheSavesSnapshotOnlyWhenEventsChange(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{events: threeEvents()}
		snaps := &fakeSnapshots{}
		c := NewCache(src, time.Minute, WithSnapshots(snaps))

		mustUpcoming(t, c)
		time.Sleep(61 * time.Second)
		mustUpcoming(t, c) // same events again

		if snaps.saves != 1 {
			t.Errorf("snapshot saved %d times, want 1 (unchanged events are not re-saved)", snaps.saves)
		}
	})
}

func TestCacheFallsBackToSnapshotWithoutFinishedEvents(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		// synctest's clock starts at 2000-01-01 00:00 UTC.
		now := time.Now()
		past := Event{ID: "past", Start: now.Add(-3 * time.Hour).Format(time.RFC3339), End: now.Add(-time.Hour).Format(time.RFC3339)}
		running := Event{ID: "running", Start: now.Add(-time.Hour).Format(time.RFC3339), End: now.Add(time.Hour).Format(time.RFC3339)}
		today := Event{ID: "today", AllDay: true, Start: now.Format(time.DateOnly), End: now.Format(time.DateOnly)}
		data, err := json.Marshal([]Event{past, running, today})
		if err != nil {
			t.Fatal(err)
		}
		snaps := &fakeSnapshots{data: data, saved: now.Add(-24 * time.Hour)}
		src := &fakeSource{err: errors.New("google is down")}
		c := NewCache(src, time.Minute, WithSnapshots(snaps))

		got, err := c.Upcoming(t.Context(), 10)
		if err != nil {
			t.Fatalf("expected the snapshot to be served, got %v", err)
		}
		want := Result{Events: []Event{running, today}, Stale: true}
		if diff := cmp.Diff(want, got); diff != "" {
			t.Errorf("result mismatch (-want +got):\n%s", diff)
		}
	})
}

func TestCacheUnavailableWithNoSnapshot(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{err: errors.New("google is down")}
		c := NewCache(src, time.Minute, WithSnapshots(&fakeSnapshots{}))

		if _, err := c.Upcoming(t.Context(), 10); !errors.Is(err, ErrUnavailable) {
			t.Fatalf("err = %v, want ErrUnavailable", err)
		}
	})
}

func TestCacheRecoversAfterGoogleComesBack(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		src := &fakeSource{err: errors.New("google is down")}
		c := NewCache(src, time.Minute)
		_, _ = c.Upcoming(t.Context(), 10) // fails while Google is down

		src.fail(nil)
		src.mu.Lock()
		src.events = threeEvents()
		src.mu.Unlock()

		got, err := c.Upcoming(t.Context(), 10)
		if err != nil {
			t.Fatal(err)
		}
		if got.Stale || len(got.Events) != 3 {
			t.Errorf("got %+v, want 3 fresh events", got)
		}
	})
}

func mustUpcoming(t *testing.T, c *Cache) {
	t.Helper()
	if _, err := c.Upcoming(t.Context(), 10); err != nil {
		t.Errorf("upcoming: %v", err)
	}
}
