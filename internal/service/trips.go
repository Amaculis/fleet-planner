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

// TripService owns trips: planning (dispatchers, admins) and the driver's own
// start/finish actions.
type TripService struct {
	repo *repo.Repo
	log  *slog.Logger
}

func NewTripService(r *repo.Repo, log *slog.Logger) *TripService {
	return &TripService{repo: r, log: log}
}

func (s *TripService) ListInRange(ctx context.Context, actor domain.Identity, from, to time.Time) ([]domain.Trip, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	return s.repo.ListTripsInRange(ctx, from.UTC(), to.UTC())
}

func (s *TripService) Get(ctx context.Context, actor domain.Identity, id int64) (domain.Trip, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Trip{}, err
	}
	return s.repo.GetTrip(ctx, id)
}

func (s *TripService) Create(ctx context.Context, actor domain.Identity, input domain.Trip, meta Meta) (domain.Trip, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Trip{}, err
	}
	input, err := validateTrip(input)
	if err != nil {
		return domain.Trip{}, err
	}

	var created domain.Trip
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		if err := checkTripClient(ctx, tx, input.ClientID, nil); err != nil {
			return err
		}
		trip, err := tx.CreateTrip(ctx, input)
		if err != nil {
			return err
		}
		created = trip
		return NewAuditor(tx).Record(ctx, &actor.UserID, "create", "trip", &trip.ID, nil, snapshotTrip(trip), meta)
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("creating trip: %w", err)
	}
	return created, nil
}

// Update also reschedules. If the trip is assigned, the new window cascades onto the
// assignment and the EXCLUDE constraints re-check it, so moving a trip on top of another
// booking fails with domain.ErrTimeConflict — the same guarantee as assigning.
func (s *TripService) Update(ctx context.Context, actor domain.Identity, input domain.Trip, meta Meta) (domain.Trip, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Trip{}, err
	}
	input, err := validateTrip(input)
	if err != nil {
		return domain.Trip{}, err
	}

	var updated domain.Trip
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetTrip(ctx, input.ID)
		if err != nil {
			return err
		}
		if before.Status == domain.TripCompleted {
			return Message(domain.ErrConflict, "error.trip_completed_locked")
		}
		if err := checkTripClient(ctx, tx, input.ClientID, before.ClientID); err != nil {
			return err
		}
		// Rescheduling an assigned trip: warn with details before the constraint fires.
		// A cancelled trip holds no booking (the EXCLUDE constraints skip it), so it
		// cannot clash; un-cancelling re-checks the slot instead.
		rescheduled := !before.ScheduledStart.Equal(input.ScheduledStart) || !before.ScheduledEnd.Equal(input.ScheduledEnd)
		if rescheduled && before.Status != domain.TripCancelled {
			if err := s.checkRescheduleConflicts(ctx, tx, input); err != nil {
				return err
			}
		}
		after, err := tx.UpdateTrip(ctx, input)
		if err != nil {
			return err
		}
		updated = after
		return NewAuditor(tx).Record(ctx, &actor.UserID, "update", "trip", &after.ID,
			snapshotTrip(before), snapshotTrip(after), meta)
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("updating trip: %w", err)
	}
	return updated, nil
}

// checkRescheduleConflicts turns "this slot is taken" into a message naming the trip in
// the way, for a trip that already has a bus and driver.
func (s *TripService) checkRescheduleConflicts(ctx context.Context, tx *repo.Repo, trip domain.Trip) error {
	assignment, err := tx.GetAssignmentForTrip(ctx, trip.ID)
	if errors.Is(err, domain.ErrNotFound) {
		return nil // unassigned trips cannot clash
	}
	if err != nil {
		return err
	}
	busConflicts, err := tx.FindBusConflicts(ctx, assignment.BusID, trip.ScheduledStart, trip.ScheduledEnd, trip.ID)
	if err != nil {
		return err
	}
	if len(busConflicts) > 0 {
		// Name the bus in the message, not just "a bus".
		bus, err := tx.GetBus(ctx, assignment.BusID)
		if err != nil {
			return err
		}
		return &ConflictError{Subject: "bus", Name: bus.Plate, Conflicts: busConflicts}
	}
	driverConflicts, err := tx.FindDriverConflicts(ctx, assignment.DriverID, trip.ScheduledStart, trip.ScheduledEnd, trip.ID)
	if err != nil {
		return err
	}
	if len(driverConflicts) > 0 {
		driver, err := tx.GetDriver(ctx, assignment.DriverID)
		if err != nil {
			return err
		}
		return &ConflictError{Subject: "driver", Name: driver.FullName, Conflicts: driverConflicts}
	}
	return nil
}

// SetStatus moves a trip through its lifecycle, rejecting transitions that make no
// sense (see domain.CanTransitionTrip).
func (s *TripService) SetStatus(ctx context.Context, actor domain.Identity, id int64, status domain.TripStatus, meta Meta) (domain.Trip, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return domain.Trip{}, err
	}
	if _, ok := domain.ParseTripStatus(string(status)); !ok {
		return domain.Trip{}, unknownValue("field.status")
	}

	var updated domain.Trip
	err := s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetTrip(ctx, id)
		if err != nil {
			return err
		}
		if !domain.CanTransitionTrip(before.Status, status) {
			return Message(domain.ErrValidation, "error.invalid_transition")
		}
		actualStart, actualEnd, err := actualsForTransition(before, status, time.Now().UTC())
		if err != nil {
			return err
		}
		// Un-cancelling puts the bus and driver back into the exclusion window; the
		// constraint decides, and the error comes back as ErrTimeConflict.
		after, err := tx.SetTripStatus(ctx, id, status, actualStart, actualEnd)
		if err != nil {
			return err
		}
		updated = after
		return NewAuditor(tx).Record(ctx, &actor.UserID, "status", "trip", &id,
			snapshotTrip(before), snapshotTrip(after), meta)
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("setting trip status: %w", err)
	}
	return updated, nil
}

// actualsForTransition keeps the worked-time window (payroll-seam) consistent with the
// status a planner sets by hand, so a trip can never end up "completed" with no times,
// or "planned" with a start recorded (both also rejected by DB CHECKs):
//
//	planned     -> in_progress : start now (the planner starts it on the driver's behalf)
//	in_progress -> completed   : end now; needs a recorded start
//	in_progress -> cancelled   : end now if it had started — the time driven is kept
//	completed   -> in_progress : clear the end, so the trip can be finished again
//	cancelled   -> planned     : clear both; the trip is back to not having run
func actualsForTransition(before domain.Trip, to domain.TripStatus, now time.Time) (start, end *time.Time, err error) {
	start, end = before.ActualStart, before.ActualEnd
	switch {
	case before.Status == domain.TripPlanned && to == domain.TripInProgress:
		start, end = &now, nil
	case before.Status == domain.TripInProgress && to == domain.TripCompleted:
		if start == nil {
			return nil, nil, Message(domain.ErrConflict, "error.trip_never_started")
		}
		end = &now
	case before.Status == domain.TripInProgress && to == domain.TripCancelled:
		if start != nil {
			end = &now
		}
	case before.Status == domain.TripCompleted && to == domain.TripInProgress:
		end = nil
	case before.Status == domain.TripCancelled && to == domain.TripPlanned:
		start, end = nil, nil
	}
	if start != nil && end != nil && !end.After(*start) {
		// A start recorded this very instant: nudge so the CHECK (end > start) holds.
		later := start.Add(time.Second)
		end = &later
	}
	return start, end, nil
}

// Delete removes a trip that was never assigned. Anything with history is cancelled
// instead, so the audit trail and any worked time keep their referents.
func (s *TripService) Delete(ctx context.Context, actor domain.Identity, id int64, meta Meta) error {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return err
	}
	err := s.repo.InTx(ctx, func(tx *repo.Repo) error {
		before, err := tx.GetTrip(ctx, id)
		if err != nil {
			return err
		}
		if before.Status == domain.TripCompleted || before.Status == domain.TripInProgress {
			return Message(domain.ErrConflict, "error.trip_has_run")
		}
		if err := tx.DeleteTrip(ctx, id); err != nil {
			return err
		}
		return NewAuditor(tx).Record(ctx, &actor.UserID, "delete", "trip", &id, snapshotTrip(before), nil, meta)
	})
	if err != nil {
		if errors.Is(err, domain.ErrValidation) {
			return Message(domain.ErrConflict, "error.trip_assigned")
		}
		return fmt.Errorf("deleting trip: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Driver-facing operations
// ---------------------------------------------------------------------------

// ListMine returns the signed-in driver's own trips. The driver id comes from the
// session; there is no parameter through which another driver's id could arrive.
func (s *TripService) ListMine(ctx context.Context, actor domain.Identity, since time.Time) ([]domain.DriverTrip, error) {
	driverID, err := driverIDOf(actor)
	if err != nil {
		return nil, err
	}
	return s.repo.ListTripsForDriver(ctx, driverID, since.UTC())
}

// StartMine records actual_start for one of the caller's own trips (payroll-seam).
// The driver id is part of the UPDATE's WHERE clause, so a trip id belonging to someone
// else matches no row and comes back as ErrNotFound.
func (s *TripService) StartMine(ctx context.Context, actor domain.Identity, tripID int64, meta Meta) (domain.Trip, error) {
	return s.driverTransition(ctx, actor, tripID, "start", meta,
		func(tx *repo.Repo, driverID int64) (domain.Trip, error) {
			return tx.StartTripAsDriver(ctx, tripID, driverID)
		})
}

// FinishMine records actual_end and completes the trip (payroll-seam).
func (s *TripService) FinishMine(ctx context.Context, actor domain.Identity, tripID int64, meta Meta) (domain.Trip, error) {
	return s.driverTransition(ctx, actor, tripID, "finish", meta,
		func(tx *repo.Repo, driverID int64) (domain.Trip, error) {
			return tx.FinishTripAsDriver(ctx, tripID, driverID)
		})
}

func (s *TripService) driverTransition(
	ctx context.Context,
	actor domain.Identity,
	tripID int64,
	action string,
	meta Meta,
	apply func(tx *repo.Repo, driverID int64) (domain.Trip, error),
) (domain.Trip, error) {
	driverID, err := driverIDOf(actor)
	if err != nil {
		return domain.Trip{}, err
	}

	var updated domain.Trip
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		// Read through the same ownership filter, so "before" can never be a trip the
		// caller is not assigned to.
		before, err := tx.GetTripForDriver(ctx, tripID, driverID)
		if err != nil {
			return err
		}

		after, err := apply(tx, driverID)
		if errors.Is(err, domain.ErrNotFound) {
			// The trip is the caller's (read above) but not in the state this action
			// needs — typically a double tap, or the planner changed it meanwhile.
			return driverTransitionRefused(action, before.Status)
		}
		if err != nil {
			return err
		}
		updated = after
		return NewAuditor(tx).Record(ctx, &actor.UserID, action, "trip", &tripID,
			snapshotTrip(before), snapshotTrip(after), meta)
	})
	if err != nil {
		return domain.Trip{}, fmt.Errorf("%sing trip: %w", action, err)
	}
	return updated, nil
}

// driverTransitionRefused names why a start/finish did nothing, instead of "not found".
func driverTransitionRefused(action string, status domain.TripStatus) error {
	switch {
	case action == "start" && status == domain.TripInProgress:
		return Message(domain.ErrConflict, "error.trip_already_started")
	case action == "finish" && status == domain.TripCompleted:
		return Message(domain.ErrConflict, "error.trip_already_finished")
	case status == domain.TripCancelled:
		return Message(domain.ErrConflict, "error.trip_cancelled_by_planner")
	case action == "finish" && status == domain.TripPlanned:
		return Message(domain.ErrConflict, "error.trip_not_started_yet")
	}
	return Message(domain.ErrConflict, "error.invalid_transition")
}

// driverIDOf resolves the caller's driver record from the session. A non-driver, or a
// driver login with no linked record (impossible per the DB CHECK, but checked anyway),
// is forbidden rather than silently treated as "all drivers".
func driverIDOf(actor domain.Identity) (int64, error) {
	if actor.Role != domain.RoleDriver || actor.DriverID == nil {
		return 0, domain.ErrForbidden
	}
	return *actor.DriverID, nil
}
