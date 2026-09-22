package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo"
)

// AssignmentService links a trip to one bus and one driver.
//
// Conflict prevention is two-layered on purpose:
//
//  1. This service queries for overlapping bookings first, so the dispatcher gets
//     "bus AA-1111 is already on Riga → Liepāja 08:00–12:00" instead of a constraint name.
//  2. Postgres refuses the INSERT anyway if the window overlaps
//     (assignments_no_bus_overlap / assignments_no_driver_overlap). Two dispatchers
//     racing on the same slot both pass step 1; only one survives step 2.
//
// Step 2 is the guarantee. Step 1 is the manners.
type AssignmentService struct {
	repo *repo.Repo
	log  *slog.Logger
}

func NewAssignmentService(r *repo.Repo, log *slog.Logger) *AssignmentService {
	return &AssignmentService{repo: r, log: log}
}

func (s *AssignmentService) ListInRange(ctx context.Context, actor domain.Identity, from, to time.Time) ([]domain.AssignmentDetail, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	return s.repo.ListAssignmentsInRange(ctx, from.UTC(), to.UTC())
}

func (s *AssignmentService) GetForTrip(ctx context.Context, actor domain.Identity, tripID int64) (domain.Assignment, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Assignment{}, err
	}
	return s.repo.GetAssignmentForTrip(ctx, tripID)
}

// NextUpcoming answers "when is the next trip after this day?" for the timeline's empty
// state. A nil result (no error) means nothing is scheduled that far out, not a failure.
func (s *AssignmentService) NextUpcoming(ctx context.Context, actor domain.Identity, after time.Time) (*time.Time, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	start, err := s.repo.NextAssignmentStart(ctx, after)
	if errors.Is(err, domain.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &start, nil
}

// Assign books a bus and a driver onto a trip, replacing any existing assignment.
func (s *AssignmentService) Assign(ctx context.Context, actor domain.Identity, tripID, busID, driverID int64, meta Meta) (domain.Assignment, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Assignment{}, err
	}

	var created domain.Assignment
	err := s.repo.InTx(ctx, func(tx *repo.Repo) error {
		trip, err := tx.GetTrip(ctx, tripID)
		if err != nil {
			return err
		}
		switch trip.Status {
		case domain.TripCompleted:
			return Message(domain.ErrConflict, "error.trip_completed_reassign")
		case domain.TripCancelled:
			return Message(domain.ErrConflict, "error.trip_cancelled_assign")
		}

		bus, err := tx.GetBus(ctx, busID)
		if err != nil {
			return err
		}
		if bus.Status != domain.BusActive {
			return Message(domain.ErrValidation, "error.bus_not_active", bus.Plate)
		}

		driver, err := tx.GetDriver(ctx, driverID)
		if err != nil {
			return err
		}
		if !driver.IsActive || driver.IsAnonymized() {
			return Message(domain.ErrValidation, "error.driver_not_active")
		}

		// Friendly pre-check. Ignores this trip's own booking so re-assigning a trip to
		// a different driver does not conflict with itself.
		busConflicts, err := tx.FindBusConflicts(ctx, busID, trip.ScheduledStart, trip.ScheduledEnd, tripID)
		if err != nil {
			return err
		}
		if len(busConflicts) > 0 {
			return &ConflictError{Subject: "bus", Name: bus.Plate, Conflicts: busConflicts}
		}
		driverConflicts, err := tx.FindDriverConflicts(ctx, driverID, trip.ScheduledStart, trip.ScheduledEnd, tripID)
		if err != nil {
			return err
		}
		if len(driverConflicts) > 0 {
			return &ConflictError{Subject: "driver", Name: driver.FullName, Conflicts: driverConflicts}
		}

		// Re-assignment: drop the old row first. The trigger refuses this for a
		// completed trip, which the status check above has already excluded.
		var before any
		if existing, err := tx.GetAssignmentForTrip(ctx, tripID); err == nil {
			before = snapshotAssignment(existing)
			if err := tx.DeleteAssignmentForTrip(ctx, tripID); err != nil {
				return err
			}
		} else if !errors.Is(err, domain.ErrNotFound) {
			return err
		}

		// The insert copies the trip's window itself; if the EXCLUDE constraint fires
		// here (a concurrent booking slipped in), translate() turns it into
		// domain.ErrTimeConflict.
		assignment, err := tx.CreateAssignment(ctx, tripID, busID, driverID)
		if err != nil {
			return err
		}
		created = assignment

		action := "create"
		if before != nil {
			action = "update"
		}
		return NewAuditor(tx).Record(ctx, &actor.UserID, action, "assignment", &assignment.ID,
			before, snapshotAssignment(assignment), meta)
	})
	if err != nil {
		return domain.Assignment{}, fmt.Errorf("assigning trip: %w", err)
	}
	return created, nil
}

// Unassign frees the bus and driver. Refused for a completed trip by the
// assignments_lock_completed trigger: payroll must keep knowing who drove it.
func (s *AssignmentService) Unassign(ctx context.Context, actor domain.Identity, tripID int64, meta Meta) error {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return err
	}
	err := s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetAssignmentForTrip(ctx, tripID)
		if err != nil {
			return err
		}
		if before.TripStatus == domain.TripCompleted {
			return Message(domain.ErrConflict, "error.completed_keeps_driver")
		}
		if err := tx.DeleteAssignmentForTrip(ctx, tripID); err != nil {
			return err
		}
		return NewAuditor(tx).Record(ctx, &actor.UserID, "delete", "assignment", &before.ID,
			snapshotAssignment(before), nil, meta)
	})
	if err != nil {
		return fmt.Errorf("unassigning trip: %w", err)
	}
	return nil
}
