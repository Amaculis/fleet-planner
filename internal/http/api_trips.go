package http

import (
	"errors"
	"net/http"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

type apiAssignment struct {
	BusID      int64  `json:"busId"`
	BusPlate   string `json:"busPlate"`
	DriverID   int64  `json:"driverId"`
	DriverName string `json:"driverName"`
}

type apiTrip struct {
	ID             int64          `json:"id"`
	Origin         string         `json:"origin"`
	Destination    string         `json:"destination"`
	ScheduledStart string         `json:"scheduledStart"`
	ScheduledEnd   string         `json:"scheduledEnd"`
	ActualStart    string         `json:"actualStart,omitempty"`
	ActualEnd      string         `json:"actualEnd,omitempty"`
	Status         string         `json:"status"`
	Notes          *string        `json:"notes,omitempty"`
	Assignment     *apiAssignment `json:"assignment,omitempty"`
}

func (s *Server) apiTripFrom(t domain.Trip) apiTrip {
	out := apiTrip{
		ID:             t.ID,
		Origin:         t.Origin,
		Destination:    t.Destination,
		ScheduledStart: formatTimestamp(t.ScheduledStart, s.cfg.Location),
		ScheduledEnd:   formatTimestamp(t.ScheduledEnd, s.cfg.Location),
		Status:         string(t.Status),
		Notes:          t.Notes,
	}
	if t.ActualStart != nil {
		out.ActualStart = formatTimestamp(*t.ActualStart, s.cfg.Location)
	}
	if t.ActualEnd != nil {
		out.ActualEnd = formatTimestamp(*t.ActualEnd, s.cfg.Location)
	}
	return out
}

type apiTripRequest struct {
	Origin         string  `json:"origin"`
	Destination    string  `json:"destination"`
	ScheduledStart string  `json:"scheduledStart"`
	ScheduledEnd   string  `json:"scheduledEnd"`
	Notes          *string `json:"notes"`
}

func (s *Server) handleAPITripList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	identity := MustIdentity(ctx)
	from, to := rangeParams(r, s.cfg.Location)

	trips, err := s.trips.ListInRange(ctx, identity, from, to)
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	assignments, err := s.assignments.ListInRange(ctx, identity, from, to)
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	byTrip := make(map[int64]domain.AssignmentDetail, len(assignments))
	for _, a := range assignments {
		byTrip[a.TripID] = a
	}

	out := make([]apiTrip, len(trips))
	for i, t := range trips {
		out[i] = s.apiTripFrom(t)
		if a, ok := byTrip[t.ID]; ok {
			out[i].Assignment = &apiAssignment{
				BusID: a.BusID, BusPlate: a.BusPlate,
				DriverID: a.DriverID, DriverName: a.DriverName,
			}
		}
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

// apiTripDetail gathers the trip plus its assignment (bus plate, driver name resolved
// from the assignable lists) — the same shape tripDetail() builds for the HTML page.
func (s *Server) apiTripDetail(r *http.Request, id int64) (apiTrip, error) {
	ctx := r.Context()
	identity := MustIdentity(ctx)

	trip, err := s.trips.Get(ctx, identity, id)
	if err != nil {
		return apiTrip{}, err
	}
	out := s.apiTripFrom(trip)

	assignment, err := s.assignments.GetForTrip(ctx, identity, id)
	switch {
	case err == nil:
		buses, err := s.fleet.ListAssignableBuses(ctx, identity)
		if err != nil {
			return apiTrip{}, err
		}
		drivers, err := s.fleet.ListAssignableDrivers(ctx, identity)
		if err != nil {
			return apiTrip{}, err
		}
		a := &apiAssignment{BusID: assignment.BusID, DriverID: assignment.DriverID}
		for _, b := range buses {
			if b.ID == assignment.BusID {
				a.BusPlate = b.Plate
			}
		}
		for _, d := range drivers {
			if d.ID == assignment.DriverID {
				a.DriverName = d.FullName
			}
		}
		out.Assignment = a
	case errors.Is(err, domain.ErrNotFound):
		// No assignment yet — fine, out.Assignment stays nil.
	default:
		return apiTrip{}, err
	}
	return out, nil
}

func (s *Server) handleAPITripGet(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	detail, err := s.apiTripDetail(r, id)
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, detail)
}

func (s *Server) tripFromJSON(req apiTripRequest, id int64) (domain.Trip, error) {
	trip := domain.Trip{ID: id, Origin: req.Origin, Destination: req.Destination, Notes: req.Notes}
	var err error
	if trip.ScheduledStart, err = jsonTimestamp(req.ScheduledStart, "field.scheduled_start", s.cfg.Location); err != nil {
		return trip, err
	}
	if trip.ScheduledEnd, err = jsonTimestamp(req.ScheduledEnd, "field.scheduled_end", s.cfg.Location); err != nil {
		return trip, err
	}
	return trip, nil
}

func (s *Server) handleAPITripCreate(w http.ResponseWriter, r *http.Request) {
	var req apiTripRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	input, err := s.tripFromJSON(req, 0)
	if err == nil {
		input, err = s.trips.Create(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, s.apiTripFrom(input))
}

func (s *Server) handleAPITripUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	var req apiTripRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	input, err := s.tripFromJSON(req, id)
	if err == nil {
		input, err = s.trips.Update(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, s.apiTripFrom(input))
}

func (s *Server) handleAPITripDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	if err := s.trips.Delete(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type apiTripStatusRequest struct {
	Status string `json:"status"`
}

func (s *Server) handleAPITripStatus(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	var req apiTripStatusRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	trip, err := s.trips.SetStatus(r.Context(), MustIdentity(r.Context()), id, domain.TripStatus(req.Status), s.meta(r))
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, s.apiTripFrom(trip))
}

type apiAssignRequest struct {
	BusID    int64 `json:"busId"`
	DriverID int64 `json:"driverId"`
}

func (s *Server) handleAPITripAssign(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	var req apiAssignRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	if req.BusID <= 0 {
		s.writeAPIError(w, r, service.FieldMessage(domain.ErrValidation, "validation.required", "field.bus"))
		return
	}
	if req.DriverID <= 0 {
		s.writeAPIError(w, r, service.FieldMessage(domain.ErrValidation, "validation.required", "field.driver"))
		return
	}
	if _, err := s.assignments.Assign(r.Context(), MustIdentity(r.Context()), id, req.BusID, req.DriverID, s.meta(r)); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	detail, err := s.apiTripDetail(r, id)
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, detail)
}

func (s *Server) handleAPITripUnassign(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	if err := s.assignments.Unassign(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	detail, err := s.apiTripDetail(r, id)
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, detail)
}
