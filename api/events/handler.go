package events

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
)

const (
	defaultLimit = 10
	maxLimit     = 50
)

// upcomingEvents is what the handler needs (a *Cache in production).
type upcomingEvents interface {
	Upcoming(ctx context.Context, limit int) (Result, error)
}

// Handler serves GET /api/events.
type Handler struct {
	events upcomingEvents
	logger *slog.Logger
}

// NewHandler returns a handler serving events from src.
func NewHandler(src upcomingEvents, logger *slog.Logger) *Handler {
	return &Handler{events: src, logger: logger}
}

type response struct {
	Events []Event `json:"events"`
	Stale  bool    `json:"stale"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	limit := defaultLimit
	if s := r.URL.Query().Get("limit"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 {
			writeError(w, http.StatusBadRequest, "limit must be a whole number of at least 1")
			return
		}
		limit = min(n, maxLimit)
	}

	res, err := h.events.Upcoming(r.Context(), limit)
	if err != nil {
		h.logger.Error("fetching events", "err", err)
		status := http.StatusInternalServerError
		if errors.Is(err, ErrUnavailable) {
			status = http.StatusServiceUnavailable
		}
		writeError(w, status, "events are unavailable right now")
		return
	}

	if res.Events == nil {
		res.Events = []Event{}
	}
	w.Header().Set("Cache-Control", "public, max-age=60")
	writeJSON(w, http.StatusOK, response(res))
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, status, map[string]string{"error": msg})
}
