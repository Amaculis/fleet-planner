package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

// CreateAssignment copies the trip's window from the trips row inside the INSERT.
// A violation of assignments_no_bus_overlap / assignments_no_driver_overlap surfaces as
// domain.ErrTimeConflict (see translate) — the database, not this code, is what makes
// double-booking impossible.
func (r *Repo) CreateAssignment(ctx context.Context, tripID, busID, driverID int64) (domain.Assignment, error) {
	row, err := r.q.CreateAssignment(ctx, sqlcgen.CreateAssignmentParams{
		TripID:   tripID,
		BusID:    busID,
		DriverID: driverID,
	})
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("creating assignment: %w", translate(err))
	}
	return assignmentFromRow(sqlcgen.Assignment(row)), nil
}

func (r *Repo) GetAssignmentForTrip(ctx context.Context, tripID int64) (domain.Assignment, error) {
	row, err := r.q.GetAssignmentForTrip(ctx, tripID)
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("getting assignment: %w", translate(err))
	}
	return assignmentFromRow(sqlcgen.Assignment(row)), nil
}

func (r *Repo) DeleteAssignmentForTrip(ctx context.Context, tripID int64) error {
	n, err := r.q.DeleteAssignmentForTrip(ctx, tripID)
	if err != nil {
		return fmt.Errorf("deleting assignment: %w", translate(err))
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

// FindBusConflicts and FindDriverConflicts power the friendly pre-check. excludeTripID
// lets a trip ignore its own existing assignment when it is being re-assigned; pass 0
// when creating a new one.
func (r *Repo) FindBusConflicts(ctx context.Context, busID int64, start, end time.Time, excludeTripID int64) ([]domain.Conflict, error) {
	rows, err := r.q.FindBusConflicts(ctx, sqlcgen.FindBusConflictsParams{
		BusID:         busID,
		ExcludeTripID: excludeTripID,
		WindowStart:   start,
		WindowEnd:     end,
	})
	if err != nil {
		return nil, fmt.Errorf("finding bus conflicts: %w", translate(err))
	}
	conflicts := make([]domain.Conflict, 0, len(rows))
	for _, row := range rows {
		conflicts = append(conflicts, domain.Conflict{
			TripID:      row.TripID,
			Origin:      row.Origin,
			Destination: row.Destination,
			Start:       row.ScheduledStart,
			End:         row.ScheduledEnd,
		})
	}
	return conflicts, nil
}

func (r *Repo) FindDriverConflicts(ctx context.Context, driverID int64, start, end time.Time, excludeTripID int64) ([]domain.Conflict, error) {
	rows, err := r.q.FindDriverConflicts(ctx, sqlcgen.FindDriverConflictsParams{
		DriverID:      driverID,
		ExcludeTripID: excludeTripID,
		WindowStart:   start,
		WindowEnd:     end,
	})
	if err != nil {
		return nil, fmt.Errorf("finding driver conflicts: %w", translate(err))
	}
	conflicts := make([]domain.Conflict, 0, len(rows))
	for _, row := range rows {
		conflicts = append(conflicts, domain.Conflict{
			TripID:      row.TripID,
			Origin:      row.Origin,
			Destination: row.Destination,
			Start:       row.ScheduledStart,
			End:         row.ScheduledEnd,
		})
	}
	return conflicts, nil
}

func (r *Repo) ListAssignmentsInRange(ctx context.Context, from, to time.Time) ([]domain.AssignmentDetail, error) {
	rows, err := r.q.ListAssignmentsInRange(ctx, sqlcgen.ListAssignmentsInRangeParams{
		RangeStart: from,
		RangeEnd:   to,
	})
	if err != nil {
		return nil, fmt.Errorf("listing assignments: %w", translate(err))
	}
	details := make([]domain.AssignmentDetail, 0, len(rows))
	for _, row := range rows {
		details = append(details, domain.AssignmentDetail{
			Assignment: domain.Assignment{
				ID:             row.ID,
				TripID:         row.TripID,
				BusID:          row.BusID,
				DriverID:       row.DriverID,
				ScheduledStart: row.ScheduledStart,
				ScheduledEnd:   row.ScheduledEnd,
				TripStatus:     domain.TripStatus(row.TripStatus),
			},
			BusPlate:    row.BusPlate,
			DriverName:  row.DriverName,
			Origin:      row.Origin,
			Destination: row.Destination,
		})
	}
	return details, nil
}

// NextAssignmentStart returns the earliest non-cancelled booking starting on or after
// after. Returns domain.ErrNotFound when nothing is scheduled that far out.
func (r *Repo) NextAssignmentStart(ctx context.Context, after time.Time) (time.Time, error) {
	start, err := r.q.NextAssignmentStart(ctx, after)
	if err != nil {
		return time.Time{}, fmt.Errorf("finding next assignment: %w", translate(err))
	}
	return start, nil
}
