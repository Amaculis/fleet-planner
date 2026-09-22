package http

import (
	"net/http"
	"time"

	"github.com/buscompany/bus_fleet/web/templates"
)

// The driver's own view. Every handler here resolves the driver from the session
// (Identity.DriverID); no route takes a driver id, so there is nothing to tamper with.
// The trip id in the path is checked against that driver inside the SQL statement.

func (s *Server) handleMyTrips(w http.ResponseWriter, r *http.Request) {
	// Yesterday onwards: a driver still needs to close out a trip that ran past midnight.
	since := time.Now().Add(-24 * time.Hour)
	trips, err := s.trips.ListMine(r.Context(), MustIdentity(r.Context()), since)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.MyTripsPage(s.view(r), trips))
}

// handleMyTripStart records actual_start (payroll-seam).
func (s *Server) handleMyTripStart(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	if _, err := s.trips.StartMine(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.abort(w, r, err)
		return
	}
	http.Redirect(w, r, "/my/trips", http.StatusSeeOther)
}

// handleMyTripFinish records actual_end and completes the trip (payroll-seam).
func (s *Server) handleMyTripFinish(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	if _, err := s.trips.FinishMine(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.abort(w, r, err)
		return
	}
	http.Redirect(w, r, "/my/trips", http.StatusSeeOther)
}
