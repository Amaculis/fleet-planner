package http

import (
	"net/http"

	"github.com/buscompany/bus_fleet/internal/domain"
)

// ---------------------------------------------------------------------------
// Buses
// ---------------------------------------------------------------------------

type apiBus struct {
	ID               int64  `json:"id"`
	Plate            string `json:"plate"`
	Model            string `json:"model"`
	Seats            int16  `json:"seats"`
	Status           string `json:"status"`
	InsuranceExpiry  string `json:"insuranceExpiry"`
	InspectionExpiry string `json:"inspectionExpiry"`
}

func apiBusFrom(b domain.Bus) apiBus {
	return apiBus{
		ID:               b.ID,
		Plate:            b.Plate,
		Model:            b.Model,
		Seats:            b.Seats,
		Status:           string(b.Status),
		InsuranceExpiry:  formatDate(b.InsuranceExpiry),
		InspectionExpiry: formatDate(b.InspectionExpiry),
	}
}

type apiBusRequest struct {
	Plate            string `json:"plate"`
	Model            string `json:"model"`
	Seats            int16  `json:"seats"`
	Status           string `json:"status"`
	InsuranceExpiry  string `json:"insuranceExpiry"`
	InspectionExpiry string `json:"inspectionExpiry"`
}

func (req apiBusRequest) toDomain(id int64) (domain.Bus, error) {
	bus := domain.Bus{ID: id, Plate: req.Plate, Model: req.Model, Seats: req.Seats, Status: domain.BusStatus(req.Status)}
	var err error
	if bus.InsuranceExpiry, err = jsonDate(req.InsuranceExpiry, "field.insurance_expiry"); err != nil {
		return bus, err
	}
	if bus.InspectionExpiry, err = jsonDate(req.InspectionExpiry, "field.inspection_expiry"); err != nil {
		return bus, err
	}
	return bus, nil
}

// isAssignable is the ?assignable=1 filter the trip-assignment picker uses (only
// active buses/drivers can be booked — see FleetService.ListAssignable{Buses,Drivers}).
func isAssignable(r *http.Request) bool { return r.URL.Query().Get("assignable") == "1" }

func (s *Server) handleAPIBusList(w http.ResponseWriter, r *http.Request) {
	identity := MustIdentity(r.Context())
	var (
		buses []domain.Bus
		err   error
	)
	if isAssignable(r) {
		buses, err = s.fleet.ListAssignableBuses(r.Context(), identity)
	} else {
		buses, err = s.fleet.ListBuses(r.Context(), identity)
	}
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	out := make([]apiBus, len(buses))
	for i, b := range buses {
		out[i] = apiBusFrom(b)
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

func (s *Server) handleAPIBusGet(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	bus, err := s.fleet.GetBus(r.Context(), MustIdentity(r.Context()), id)
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, apiBusFrom(bus))
}

func (s *Server) handleAPIBusCreate(w http.ResponseWriter, r *http.Request) {
	var req apiBusRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	input, err := req.toDomain(0)
	if err == nil {
		input, err = s.fleet.CreateBus(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, apiBusFrom(input))
}

func (s *Server) handleAPIBusUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	var req apiBusRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	input, err := req.toDomain(id)
	if err == nil {
		input, err = s.fleet.UpdateBus(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, apiBusFrom(input))
}

func (s *Server) handleAPIBusDelete(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	if err := s.fleet.DeleteBus(r.Context(), MustIdentity(r.Context()), id, s.meta(r)); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------------------
// Drivers
// ---------------------------------------------------------------------------

type apiDriver struct {
	ID            int64   `json:"id"`
	FullName      string  `json:"fullName"`
	Phone         *string `json:"phone,omitempty"`
	LicenseNumber *string `json:"licenseNumber,omitempty"`
	LicenseExpiry string  `json:"licenseExpiry"`
	HourlyRate    *string `json:"hourlyRate,omitempty"`
	PayType       *string `json:"payType,omitempty"`
	IsActive      bool    `json:"isActive"`
	Anonymized    bool    `json:"anonymized"`
}

func apiDriverFrom(d domain.Driver) apiDriver {
	out := apiDriver{
		ID:            d.ID,
		FullName:      d.FullName,
		Phone:         d.Phone,
		LicenseNumber: d.LicenseNumber,
		LicenseExpiry: formatDate(d.LicenseExpiry),
		HourlyRate:    d.HourlyRate,
		IsActive:      d.IsActive,
		Anonymized:    d.IsAnonymized(),
	}
	if d.PayType != nil {
		pt := string(*d.PayType)
		out.PayType = &pt
	}
	return out
}

type apiDriverRequest struct {
	FullName      string  `json:"fullName"`
	Phone         *string `json:"phone"`
	LicenseNumber *string `json:"licenseNumber"`
	LicenseExpiry string  `json:"licenseExpiry"`
	HourlyRate    *string `json:"hourlyRate"`
	PayType       *string `json:"payType"`
	IsActive      bool    `json:"isActive"`
}

func (req apiDriverRequest) toDomain(id int64, isNew bool) (domain.Driver, error) {
	driver := domain.Driver{
		ID:            id,
		FullName:      req.FullName,
		Phone:         req.Phone,
		LicenseNumber: req.LicenseNumber,
		HourlyRate:    req.HourlyRate,
		IsActive:      true,
	}
	if !isNew {
		driver.IsActive = req.IsActive
	}
	if req.PayType != nil {
		pt := domain.PayType(*req.PayType)
		driver.PayType = &pt
	}
	var err error
	if driver.LicenseExpiry, err = jsonDate(req.LicenseExpiry, "field.license_expiry"); err != nil {
		return driver, err
	}
	return driver, nil
}

func (s *Server) handleAPIDriverList(w http.ResponseWriter, r *http.Request) {
	identity := MustIdentity(r.Context())
	var (
		drivers []domain.Driver
		err     error
	)
	if isAssignable(r) {
		drivers, err = s.fleet.ListAssignableDrivers(r.Context(), identity)
	} else {
		drivers, err = s.fleet.ListDrivers(r.Context(), identity)
	}
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	out := make([]apiDriver, len(drivers))
	for i, d := range drivers {
		out[i] = apiDriverFrom(d)
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

func (s *Server) handleAPIDriverGet(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	driver, err := s.fleet.GetDriver(r.Context(), MustIdentity(r.Context()), id)
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, apiDriverFrom(driver))
}

func (s *Server) handleAPIDriverCreate(w http.ResponseWriter, r *http.Request) {
	var req apiDriverRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	input, err := req.toDomain(0, true)
	if err == nil {
		input, err = s.fleet.CreateDriver(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, apiDriverFrom(input))
}

func (s *Server) handleAPIDriverUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	var req apiDriverRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	input, err := req.toDomain(id, false)
	if err == nil {
		input, err = s.fleet.UpdateDriver(r.Context(), MustIdentity(r.Context()), input, s.meta(r))
	}
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, apiDriverFrom(input))
}

// handleAPIDriverAnonymize is the GDPR erasure action — irreversible, hence its own
// endpoint rather than a field on the update request (see docs/... and CLAUDE.md's
// GDPR baseline).
func (s *Server) handleAPIDriverAnonymize(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	driver, err := s.fleet.AnonymizeDriver(r.Context(), MustIdentity(r.Context()), id, s.meta(r))
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, apiDriverFrom(driver))
}
