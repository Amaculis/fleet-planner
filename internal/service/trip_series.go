package service

import (
	"fmt"
	"log/slog"
	"time"

	"context"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo"
)

// TripSeriesService generates and bulk-edits recurring trips. A series is a weekly
// pattern described once ("every Monday 8-10am until <date>"); creating one eagerly
// inserts one plain trips row per matching day (see repo.CreateTripSeries /
// seriesOccurrences) — there is no background job and no open-ended recurrence, so
// every trip a user will ever see already exists as an ordinary row from the moment
// the series is created. Editing or cancelling "this and all future trips" therefore
// just bulk-operates on those rows; the trip_series row itself is never edited.
type TripSeriesService struct {
	repo *repo.Repo
	log  *slog.Logger
}

func NewTripSeriesService(r *repo.Repo, log *slog.Logger) *TripSeriesService {
	return &TripSeriesService{repo: r, log: log}
}

type tripSeriesSnapshot struct {
	Origin        string  `json:"origin"`
	Destination   string  `json:"destination"`
	DaysOfWeek    []int32 `json:"days_of_week"`
	FirstStart    string  `json:"first_start"`
	FirstEnd      string  `json:"first_end"`
	EndsOn        string  `json:"ends_on"`
	PaymentStatus string  `json:"payment_status"`
	Notes         *string `json:"notes"`
	TripCount     int     `json:"trip_count"`
	TripIDs       []int64 `json:"trip_ids"`
}

// Create validates the pattern, generates every occurrence between its first trip and
// EndsOn (exclusive), and inserts them all in one transaction. loc resolves which
// calendar day each occurrence's weekday falls on — the same timezone the HTTP layer
// already parses scheduled_start/end in (see jsonTimestamp), threaded through
// explicitly because weekday is inherently a local-calendar concept, not a UTC one.
func (s *TripSeriesService) Create(ctx context.Context, actor domain.Identity, input domain.TripSeries, loc *time.Location, meta Meta) ([]domain.Trip, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	input, err := validateTripSeries(input)
	if err != nil {
		return nil, err
	}
	occurrences := seriesOccurrences(input, loc)
	if len(occurrences) == 0 {
		return nil, Message(domain.ErrValidation, "error.series_no_occurrences")
	}

	var created []domain.Trip
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		series, err := tx.CreateTripSeries(ctx, input)
		if err != nil {
			return err
		}
		for _, occ := range occurrences {
			trip := domain.Trip{
				Origin: series.Origin, Destination: series.Destination,
				ScheduledStart: occ.start, ScheduledEnd: occ.end,
				PaymentStatus: series.PaymentStatus, Notes: series.Notes,
				SeriesID: &series.ID,
			}
			t, err := tx.CreateTrip(ctx, trip)
			if err != nil {
				return err
			}
			created = append(created, t)
		}

		ids := make([]int64, len(created))
		for i, t := range created {
			ids[i] = t.ID
		}
		return NewAuditor(tx).Record(ctx, &actor.UserID, "create", "trip_series", &series.ID, nil, tripSeriesSnapshot{
			Origin: series.Origin, Destination: series.Destination, DaysOfWeek: series.DaysOfWeek,
			FirstStart: series.FirstStart.UTC().Format(timestampLayout),
			FirstEnd:   series.FirstEnd.UTC().Format(timestampLayout),
			EndsOn:     series.EndsOn.UTC().Format(timestampLayout),
			PaymentStatus: string(series.PaymentStatus), Notes: series.Notes,
			TripCount: len(created), TripIDs: ids,
		}, meta)
	})
	if err != nil {
		return nil, fmt.Errorf("creating trip series: %w", err)
	}
	return created, nil
}

// UpdateFuture applies an edit to one occurrence and every later still-planned
// occurrence in the same series, re-anchoring each at its own calendar date but the
// edited time-of-day/duration/other fields — "every Monday 8-10" becomes "every Monday
// 8:30-10:30" without moving which Mondays are in the series. The returned slice's
// first element is always the trip named by input.ID.
func (s *TripSeriesService) UpdateFuture(ctx context.Context, actor domain.Identity, input domain.Trip, loc *time.Location, meta Meta) ([]domain.Trip, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}
	input, err := validateTrip(input)
	if err != nil {
		return nil, err
	}

	var updated []domain.Trip
	err = s.repo.InTx(ctx, func(tx *repo.Repo) error {
		from, err := tx.GetTrip(ctx, input.ID)
		if err != nil {
			return err
		}
		if from.SeriesID == nil {
			return Message(domain.ErrValidation, "error.trip_not_in_series")
		}
		if from.Status == domain.TripCompleted {
			return Message(domain.ErrConflict, "error.trip_completed_locked")
		}
		future, err := tx.ListFutureSeriesTrips(ctx, *from.SeriesID, from.ScheduledStart)
		if err != nil {
			return err
		}

		duration := input.ScheduledEnd.Sub(input.ScheduledStart)
		startLocal := input.ScheduledStart.In(loc)
		hour, min, sec := startLocal.Hour(), startLocal.Minute(), startLocal.Second()

		for _, t := range future {
			next := t
			next.Origin, next.Destination = input.Origin, input.Destination
			next.PaymentStatus, next.Notes = input.PaymentStatus, input.Notes
			if t.ID == from.ID {
				next.ScheduledStart, next.ScheduledEnd = input.ScheduledStart, input.ScheduledEnd
			} else {
				day := t.ScheduledStart.In(loc)
				next.ScheduledStart = time.Date(day.Year(), day.Month(), day.Day(), hour, min, sec, 0, loc).UTC()
				next.ScheduledEnd = next.ScheduledStart.Add(duration)
			}
			after, err := tx.UpdateTrip(ctx, next)
			if err != nil {
				return err
			}
			updated = append(updated, after)
		}
		if len(updated) == 0 {
			return domain.ErrNotFound
		}
		return NewAuditor(tx).Record(ctx, &actor.UserID, "update_future", "trip_series", from.SeriesID, nil,
			map[string]any{"from_trip_id": from.ID, "applied_count": len(updated)}, meta)
	})
	if err != nil {
		return nil, fmt.Errorf("updating series from trip: %w", err)
	}
	return updated, nil
}

// CancelFuture cancels one occurrence and every later still-planned occurrence in the
// same series. The returned slice's first element is always the trip named by
// fromTripID.
func (s *TripSeriesService) CancelFuture(ctx context.Context, actor domain.Identity, fromTripID int64, meta Meta) ([]domain.Trip, error) {
	if err := requireRole(actor, domain.RoleAdmin, domain.RoleDispatcher); err != nil {
		return nil, err
	}

	var updated []domain.Trip
	err := s.repo.InTx(ctx, func(tx *repo.Repo) error {
		from, err := tx.GetTrip(ctx, fromTripID)
		if err != nil {
			return err
		}
		if from.SeriesID == nil {
			return Message(domain.ErrValidation, "error.trip_not_in_series")
		}
		future, err := tx.ListFutureSeriesTrips(ctx, *from.SeriesID, from.ScheduledStart)
		if err != nil {
			return err
		}
		for _, t := range future {
			after, err := tx.SetTripStatus(ctx, t.ID, domain.TripCancelled)
			if err != nil {
				return err
			}
			updated = append(updated, after)
		}
		if len(updated) == 0 {
			return domain.ErrNotFound
		}
		return NewAuditor(tx).Record(ctx, &actor.UserID, "cancel_future", "trip_series", from.SeriesID, nil,
			map[string]any{"from_trip_id": fromTripID, "applied_count": len(updated)}, meta)
	})
	if err != nil {
		return nil, fmt.Errorf("cancelling series from trip: %w", err)
	}
	return updated, nil
}

type occurrence struct {
	start, end time.Time
}

// seriesOccurrences walks every calendar day from the series' first occurrence up to
// (not including) EndsOn, keeping the ones whose local weekday is in DaysOfWeek. Each
// kept day reuses FirstStart's time-of-day and the FirstEnd-FirstStart duration.
func seriesOccurrences(ts domain.TripSeries, loc *time.Location) []occurrence {
	duration := ts.FirstEnd.Sub(ts.FirstStart)
	startLocal := ts.FirstStart.In(loc)
	hour, min, sec := startLocal.Hour(), startLocal.Minute(), startLocal.Second()

	days := make(map[time.Weekday]bool, len(ts.DaysOfWeek))
	for _, d := range ts.DaysOfWeek {
		days[isoWeekday(d)] = true
	}

	endsOnLocal := ts.EndsOn.In(loc)
	var out []occurrence
	for cursor := time.Date(startLocal.Year(), startLocal.Month(), startLocal.Day(), 0, 0, 0, 0, loc); cursor.Before(endsOnLocal); cursor = cursor.AddDate(0, 0, 1) {
		if !days[cursor.Weekday()] {
			continue
		}
		start := time.Date(cursor.Year(), cursor.Month(), cursor.Day(), hour, min, sec, 0, loc).UTC()
		if start.Before(ts.FirstStart) || !start.Before(ts.EndsOn) {
			continue
		}
		out = append(out, occurrence{start: start, end: start.Add(duration)})
	}
	return out
}

// isoWeekday converts an ISO weekday number (1=Monday .. 7=Sunday) to Go's
// time.Weekday (0=Sunday .. 6=Saturday) — they already agree on 1-6 (Monday-Saturday);
// only Sunday differs (ISO 7 vs Go 0).
func isoWeekday(d int32) time.Weekday {
	if d == 7 {
		return time.Sunday
	}
	return time.Weekday(d)
}
