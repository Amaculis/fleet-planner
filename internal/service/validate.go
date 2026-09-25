package service

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/buscompany/bus_fleet/internal/domain"
)

// Server-side validation for every field that reaches the database. These mirror the DB
// CHECK constraints deliberately: the database is the guarantee, this layer is what
// turns a violation into a message a dispatcher can act on — as a message id, so it
// renders in the user's language (see MessageError).

var (
	platePattern = regexp.MustCompile(`^[A-Z0-9-]{2,16}$`)
	phonePattern = regexp.MustCompile(`^\+?[0-9 ()-]{5,32}$`)
	ratePattern  = regexp.MustCompile(`^[0-9]{1,8}(\.[0-9]{1,2})?$`)
)

// MaxTripDuration guards against a mistyped year turning one trip into a decade-long
// block on a bus. Long coach tours are still comfortably inside it.
const MaxTripDuration = 30 * 24 * time.Hour

// Validation message ids, paired with field ids ("field.plate", "field.origin", ...).
const (
	msgRequired  = "validation.required"
	msgTooLong   = "validation.too_long"
	msgRange     = "validation.range"
	msgFormat    = "validation.format"
	msgUnknown   = "validation.unknown_value"
	msgBadPeriod = "validation.end_before_start"
	msgTooLongTr = "validation.duration_too_long"
)

func required(fieldKey string) error {
	return FieldMessage(domain.ErrValidation, msgRequired, fieldKey)
}

func tooLong(fieldKey string, max int) error {
	return FieldMessage(domain.ErrValidation, msgTooLong, fieldKey, max)
}

func badFormat(fieldKey string) error {
	return FieldMessage(domain.ErrValidation, msgFormat, fieldKey)
}

func outOfRange(fieldKey string, min, max int) error {
	return FieldMessage(domain.ErrValidation, msgRange, fieldKey, min, max)
}

func unknownValue(fieldKey string) error {
	return FieldMessage(domain.ErrValidation, msgUnknown, fieldKey)
}

func validateText(fieldKey, value string, min, max int) (string, error) {
	value = strings.TrimSpace(value)
	n := utf8.RuneCountInString(value)
	if n < min {
		return "", required(fieldKey)
	}
	if n > max {
		return "", tooLong(fieldKey, max)
	}
	return value, nil
}

// NormalizePlate matches the DB CHECK: upper-case, no spaces.
func NormalizePlate(plate string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(plate), " ", ""))
}

func validateBus(b domain.Bus) (domain.Bus, error) {
	b.Plate = NormalizePlate(b.Plate)
	if b.Plate == "" {
		return b, required("field.plate")
	}
	if !platePattern.MatchString(b.Plate) {
		return b, badFormat("field.plate")
	}
	model, err := validateText("field.model", b.Model, 1, 120)
	if err != nil {
		return b, err
	}
	b.Model = model
	if b.Seats < 1 || b.Seats > 120 {
		return b, outOfRange("field.seats", 1, 120)
	}
	if _, ok := domain.ParseBusStatus(string(b.Status)); !ok {
		return b, unknownValue("field.status")
	}
	return b, nil
}

func validateDriver(d domain.Driver) (domain.Driver, error) {
	name, err := validateText("field.full_name", d.FullName, 1, 200)
	if err != nil {
		return d, err
	}
	d.FullName = name

	if d.Phone = trimOptional(d.Phone); d.Phone != nil && !phonePattern.MatchString(*d.Phone) {
		return d, badFormat("field.phone")
	}
	if d.LicenseNumber = trimOptional(d.LicenseNumber); d.LicenseNumber != nil {
		if utf8.RuneCountInString(*d.LicenseNumber) > 64 {
			return d, tooLong("field.license_number", 64)
		}
	}
	// payroll-seam: validated so the data is usable when payroll is built, but nothing
	// reads these fields today.
	if d.HourlyRate = trimOptional(d.HourlyRate); d.HourlyRate != nil && !ratePattern.MatchString(*d.HourlyRate) {
		return d, badFormat("field.hourly_rate")
	}
	if d.PayType != nil {
		if _, ok := domain.ParsePayType(string(*d.PayType)); !ok {
			return d, unknownValue("field.pay_type")
		}
	}
	return d, nil
}

func validateTrip(t domain.Trip) (domain.Trip, error) {
	origin, err := validateText("field.origin", t.Origin, 1, 200)
	if err != nil {
		return t, err
	}
	t.Origin = origin

	destination, err := validateText("field.destination", t.Destination, 1, 200)
	if err != nil {
		return t, err
	}
	t.Destination = destination

	if t.ScheduledStart.IsZero() || t.ScheduledEnd.IsZero() {
		return t, required("field.schedule")
	}
	if !t.ScheduledEnd.After(t.ScheduledStart) {
		return t, FieldMessage(domain.ErrValidation, msgBadPeriod, "field.schedule")
	}
	if t.ScheduledEnd.Sub(t.ScheduledStart) > MaxTripDuration {
		return t, FieldMessage(domain.ErrValidation, msgTooLongTr, "field.schedule", 30)
	}
	if t.Notes != nil {
		if utf8.RuneCountInString(*t.Notes) > 2000 {
			return t, tooLong("field.notes", 2000)
		}
		t.Notes = trimOptional(t.Notes)
	}
	if _, ok := domain.ParsePaymentStatus(string(t.PaymentStatus)); !ok {
		return t, unknownValue("field.payment_status")
	}
	// Times are stored and compared in UTC; the view layer formats them per locale.
	t.ScheduledStart = t.ScheduledStart.UTC()
	t.ScheduledEnd = t.ScheduledEnd.UTC()
	return t, nil
}

// trimOptional trims an optional string and turns an empty result into nil, so "unset"
// and "blank" are the same thing in the database.
func trimOptional(s *string) *string {
	if s == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*s)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}
