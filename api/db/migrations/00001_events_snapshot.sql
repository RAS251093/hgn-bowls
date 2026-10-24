-- +goose Up
-- The last good list of upcoming events, served when Google Calendar is down.
CREATE TABLE events_snapshot (
    id         INTEGER PRIMARY KEY CHECK (id = 1), -- only ever one row
    data       TEXT    NOT NULL,                   -- JSON array of events
    fetched_at TEXT    NOT NULL                    -- RFC 3339
);

-- +goose Down
DROP TABLE events_snapshot;
