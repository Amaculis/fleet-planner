package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

func (r *Repo) ListTripsInRange(ctx context.Context, from, to time.Time) ([]domain.Trip, error) {
	rows, err := r.q.ListTripsInRange(ctx, sqlcgen.ListTripsInRangeParams{
		RangeStart: from,
		RangeEnd:   to,
	})
	if err != nil {
		return nil, fmt.Errorf("listing trips: %w", translate(err))
	}
	trips := make([]domain.Trip, 0, len(rows))
	for _, row := range rows {
		trips = append(trips, tripFromRow(sqlcgen.Trip(row)))
	}
	return trips, nil
}

func (r *Repo) GetTrip(ctx context.Context, id int64) (domain.Trip, error) {
	row, err := r.q.GetTrip(ctx, id)
	if err != nil {
		return domain.Trip{}, fmt.Errorf("getting trip: %w", translate(err))
	}
	return tripFromRow(sqlcgen.Trip(row)), nil
}

func (r *Repo) CreateTrip(ctx context.Context, t domain.Trip) (domain.Trip, error) {
	row, err := r.q.CreateTrip(ctx, sqlcgen.CreateTripParams{
		Origin:         t.Origin,
		Destination:    t.Destination,
		ScheduledStart: t.ScheduledStart,
		ScheduledEnd:   t.ScheduledEnd,
		PaymentStatus:  sqlcgen.PaymentStatus(t.PaymentStatus),
		Notes:          t.Notes,
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("creating trip: %w", translate(err))
	}
	return tripFromRow(sqlcgen.Trip(row)), nil
}

// UpdateTrip can fail with domain.ErrTimeConflict: rescheduling cascades the new window
// onto the assignment, where the EXCLUDE constraints re-check it.
func (r *Repo) UpdateTrip(ctx context.Context, t domain.Trip) (domain.Trip, error) {
	row, err := r.q.UpdateTrip(ctx, sqlcgen.UpdateTripParams{
		ID:             t.ID,
		Origin:         t.Origin,
		Destination:    t.Destination,
		ScheduledStart: t.ScheduledStart,
		ScheduledEnd:   t.ScheduledEnd,
		PaymentStatus:  sqlcgen.PaymentStatus(t.PaymentStatus),
		Notes:          t.Notes,
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("updating trip: %w", translate(err))
	}
	return tripFromRow(sqlcgen.Trip(row)), nil
}

// SetTripStatus can also fail with domain.ErrTimeConflict: un-cancelling a trip puts its
// bus and driver back into the exclusion window.
func (r *Repo) SetTripStatus(ctx context.Context, id int64, status domain.TripStatus) (domain.Trip, error) {
	row, err := r.q.SetTripStatus(ctx, sqlcgen.SetTripStatusParams{
		ID:     id,
		Status: sqlcgen.TripStatus(status),
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("setting trip status: %w", translate(err))
	}
	return tripFromRow(sqlcgen.Trip(row)), nil
}

func (r *Repo) DeleteTrip(ctx context.Context, id int64) error {
	n, err := r.q.DeleteTrip(ctx, id)
	if err != nil {
		return fmt.Errorf("deleting trip: %w", translate(err))
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// ListTripsForDriver returns only the given driver's trips. The caller passes the id
// from the session; no query in this file accepts a driver id from a request.
func (r *Repo) ListTripsForDriver(ctx context.Context, driverID int64, since time.Time) ([]domain.DriverTrip, error) {
	rows, err := r.q.ListTripsForDriver(ctx, sqlcgen.ListTripsForDriverParams{
		DriverID: driverID,
		Since:    since,
	})
	if err != nil {
		return nil, fmt.Errorf("listing driver trips: %w", translate(err))
	}
	trips := make([]domain.DriverTrip, 0, len(rows))
	for _, row := range rows {
		trips = append(trips, domain.DriverTrip{
			Trip: driverTripFromRow(driverTripRow{
				ID:             row.ID,
				Origin:         row.Origin,
				Destination:    row.Destination,
				ScheduledStart: row.ScheduledStart,
				ScheduledEnd:   row.ScheduledEnd,
				ActualStart:    row.ActualStart,
				ActualEnd:      row.ActualEnd,
				Status:         row.Status,
				Notes:          row.Notes,
				CreatedAt:      row.CreatedAt,
				UpdatedAt:      row.UpdatedAt,
			}),
			BusPlate: row.BusPlate,
		})
	}
	return trips, nil
}

// GetTripForDriver returns a trip only if it is assigned to this driver.
func (r *Repo) GetTripForDriver(ctx context.Context, tripID, driverID int64) (domain.Trip, error) {
	row, err := r.q.GetTripForDriver(ctx, sqlcgen.GetTripForDriverParams{
		TripID:   tripID,
		DriverID: driverID,
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("getting driver trip: %w", translate(err))
	}
	return driverTripFromRow(driverTripRow(row)), nil
}

// StartTripAsDriver and FinishTripAsDriver match the driver id inside the UPDATE, so a
// forged trip id simply matches no row — another driver's trip can never be touched.
// domain.ErrNotFound therefore covers both "no such trip" and "not yours", which is
// also what the client should be told.
func (r *Repo) StartTripAsDriver(ctx context.Context, tripID, driverID int64) (domain.Trip, error) {
	row, err := r.q.StartTripAsDriver(ctx, sqlcgen.StartTripAsDriverParams{
		TripID:   tripID,
		DriverID: driverID,
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("starting trip: %w", translate(err))
	}
	return driverTripFromRow(driverTripRow(row)), nil
}

func (r *Repo) FinishTripAsDriver(ctx context.Context, tripID, driverID int64) (domain.Trip, error) {
	row, err := r.q.FinishTripAsDriver(ctx, sqlcgen.FinishTripAsDriverParams{
		TripID:   tripID,
		DriverID: driverID,
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("finishing trip: %w", translate(err))
	}
	return driverTripFromRow(driverTripRow(row)), nil
}
