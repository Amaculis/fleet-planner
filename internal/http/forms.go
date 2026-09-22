package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

// Form parsing helpers. Every one of them is total: a malformed value becomes a
// validation error or a zero value, never a panic and never a silent pass-through to
// the database.

// idParam reads a positive integer path parameter.
func idParam(r *http.Request, name string) (int64, error) {
	raw := chi.URLParam(r, name)
	id, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || id <= 0 {
		return 0, domain.ErrNotFound
	}
	return id, nil
}

func formString(r *http.Request, name string) string {
	return strings.TrimSpace(r.PostFormValue(name))
}

// formOptional returns nil for an empty field, so "blank" and "unset" are the same.
func formOptional(r *http.Request, name string) *string {
	v := formString(r, name)
	if v == "" {
		return nil
	}
	return &v
}

func formInt(r *http.Request, name, fieldKey string) (int64, error) {
	v := formString(r, name)
	if v == "" {
		return 0, service.FieldMessage(domain.ErrValidation, "validation.required", fieldKey)
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, service.FieldMessage(domain.ErrValidation, "validation.format", fieldKey)
	}
	return n, nil
}

// formDate parses a <input type="date"> value (YYYY-MM-DD) as a UTC date.
func formDate(r *http.Request, name, fieldKey string) (*time.Time, error) {
	v := formString(r, name)
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, service.FieldMessage(domain.ErrValidation, "validation.format", fieldKey)
	}
	return &t, nil
}

// formTimestamp parses a <input type="datetime-local"> value. The browser submits local
// wall-clock time with no zone, so it is interpreted in the configured display zone and
// stored as UTC — the database never sees a naive timestamp.
func formTimestamp(r *http.Request, name, fieldKey string, loc *time.Location) (time.Time, error) {
	v := formString(r, name)
	if v == "" {
		return time.Time{}, service.FieldMessage(domain.ErrValidation, "validation.required", fieldKey)
	}
	t, err := time.ParseInLocation(service.FormTimeLayout, v, loc)
	if err != nil {
		// Some browsers include seconds.
		t, err = time.ParseInLocation("2006-01-02T15:04:05", v, loc)
		if err != nil {
			return time.Time{}, service.FieldMessage(domain.ErrValidation, "validation.format", fieldKey)
		}
	}
	return t.UTC(), nil
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
