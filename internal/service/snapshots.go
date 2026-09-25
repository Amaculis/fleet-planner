package service

import "github.com/buscompany/bus_fleet/internal/domain"

// Audit snapshots. Explicit types rather than marshalling the domain structs directly,
// so a field added later (a password hash, a token, a secret) cannot leak into the audit
// log by accident. Driver snapshots do carry personal data — that is what an audit trail
// is for; the GDPR erasure path redacts them separately.

type busSnapshot struct {
	Plate            string  `json:"plate"`
	Model            string  `json:"model"`
	Seats            int16   `json:"seats"`
	Status           string  `json:"status"`
	InsuranceExpiry  *string `json:"insurance_expiry,omitempty"`
	InspectionExpiry *string `json:"inspection_expiry,omitempty"`
}

func snapshotBus(b domain.Bus) busSnapshot {
	return busSnapshot{
		Plate:            b.Plate,
		Model:            b.Model,
		Seats:            b.Seats,
		Status:           string(b.Status),
		InsuranceExpiry:  formatDatePtr(b.InsuranceExpiry),
		InspectionExpiry: formatDatePtr(b.InspectionExpiry),
	}
}

type driverSnapshot struct {
	FullName      string  `json:"full_name"`
	Phone         *string `json:"phone,omitempty"`
	LicenseNumber *string `json:"license_number,omitempty"`
	LicenseExpiry *string `json:"license_expiry,omitempty"`
	HourlyRate    *string `json:"hourly_rate,omitempty"` // payroll-seam
	PayType       *string `json:"pay_type,omitempty"`    // payroll-seam
	IsActive      bool    `json:"is_active"`
	Anonymized    bool    `json:"anonymized"`
}

func snapshotDriver(d domain.Driver) driverSnapshot {
	var payType *string
	if d.PayType != nil {
		s := string(*d.PayType)
		payType = &s
	}
	return driverSnapshot{
		FullName:      d.FullName,
		Phone:         d.Phone,
		LicenseNumber: d.LicenseNumber,
		LicenseExpiry: formatDatePtr(d.LicenseExpiry),
		HourlyRate:    d.HourlyRate,
		PayType:       payType,
		IsActive:      d.IsActive,
		Anonymized:    d.IsAnonymized(),
	}
}

type tripSnapshot struct {
	Origin         string  `json:"origin"`
	Destination    string  `json:"destination"`
	ScheduledStart string  `json:"scheduled_start"`
	ScheduledEnd   string  `json:"scheduled_end"`
	ActualStart    *string `json:"actual_start,omitempty"` // payroll-seam
	ActualEnd      *string `json:"actual_end,omitempty"`   // payroll-seam
	Status         string  `json:"status"`
	PaymentStatus  string  `json:"payment_status"`
	Notes          *string `json:"notes,omitempty"`
}

func snapshotTrip(t domain.Trip) tripSnapshot {
	return tripSnapshot{
		Origin:         t.Origin,
		Destination:    t.Destination,
		ScheduledStart: t.ScheduledStart.UTC().Format(timestampLayout),
		ScheduledEnd:   t.ScheduledEnd.UTC().Format(timestampLayout),
		ActualStart:    formatTimestampPtr(t.ActualStart),
		ActualEnd:      formatTimestampPtr(t.ActualEnd),
		Status:         string(t.Status),
		PaymentStatus:  string(t.PaymentStatus),
		Notes:          t.Notes,
	}
}

type assignmentSnapshot struct {
	TripID   int64 `json:"trip_id"`
	BusID    int64 `json:"bus_id"`
	DriverID int64 `json:"driver_id"`
}

func snapshotAssignment(a domain.Assignment) assignmentSnapshot {
	return assignmentSnapshot{TripID: a.TripID, BusID: a.BusID, DriverID: a.DriverID}
}

// snapshotUser deliberately omits PasswordHash.
type userSnapshot struct {
	Email    string `json:"email"`
	Role     string `json:"role"`
	DriverID *int64 `json:"driver_id,omitempty"`
	IsActive bool   `json:"is_active"`
}

func snapshotUser(u domain.User) userSnapshot {
	return userSnapshot{
		Email:    u.Email,
		Role:     string(u.Role),
		DriverID: u.DriverID,
		IsActive: u.IsActive,
	}
}
