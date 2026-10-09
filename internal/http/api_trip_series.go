package http

import (
	"net/http"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

type apiTripSeriesRequest struct {
	Origin        string  `json:"origin"`
	Destination   string  `json:"destination"`
	DaysOfWeek    []int32 `json:"daysOfWeek"`
	FirstStart    string  `json:"firstStart"`
	FirstEnd      string  `json:"firstEnd"`
	EndsOn        string  `json:"endsOn"` // "YYYY-MM-DD", the last local date a trip may occur on (inclusive)
	PaymentStatus string  `json:"paymentStatus"`
	Notes         *string `json:"notes"`
}

func (s *Server) handleAPITripSeriesCreate(w http.ResponseWriter, r *http.Request) {
	var req apiTripSeriesRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}

	paymentStatus := domain.PaymentStatus(req.PaymentStatus)
	if paymentStatus == "" {
		paymentStatus = domain.PaymentUnpaid
	}
	input := domain.TripSeries{
		Origin: req.Origin, Destination: req.Destination,
		DaysOfWeek: req.DaysOfWeek, PaymentStatus: paymentStatus, Notes: req.Notes,
	}
	var err error
	if input.FirstStart, err = jsonTimestamp(req.FirstStart, "field.scheduled_start", s.cfg.Location); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	if input.FirstEnd, err = jsonTimestamp(req.FirstEnd, "field.scheduled_end", s.cfg.Location); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	endsOnDate, err := time.ParseInLocation("2006-01-02", req.EndsOn, s.cfg.Location)
	if err != nil {
		s.writeAPIError(w, r, service.FieldMessage(domain.ErrValidation, "validation.format", "field.ends_on"))
		return
	}
	// The chosen "ends on" date is the last date a trip may occur on; generation treats
	// EndsOn as an exclusive cutoff, so advance to the start of the following local day.
	input.EndsOn = endsOnDate.AddDate(0, 0, 1)

	trips, err := s.tripSeries.Create(r.Context(), MustIdentity(r.Context()), input, s.cfg.Location, s.meta(r))
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	out := make([]apiTrip, len(trips))
	for i, t := range trips {
		out[i] = s.apiTripFrom(t)
	}
	s.writeJSON(w, r, http.StatusCreated, out)
}
