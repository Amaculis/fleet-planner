package httpapi

import (
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buscompany/bus_fleet/internal/domain"
)

// idParam reads a positive integer path parameter.
func idParam(r *http.Request, name string) (int64, error) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.ErrNotFound
	}
	return id, nil
}

// rangeParams reads the ?from=&to= window used by the planning views, defaulting to a
// week starting today.
func rangeParams(r *http.Request, loc *time.Location) (from, to time.Time) {
	today := time.Now().In(loc)
	from = time.Date(today.Year(), today.Month(), today.Day(), 0, 0, 0, 0, loc)
	if v := r.URL.Query().Get("from"); v != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
			from = parsed
		}
	}
	to = from.AddDate(0, 0, 7)
	if v := r.URL.Query().Get("to"); v != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", v, loc); err == nil && parsed.After(from) {
			to = parsed
		}
	}
	// A hostile ?to= cannot be used to ask for a decade of rows.
	if to.Sub(from) > 90*24*time.Hour {
		to = from.AddDate(0, 0, 90)
	}
	return from.UTC(), to.UTC()
}
