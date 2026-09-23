package http

import (
	"net/http"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
)

// apiDriverTrip is a trip as the signed-in driver sees it — their own bus plate
// included, nothing about other drivers or unrelated trips.
type apiDriverTrip struct {
	ID             int64  `json:"id"`
	Origin         string `json:"origin"`
	Destination    string `json:"destination"`
	ScheduledStart string `json:"scheduledStart"`
	ScheduledEnd   string `json:"scheduledEnd"`
	Status         string `json:"status"`
	BusPlate       string `json:"busPlate"`
}

func (s *Server) apiDriverTripFrom(t domain.DriverTrip) apiDriverTrip {
	return apiDriverTrip{
		ID:             t.ID,
		Origin:         t.Origin,
		Destination:    t.Destination,
		ScheduledStart: formatTimestamp(t.ScheduledStart, s.cfg.Location),
		ScheduledEnd:   formatTimestamp(t.ScheduledEnd, s.cfg.Location),
		Status:         string(t.Status),
		BusPlate:       t.BusPlate,
	}
}

// handleAPIMyTrips mirrors handleMyTrips (see handlers_driver.go): yesterday onwards,
// so a driver can still close out a trip that ran past midnight.
func (s *Server) handleAPIMyTrips(w http.ResponseWriter, r *http.Request) {
	since := time.Now().Add(-24 * time.Hour)
	trips, err := s.trips.ListMine(r.Context(), MustIdentity(r.Context()), since)
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	out := make([]apiDriverTrip, len(trips))
	for i, t := range trips {
		out[i] = s.apiDriverTripFrom(t)
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

func (s *Server) handleAPIMyTripStart(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	if _, err := s.trips.StartMine(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleAPIMyTripFinish(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	if _, err := s.trips.FinishMine(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
