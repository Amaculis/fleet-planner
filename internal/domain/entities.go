package domain

import "time"

// ---------------------------------------------------------------------------
// Buses
// ---------------------------------------------------------------------------

// BusStatus mirrors the Postgres enum bus_status.
type BusStatus string

const (
	BusActive      BusStatus = "active"
	BusMaintenance BusStatus = "maintenance"
	BusRetired     BusStatus = "retired"
)

func ParseBusStatus(s string) (BusStatus, bool) {
	switch BusStatus(s) {
	case BusActive, BusMaintenance, BusRetired:
		return BusStatus(s), true
	}
	return "", false
}

type Bus struct {
	ID     int64
	Plate  string // stored normalised: upper-case, no spaces (DB CHECK enforces it)
	Model  string
	Seats  int16
	Status BusStatus
	// Document seams: stored, but nothing reminds on them yet.
	InsuranceExpiry  *time.Time
	InspectionExpiry *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

// ---------------------------------------------------------------------------
// Drivers
// ---------------------------------------------------------------------------

// PayType mirrors the Postgres enum pay_type. payroll-seam: unused in the MVP.
type PayType string

const (
	PayHourly  PayType = "hourly"
	PayPerTrip PayType = "per_trip"
	PayMonthly PayType = "monthly"
)

func ParsePayType(s string) (PayType, bool) {
	switch PayType(s) {
	case PayHourly, PayPerTrip, PayMonthly:
		return PayType(s), true
	}
	return "", false
}

type Driver struct {
	ID            int64
	FullName      string
	Phone         *string
	LicenseNumber *string
	LicenseExpiry *time.Time
	// payroll-seam: carried through the whole stack but never read by MVP logic.
	// Decimal string ("12.50", EUR) so no precision is lost between Postgres numeric
	// and Go. Payroll later = worked hours x rate.
	HourlyRate *string
	PayType    *PayType
	IsActive   bool
	// Set by GDPR erasure. An anonymised driver keeps its id (and therefore its
	// assignments and audit trail) but holds no identifying data.
	AnonymizedAt *time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (d Driver) IsAnonymized() bool { return d.AnonymizedAt != nil }

// ---------------------------------------------------------------------------
// Trips
// ---------------------------------------------------------------------------

// TripStatus mirrors the Postgres enum trip_status.
type TripStatus string

const (
	TripPlanned    TripStatus = "planned"
	TripInProgress TripStatus = "in_progress"
	TripCompleted  TripStatus = "completed"
	TripCancelled  TripStatus = "cancelled"
)

func ParseTripStatus(s string) (TripStatus, bool) {
	switch TripStatus(s) {
	case TripPlanned, TripInProgress, TripCompleted, TripCancelled:
		return TripStatus(s), true
	}
	return "", false
}

// CanTransitionTrip defines the allowed status moves.
//
//	planned     -> in_progress | cancelled
//	in_progress -> completed   | cancelled
//	completed   -> in_progress   (correction; unlocks the assignment for editing)
//	cancelled   -> planned       (un-cancel; the EXCLUDE constraints re-check the slot)
//
// Reopening a completed trip is deliberately possible but audited: it is the only way
// to fix a wrong bus or driver, because the assignment is otherwise locked.
func CanTransitionTrip(from, to TripStatus) bool {
	if from == to {
		return false
	}
	switch from {
	case TripPlanned:
		return to == TripInProgress || to == TripCancelled
	case TripInProgress:
		return to == TripCompleted || to == TripCancelled
	case TripCompleted:
		return to == TripInProgress
	case TripCancelled:
		return to == TripPlanned
	}
	return false
}

// PaymentStatus mirrors the Postgres enum payment_status. It tracks what the client
// has paid for a trip — planner-only (admin/dispatcher); never exposed to a driver.
type PaymentStatus string

const (
	PaymentUnpaid      PaymentStatus = "unpaid"
	PaymentReserved    PaymentStatus = "reserved"
	PaymentAdvancePaid PaymentStatus = "advance_paid"
	PaymentPaid        PaymentStatus = "paid"
)

func ParsePaymentStatus(s string) (PaymentStatus, bool) {
	switch PaymentStatus(s) {
	case PaymentUnpaid, PaymentReserved, PaymentAdvancePaid, PaymentPaid:
		return PaymentStatus(s), true
	}
	return "", false
}

type Trip struct {
	ID             int64
	Origin         string
	Destination    string
	ScheduledStart time.Time
	ScheduledEnd   time.Time
	// payroll-seam: the worked-time window. Recorded by the assigned driver.
	ActualStart *time.Time
	ActualEnd   *time.Time
	Status      TripStatus
	// PaymentStatus is planner-only — see the type's own doc comment. Deliberately
	// absent from every driver-facing query and API shape (ListTripsForDriver,
	// apiDriverTrip, ...), not just hidden by the frontend.
	PaymentStatus PaymentStatus
	Notes         *string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// WorkedDuration is the payroll seam made explicit: once a trip is completed, this is
// the time the assigned driver is paid for. Nothing in the MVP multiplies it by a rate.
func (t Trip) WorkedDuration() time.Duration {
	if t.ActualStart == nil || t.ActualEnd == nil {
		return 0
	}
	return t.ActualEnd.Sub(*t.ActualStart)
}

// DriverTrip is a trip as the assigned driver sees it.
type DriverTrip struct {
	Trip
	BusPlate string
}

// ---------------------------------------------------------------------------
// Assignments
// ---------------------------------------------------------------------------

type Assignment struct {
	ID       int64
	TripID   int64
	BusID    int64
	DriverID int64
	// Copies of the trip's window, kept in step by a composite FK with ON UPDATE
	// CASCADE. They exist so the EXCLUDE constraints can see them.
	ScheduledStart time.Time
	ScheduledEnd   time.Time
	TripStatus     TripStatus
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

// AssignmentDetail joins the names a planner needs to read a row.
type AssignmentDetail struct {
	Assignment
	BusPlate    string
	DriverName  string
	Origin      string
	Destination string
}

// Conflict describes an existing booking that overlaps a proposed one. Returned by the
// service's pre-check so the user sees which trip is in the way, rather than a database
// constraint name.
type Conflict struct {
	TripID      int64
	Origin      string
	Destination string
	Start       time.Time
	End         time.Time
}

// UserListItem is a user row plus the name of the driver it belongs to, if any.
type UserListItem struct {
	User
	DriverName *string
}
