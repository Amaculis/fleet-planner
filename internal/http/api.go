package http

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

// writeJSON encodes v as the response body. Every JSON handler in this package goes
// through this one function so the content type and error handling stay consistent.
func (s *Server) writeJSON(w http.ResponseWriter, r *http.Request, status int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		s.log.Error("encoding json response",
			"request_id", RequestIDFrom(r.Context()),
			"path", r.URL.Path,
			"error", err)
	}
}

// readJSON decodes the request body into v. The body is already size-limited by
// MaxBytesReader in SecurityHeaders, so a huge payload fails here rather than
// exhausting memory.
func readJSON(r *http.Request, v any) error {
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// apiError is the JSON shape every failed API call renders. A booking clash's
// conflicting trips ride along the same way the HTML flash banner carries them.
type apiError struct {
	Error     string            `json:"error"`
	Conflicts []domain.Conflict `json:"conflicts,omitempty"`
}

// writeAPIError maps a service error to a status and this shape, logging internal
// detail exactly the way s.abort does for HTML responses — nothing more, nothing less.
func (s *Server) writeAPIError(w http.ResponseWriter, r *http.Request, err error) {
	status, msg := s.failed(r, err)
	s.writeJSON(w, r, status, apiError{Error: msg.Text, Conflicts: msg.Conflicts})
}

// jsonDate parses a "YYYY-MM-DD" string — the same wire format a plain <input
// type="date"> submits and what formDate parses for the HTML forms — into a UTC date.
// An empty string is not an error: it means "unset", same as formDate.
func jsonDate(v, fieldKey string) (*time.Time, error) {
	if v == "" {
		return nil, nil
	}
	t, err := time.Parse("2006-01-02", v)
	if err != nil {
		return nil, service.FieldMessage(domain.ErrValidation, "validation.format", fieldKey)
	}
	return &t, nil
}

// jsonTimestamp parses a "YYYY-MM-DDTHH:MM" (or with seconds) local wall-clock string
// — what a plain <input type="datetime-local"> submits — in the given zone, mirroring
// formTimestamp exactly so a trip's scheduling behaves identically whether it came
// through the server-rendered form or the SPA.
func jsonTimestamp(v, fieldKey string, loc *time.Location) (time.Time, error) {
	if v == "" {
		return time.Time{}, service.FieldMessage(domain.ErrValidation, "validation.required", fieldKey)
	}
	t, err := time.ParseInLocation(service.FormTimeLayout, v, loc)
	if err != nil {
		t, err = time.ParseInLocation("2006-01-02T15:04:05", v, loc)
		if err != nil {
			return time.Time{}, service.FieldMessage(domain.ErrValidation, "validation.format", fieldKey)
		}
	}
	return t.UTC(), nil
}

// formatDate/formatTimestamp render the *time.Time / time.Time fields domain structs
// carry back into the same wire formats jsonDate/jsonTimestamp accept, so a value the
// API returns can always be fed straight back into an update request unchanged.
func formatDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format("2006-01-02")
}

func formatTimestamp(t time.Time, loc *time.Location) string {
	return t.In(loc).Format(service.FormTimeLayout)
}
