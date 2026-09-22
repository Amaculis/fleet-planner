package service

import (
	"fmt"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
)

// Layouts used for audit snapshots and form parsing. The view layer formats times per
// locale; everything below it works in UTC.
const (
	timestampLayout = time.RFC3339
	dateLayout      = "2006-01-02"
	// FormTimeLayout is what <input type="datetime-local"> submits.
	FormTimeLayout = "2006-01-02T15:04"
)

func formatDatePtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.Format(dateLayout)
	return &s
}

func formatTimestampPtr(t *time.Time) *string {
	if t == nil {
		return nil
	}
	s := t.UTC().Format(timestampLayout)
	return &s
}

// ConflictError reports which existing bookings block a proposed assignment. It wraps
// domain.ErrTimeConflict, so callers can either branch on the sentinel or show details.
//
// The service produces this by querying for overlaps *before* inserting. That is a
// courtesy, not the guarantee: two concurrent requests can both pass this check, and the
// EXCLUDE constraints in Postgres are what actually make double-booking impossible. The
// same error type is returned when the constraint fires.
type ConflictError struct {
	Subject   string // "bus" or "driver"
	Name      string // plate or driver name, for the message
	Conflicts []domain.Conflict
}

func (e *ConflictError) Error() string {
	return fmt.Sprintf("%s %q is already booked for %d overlapping trip(s)", e.Subject, e.Name, len(e.Conflicts))
}

func (e *ConflictError) Unwrap() error { return domain.ErrTimeConflict }

// MessageKey is the i18n id for the headline; the overlapping trips themselves are data
// and get rendered as a list under it.
func (e *ConflictError) MessageKey() string {
	if e.Subject == "driver" {
		return "error.driver_conflict"
	}
	return "error.bus_conflict"
}
