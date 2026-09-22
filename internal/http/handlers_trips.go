package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/web/templates"
)

func (s *Server) handleTripList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	identity := MustIdentity(ctx)
	from, to := rangeParams(r, s.cfg.Location)

	trips, err := s.trips.ListInRange(ctx, identity, from, to)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	assignments, err := s.assignments.ListInRange(ctx, identity, from, to)
	if err != nil {
		s.abort(w, r, err)
		return
	}

	byTrip := make(map[int64]*domain.AssignmentDetail, len(assignments))
	for i := range assignments {
		byTrip[assignments[i].TripID] = &assignments[i]
	}
	rows := make([]templates.TripRow, 0, len(trips))
	for _, trip := range trips {
		rows = append(rows, templates.TripRow{Trip: trip, Assignment: byTrip[trip.ID]})
	}

	s.render(w, r, http.StatusOK, templates.TripsPage(s.view(r), rows,
		from.In(s.cfg.Location).Format("2006-01-02"),
		to.In(s.cfg.Location).Format("2006-01-02")))
}

func (s *Server) handleTripNew(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, templates.TripFormPage(s.view(r), domain.Trip{}, true))
}

func (s *Server) handleTripDetail(w http.ResponseWriter, r *http.Request) {
	detail, err := s.tripDetail(r)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.TripDetailPage(s.view(r), detail))
}

// tripDetail gathers the trip, its assignment and the pickable buses and drivers.
func (s *Server) tripDetail(r *http.Request) (templates.TripDetail, error) {
	ctx := r.Context()
	identity := MustIdentity(ctx)

	id, err := idParam(r, "id")
	if err != nil {
		return templates.TripDetail{}, err
	}
	trip, err := s.trips.Get(ctx, identity, id)
	if err != nil {
		return templates.TripDetail{}, err
	}
	buses, err := s.fleet.ListAssignableBuses(ctx, identity)
	if err != nil {
		return templates.TripDetail{}, err
	}
	drivers, err := s.fleet.ListAssignableDrivers(ctx, identity)
	if err != nil {
		return templates.TripDetail{}, err
	}

	detail := templates.TripDetail{Trip: trip, Buses: buses, Drivers: drivers}
	if assignment, err := s.assignments.GetForTrip(ctx, identity, id); err == nil {
		detail.Assignment = &domain.AssignmentDetail{Assignment: assignment}
		for _, bus := range buses {
			if bus.ID == assignment.BusID {
				detail.Assignment.BusPlate = bus.Plate
			}
		}
		for _, driver := range drivers {
			if driver.ID == assignment.DriverID {
				detail.Assignment.DriverName = driver.FullName
			}
		}
	} else if !errors.Is(err, domain.ErrNotFound) {
		return templates.TripDetail{}, err
	}
	return detail, nil
}

func (s *Server) handleTripEdit(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	trip, err := s.trips.Get(r.Context(), MustIdentity(r.Context()), id)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.TripFormPage(s.view(r), trip, false))
}

func (s *Server) handleTripCreate(w http.ResponseWriter, r *http.Request) {
	input, err := s.tripFromForm(r, 0)
	if err == nil {
		var created domain.Trip
		created, err = s.trips.Create(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
		if err == nil {
			http.Redirect(w, r, "/trips/"+strconv.FormatInt(created.ID, 10), http.StatusSeeOther)
			return
		}
	}
	status, msg := s.failed(r, err)
	s.render(w, r, status, templates.TripFormPage(s.viewWithError(r, msg), input, true))
}

func (s *Server) handleTripUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	input, err := s.tripFromForm(r, id)
	if err == nil {
		_, err = s.trips.Update(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
		if err == nil {
			http.Redirect(w, r, "/trips/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
			return
		}
	}
	// A rescheduling clash lands here with the blocking trips attached.
	status, msg := s.failed(r, err)
	s.render(w, r, status, templates.TripFormPage(s.viewWithError(r, msg), input, false))
}

func (s *Server) handleTripStatus(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	status := domain.TripStatus(formString(r, "status"))
	if _, err := s.trips.SetStatus(r.Context(), MustIdentity(r.Context()), id, status, s.meta(r)); err != nil {
		s.renderTripDetailError(w, r, err)
		return
	}
	http.Redirect(w, r, "/trips/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *Server) handleTripDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	if err := s.trips.Delete(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.renderTripDetailError(w, r, err)
		return
	}
	http.Redirect(w, r, "/trips", http.StatusSeeOther)
}

func (s *Server) tripFromForm(r *http.Request, id int64) (domain.Trip, error) {
	trip := domain.Trip{ID: id}
	if err := r.ParseForm(); err != nil {
		return trip, domain.ErrValidation
	}
	trip.Origin = formString(r, "origin")
	trip.Destination = formString(r, "destination")
	trip.Notes = formOptional(r, "notes")

	var err error
	if trip.ScheduledStart, err = formTimestamp(r, "scheduled_start", "field.scheduled_start", s.cfg.Location); err != nil {
		return trip, err
	}
	if trip.ScheduledEnd, err = formTimestamp(r, "scheduled_end", "field.scheduled_end", s.cfg.Location); err != nil {
		return trip, err
	}
	return trip, nil
}

// ---------------------------------------------------------------------------
// Assignments
// ---------------------------------------------------------------------------

// handleTripAssign books a bus and driver. A clash re-renders the trip page with the
// overlapping trips listed, rather than a bare error.
func (s *Server) handleTripAssign(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	busID, err := formInt(r, "bus_id", "field.bus")
	if err != nil {
		s.renderTripDetailError(w, r, err)
		return
	}
	driverID, err := formInt(r, "driver_id", "field.driver")
	if err != nil {
		s.renderTripDetailError(w, r, err)
		return
	}
	if _, err := s.assignments.Assign(r.Context(), MustIdentity(r.Context()), id, busID, driverID, s.meta(r)); err != nil {
		s.renderTripDetailError(w, r, err)
		return
	}
	http.Redirect(w, r, "/trips/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

func (s *Server) handleTripUnassign(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	if err := s.assignments.Unassign(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.renderTripDetailError(w, r, err)
		return
	}
	http.Redirect(w, r, "/trips/"+strconv.FormatInt(id, 10), http.StatusSeeOther)
}

// renderTripDetailError re-renders the trip page carrying the failure, so the user keeps
// their context (and sees which bookings clash).
func (s *Server) renderTripDetailError(w http.ResponseWriter, r *http.Request, cause error) {
	status, msg := s.failed(r, cause)
	detail, err := s.tripDetail(r)
	if err != nil {
		s.abort(w, r, cause)
		return
	}
	s.render(w, r, status, templates.TripDetailPage(s.viewWithError(r, msg), detail))
}
