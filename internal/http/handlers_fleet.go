package http

import (
	"net/http"
	"strconv"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/web/templates"
)

// Buses and drivers. Reading is open to dispatchers (they need it to plan); every
// mutation is admin-only, enforced by RequireRole on the route and re-checked in the
// service.

func (s *Server) handleBusList(w http.ResponseWriter, r *http.Request) {
	identity := MustIdentity(r.Context())
	buses, err := s.fleet.ListBuses(r.Context(), identity)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.BusesPage(s.view(r), buses))
}

func (s *Server) handleBusNew(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, templates.BusFormPage(s.view(r), domain.Bus{Status: domain.BusActive, Seats: 50}, true))
}

func (s *Server) handleBusEdit(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	bus, err := s.fleet.GetBus(r.Context(), MustIdentity(r.Context()), id)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.BusFormPage(s.view(r), bus, false))
}

func (s *Server) handleBusCreate(w http.ResponseWriter, r *http.Request) {
	input, err := s.busFromForm(r, 0)
	if err == nil {
		_, err = s.fleet.CreateBus(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		// Re-render the form with the message, keeping what was typed.
		status, msg := s.failed(r, err)
		s.render(w, r, status, templates.BusFormPage(s.viewWithError(r, msg), input, true))
		return
	}
	http.Redirect(w, r, "/buses", http.StatusSeeOther)
}

func (s *Server) handleBusUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	input, err := s.busFromForm(r, id)
	if err == nil {
		_, err = s.fleet.UpdateBus(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		status, msg := s.failed(r, err)
		s.render(w, r, status, templates.BusFormPage(s.viewWithError(r, msg), input, false))
		return
	}
	http.Redirect(w, r, "/buses", http.StatusSeeOther)
}

func (s *Server) handleBusDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	if err := s.fleet.DeleteBus(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.abort(w, r, err)
		return
	}
	http.Redirect(w, r, "/buses", http.StatusSeeOther)
}

func (s *Server) busFromForm(r *http.Request, id int64) (domain.Bus, error) {
	bus := domain.Bus{ID: id}
	if err := r.ParseForm(); err != nil {
		return bus, domain.ErrValidation
	}
	bus.Plate = formString(r, "plate")
	bus.Model = formString(r, "model")
	bus.Status = domain.BusStatus(formString(r, "status"))

	seats, err := strconv.ParseInt(formString(r, "seats"), 10, 16)
	if err != nil {
		seats = 0 // validated in the service, which produces the message
	}
	bus.Seats = int16(seats)

	if bus.InsuranceExpiry, err = formDate(r, "insurance_expiry", "field.insurance_expiry"); err != nil {
		return bus, err
	}
	if bus.InspectionExpiry, err = formDate(r, "inspection_expiry", "field.inspection_expiry"); err != nil {
		return bus, err
	}
	return bus, nil
}

// ---------------------------------------------------------------------------
// Drivers
// ---------------------------------------------------------------------------

func (s *Server) handleDriverList(w http.ResponseWriter, r *http.Request) {
	drivers, err := s.fleet.ListDrivers(r.Context(), MustIdentity(r.Context()))
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.DriversPage(s.view(r), drivers))
}

func (s *Server) handleDriverNew(w http.ResponseWriter, r *http.Request) {
	s.render(w, r, http.StatusOK, templates.DriverFormPage(s.view(r), domain.Driver{IsActive: true}, true))
}

func (s *Server) handleDriverEdit(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	driver, err := s.fleet.GetDriver(r.Context(), MustIdentity(r.Context()), id)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.DriverFormPage(s.view(r), driver, false))
}

func (s *Server) handleDriverCreate(w http.ResponseWriter, r *http.Request) {
	input, err := s.driverFromForm(r, 0, true)
	if err == nil {
		_, err = s.fleet.CreateDriver(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		status, msg := s.failed(r, err)
		s.render(w, r, status, templates.DriverFormPage(s.viewWithError(r, msg), input, true))
		return
	}
	http.Redirect(w, r, "/drivers", http.StatusSeeOther)
}

func (s *Server) handleDriverUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	input, err := s.driverFromForm(r, id, false)
	if err == nil {
		_, err = s.fleet.UpdateDriver(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		status, msg := s.failed(r, err)
		s.render(w, r, status, templates.DriverFormPage(s.viewWithError(r, msg), input, false))
		return
	}
	http.Redirect(w, r, "/drivers", http.StatusSeeOther)
}

// handleDriverAnonymize is the GDPR erasure action. Irreversible, hence its own
// endpoint rather than a flag on the edit form.
func (s *Server) handleDriverAnonymize(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	if _, err := s.fleet.AnonymizeDriver(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.abort(w, r, err)
		return
	}
	http.Redirect(w, r, "/drivers", http.StatusSeeOther)
}

func (s *Server) driverFromForm(r *http.Request, id int64, isNew bool) (domain.Driver, error) {
	driver := domain.Driver{ID: id, IsActive: true}
	if err := r.ParseForm(); err != nil {
		return driver, domain.ErrValidation
	}
	driver.FullName = formString(r, "full_name")
	driver.Phone = formOptional(r, "phone")
	driver.LicenseNumber = formOptional(r, "license_number")
	driver.HourlyRate = formOptional(r, "hourly_rate") // payroll-seam
	if payType := formOptional(r, "pay_type"); payType != nil {
		pt := domain.PayType(*payType)
		driver.PayType = &pt
	}
	if !isNew {
		driver.IsActive = formString(r, "is_active") == "true"
	}

	var err error
	if driver.LicenseExpiry, err = formDate(r, "license_expiry", "field.license_expiry"); err != nil {
		return driver, err
	}
	return driver, nil
}
