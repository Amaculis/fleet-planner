package service

import (
	"errors"
	"testing"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
)

func TestActualsForTransition(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	started := now.Add(-2 * time.Hour)
	finished := now.Add(-time.Hour)

	tests := []struct {
		name      string
		before    domain.Trip
		to        domain.TripStatus
		wantStart *time.Time
		wantEnd   *time.Time
		wantErr   error
	}{
		{
			name:      "planner starts a planned trip: start is now",
			before:    domain.Trip{Status: domain.TripPlanned},
			to:        domain.TripInProgress,
			wantStart: &now,
		},
		{
			name:      "completing records the end",
			before:    domain.Trip{Status: domain.TripInProgress, ActualStart: &started},
			to:        domain.TripCompleted,
			wantStart: &started,
			wantEnd:   &now,
		},
		{
			name:    "completing a trip that never started is a conflict",
			before:  domain.Trip{Status: domain.TripInProgress},
			to:      domain.TripCompleted,
			wantErr: domain.ErrConflict,
		},
		{
			name:      "cancelling a started trip keeps the time driven",
			before:    domain.Trip{Status: domain.TripInProgress, ActualStart: &started},
			to:        domain.TripCancelled,
			wantStart: &started,
			wantEnd:   &now,
		},
		{
			name:      "reopening a completed trip clears only the end",
			before:    domain.Trip{Status: domain.TripCompleted, ActualStart: &started, ActualEnd: &finished},
			to:        domain.TripInProgress,
			wantStart: &started,
		},
		{
			name:   "un-cancelling clears both",
			before: domain.Trip{Status: domain.TripCancelled, ActualStart: &started, ActualEnd: &finished},
			to:     domain.TripPlanned,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			start, end, err := actualsForTransition(tt.before, tt.to, now)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !sameTime(start, tt.wantStart) || !sameTime(end, tt.wantEnd) {
				t.Errorf("got start=%v end=%v, want start=%v end=%v", start, end, tt.wantStart, tt.wantEnd)
			}
		})
	}
}

// The DB CHECK requires end > start; a trip started and finished in the same instant
// must not violate it.
func TestActualsForTransitionKeepsEndAfterStart(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	start, end, err := actualsForTransition(
		domain.Trip{Status: domain.TripInProgress, ActualStart: &now}, domain.TripCompleted, now)
	if err != nil {
		t.Fatal(err)
	}
	if !end.After(*start) {
		t.Errorf("end %v must be after start %v", end, start)
	}
}

func sameTime(a, b *time.Time) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Equal(*b)
}

func TestValidatePassword(t *testing.T) {
	if err := ValidatePassword("correct-horse-battery-staple"); err != nil {
		t.Errorf("a long passphrase was rejected: %v", err)
	}
	for name, pw := range map[string]string{
		"empty":     "",
		"too short": "short",
		"too long":  string(make([]byte, MaxPasswordLength+1)),
	} {
		if err := ValidatePassword(pw); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", name, err)
		}
	}
	// Whitespace-only must not count as a password even when long enough.
	if err := ValidatePassword("                "); !errors.Is(err, domain.ErrValidation) {
		t.Errorf("whitespace-only password accepted: %v", err)
	}
}

func TestNormalizePlate(t *testing.T) {
	for in, want := range map[string]string{
		"ab 1234":  "AB1234",
		"  AB-12 ": "AB-12",
		"lv  9 9":  "LV99",
	} {
		if got := NormalizePlate(in); got != want {
			t.Errorf("NormalizePlate(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestValidateTripPeriod(t *testing.T) {
	start := time.Date(2026, 5, 1, 8, 0, 0, 0, time.UTC)
	base := domain.Trip{PaymentStatus: domain.PaymentUnpaid, Origin: "Riga", Destination: "Tallinn", ScheduledStart: start, ScheduledEnd: start.Add(4 * time.Hour)}

	if _, err := validateTrip(base); err != nil {
		t.Fatalf("a valid trip was rejected: %v", err)
	}
	cases := map[string]func(*domain.Trip){
		"end equal to start":  func(tr *domain.Trip) { tr.ScheduledEnd = tr.ScheduledStart },
		"end before start":    func(tr *domain.Trip) { tr.ScheduledEnd = tr.ScheduledStart.Add(-time.Hour) },
		"missing origin":      func(tr *domain.Trip) { tr.Origin = "  " },
		"missing destination": func(tr *domain.Trip) { tr.Destination = "" },
		"decade-long trip":    func(tr *domain.Trip) { tr.ScheduledEnd = tr.ScheduledStart.AddDate(10, 0, 0) },
	}
	for name, mutate := range cases {
		tr := base
		mutate(&tr)
		if _, err := validateTrip(tr); !errors.Is(err, domain.ErrValidation) {
			t.Errorf("%s: err = %v, want ErrValidation", name, err)
		}
	}
}
