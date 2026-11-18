package domain_test

import (
	"testing"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
)

func TestCanTransitionTrip(t *testing.T) {
	all := []domain.TripStatus{domain.TripPlanned, domain.TripInProgress, domain.TripCompleted, domain.TripCancelled}
	allowed := map[[2]domain.TripStatus]bool{
		{domain.TripPlanned, domain.TripInProgress}:   true,
		{domain.TripPlanned, domain.TripCancelled}:    true,
		{domain.TripInProgress, domain.TripCompleted}: true,
		{domain.TripInProgress, domain.TripCancelled}: true,
		{domain.TripCompleted, domain.TripInProgress}: true,
		{domain.TripCancelled, domain.TripPlanned}:    true,
	}
	for _, from := range all {
		for _, to := range all {
			if got, want := domain.CanTransitionTrip(from, to), allowed[[2]domain.TripStatus{from, to}]; got != want {
				t.Errorf("CanTransitionTrip(%s, %s) = %v, want %v", from, to, got, want)
			}
		}
	}
}

func TestOwnsDriver(t *testing.T) {
	own, other := int64(7), int64(8)
	tests := []struct {
		name string
		id   domain.Identity
		want bool
	}{
		{"admin may act on any driver", domain.Identity{Role: domain.RoleAdmin}, true},
		{"dispatcher may act on any driver", domain.Identity{Role: domain.RoleDispatcher}, true},
		{"driver on their own record", domain.Identity{Role: domain.RoleDriver, DriverID: &own}, true},
		{"driver on someone else's record", domain.Identity{Role: domain.RoleDriver, DriverID: &other}, false},
		{"driver account not linked to a driver", domain.Identity{Role: domain.RoleDriver}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.id.OwnsDriver(own); got != tt.want {
				t.Errorf("OwnsDriver = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkedDuration(t *testing.T) {
	start := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	end := start.Add(3*time.Hour + 30*time.Minute)

	if got := (domain.Trip{ActualStart: &start, ActualEnd: &end}).WorkedDuration(); got != 3*time.Hour+30*time.Minute {
		t.Errorf("completed trip: got %v", got)
	}
	if got := (domain.Trip{ActualStart: &start}).WorkedDuration(); got != 0 {
		t.Errorf("unfinished trip must have no worked time, got %v", got)
	}
	if got := (domain.Trip{}).WorkedDuration(); got != 0 {
		t.Errorf("trip that never ran must have no worked time, got %v", got)
	}
}

func TestParseRoleRejectsUnknown(t *testing.T) {
	for _, s := range []string{"admin", "dispatcher", "driver"} {
		if _, ok := domain.ParseRole(s); !ok {
			t.Errorf("ParseRole(%q) rejected a valid role", s)
		}
	}
	for _, s := range []string{"", "root", "Admin", "driver "} {
		if _, ok := domain.ParseRole(s); ok {
			t.Errorf("ParseRole(%q) accepted an invalid role", s)
		}
	}
}
