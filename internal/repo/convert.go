package repo

import (
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

// Conversions between sqlc/pgx column types and the domain types. Kept in one file so
// the rest of the repository reads like plain Go.

func dateToTime(d pgtype.Date) *time.Time {
	if !d.Valid {
		return nil
	}
	t := d.Time
	return &t
}

func timeToDate(t *time.Time) pgtype.Date {
	if t == nil {
		return pgtype.Date{}
	}
	return pgtype.Date{Time: *t, Valid: true}
}

// numericToString renders a Postgres numeric without going through float64, so a rate
// like 12.35 stays exact. payroll-seam.
func numericToString(n pgtype.Numeric) *string {
	if !n.Valid {
		return nil
	}
	v, err := n.Value()
	if err != nil {
		return nil
	}
	s, ok := v.(string)
	if !ok {
		return nil
	}
	return &s
}

func stringToNumeric(s *string) (pgtype.Numeric, error) {
	var n pgtype.Numeric
	if s == nil || *s == "" {
		return n, nil // invalid => NULL
	}
	if err := n.Scan(*s); err != nil {
		return n, fmt.Errorf("%w: %q is not a valid decimal amount", domain.ErrValidation, *s)
	}
	return n, nil
}

// payroll-seam: pay_type is a Postgres enum, so sqlc gives a Null* wrapper.
func payTypeToDomain(p sqlcgen.NullPayType) *domain.PayType {
	if !p.Valid {
		return nil
	}
	pt := domain.PayType(p.PayType)
	return &pt
}

func payTypeToDB(p *domain.PayType) sqlcgen.NullPayType {
	if p == nil {
		return sqlcgen.NullPayType{}
	}
	return sqlcgen.NullPayType{PayType: sqlcgen.PayType(*p), Valid: true}
}

func busFromRow(row sqlcgen.Bus) domain.Bus {
	return domain.Bus{
		ID:               row.ID,
		Plate:            row.Plate,
		Model:            row.Model,
		Seats:            row.Seats,
		Status:           domain.BusStatus(row.Status),
		InsuranceExpiry:  dateToTime(row.InsuranceExpiry),
		InspectionExpiry: dateToTime(row.InspectionExpiry),
		CreatedAt:        row.CreatedAt,
		UpdatedAt:        row.UpdatedAt,
	}
}

func driverFromRow(row sqlcgen.Driver) domain.Driver {
	return domain.Driver{
		ID:            row.ID,
		FullName:      row.FullName,
		Phone:         row.Phone,
		LicenseNumber: row.LicenseNumber,
		LicenseExpiry: dateToTime(row.LicenseExpiry),
		HourlyRate:    numericToString(row.HourlyRate),
		PayType:       payTypeToDomain(row.PayType),
		IsActive:      row.IsActive,
		AnonymizedAt:  row.AnonymizedAt,
		CreatedAt:     row.CreatedAt,
		UpdatedAt:     row.UpdatedAt,
	}
}

func tripFromRow(row sqlcgen.Trip) domain.Trip {
	return domain.Trip{
		ID:             row.ID,
		Origin:         row.Origin,
		Destination:    row.Destination,
		ScheduledStart: row.ScheduledStart,
		ScheduledEnd:   row.ScheduledEnd,
		ActualStart:    row.ActualStart,
		ActualEnd:      row.ActualEnd,
		Status:         domain.TripStatus(row.Status),
		Notes:          row.Notes,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func assignmentFromRow(row sqlcgen.Assignment) domain.Assignment {
	return domain.Assignment{
		ID:             row.ID,
		TripID:         row.TripID,
		BusID:          row.BusID,
		DriverID:       row.DriverID,
		ScheduledStart: row.ScheduledStart,
		ScheduledEnd:   row.ScheduledEnd,
		TripStatus:     domain.TripStatus(row.TripStatus),
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}
