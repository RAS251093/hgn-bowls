package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Snapshots stores the events snapshot. It satisfies events.SnapshotStore.
type Snapshots struct {
	q *Queries
}

// NewSnapshots returns a snapshot store using q.
func NewSnapshots(q *Queries) *Snapshots {
	return &Snapshots{q: q}
}

// SaveEventsSnapshot replaces the saved snapshot.
func (s *Snapshots) SaveEventsSnapshot(ctx context.Context, data []byte, fetchedAt time.Time) error {
	err := s.q.SaveEventsSnapshot(ctx, SaveEventsSnapshotParams{
		Data:      string(data),
		FetchedAt: fetchedAt.UTC().Format(time.RFC3339Nano),
	})
	if err != nil {
		return fmt.Errorf("saving events snapshot: %w", err)
	}
	return nil
}

// LoadEventsSnapshot returns the saved snapshot, or nil data if there is none.
func (s *Snapshots) LoadEventsSnapshot(ctx context.Context) ([]byte, time.Time, error) {
	row, err := s.q.LoadEventsSnapshot(ctx)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, time.Time{}, nil
	}
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("loading events snapshot: %w", err)
	}
	at, err := time.Parse(time.RFC3339Nano, row.FetchedAt)
	if err != nil {
		return nil, time.Time{}, fmt.Errorf("parsing snapshot time: %w", err)
	}
	return []byte(row.Data), at, nil
}
