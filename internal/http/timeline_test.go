package http

import (
	"math"
	"strings"
	"testing"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/web/templates"
)

var day = time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)

func hour(h int) time.Time { return day.Add(time.Duration(h) * time.Hour) }

func TestBlockGeometry(t *testing.T) {
	dayStart, dayEnd := day, day.AddDate(0, 0, 1)

	tests := []struct {
		name                            string
		start, end                      time.Time
		wantLeft, wantWidth             float64
		wantBefore, wantAfter, wantDrop bool
	}{
		{
			name: "morning trip", start: hour(8), end: hour(12),
			wantLeft: 100 * 8 / 24.0, wantWidth: 100 * 4 / 24.0,
		},
		{
			name: "starts at midnight", start: hour(0), end: hour(6),
			wantLeft: 0, wantWidth: 25,
		},
		{
			name: "runs to midnight", start: hour(18), end: hour(24),
			wantLeft: 75, wantWidth: 25,
		},
		{
			name: "whole day", start: hour(0), end: hour(24),
			wantLeft: 0, wantWidth: 100,
		},
		{
			name: "started yesterday", start: hour(-4), end: hour(6),
			wantLeft: 0, wantWidth: 25, wantBefore: true,
		},
		{
			name: "ends tomorrow", start: hour(20), end: hour(30),
			wantLeft: 100 * 20 / 24.0, wantWidth: 100 * 4 / 24.0, wantAfter: true,
		},
		{
			name: "spans the whole day and beyond", start: hour(-6), end: hour(30),
			wantLeft: 0, wantWidth: 100, wantBefore: true, wantAfter: true,
		},
		{
			// Fifteen minutes would be a sliver; it is widened so it stays clickable.
			name: "very short trip keeps a minimum width", start: hour(9), end: hour(9).Add(15 * time.Minute),
			wantLeft: 100 * 9 / 24.0, wantWidth: 1.2,
		},
		{
			name:  "a trip that never touches this day is dropped",
			start: hour(30), end: hour(36), wantDrop: true,
		},
		{
			name:  "yesterday's trip is dropped",
			start: hour(-10), end: hour(-2), wantDrop: true,
		},
		{
			name:  "an end before its start is dropped",
			start: hour(12), end: hour(8), wantDrop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			left, width, before, after := blockGeometry(dayStart, dayEnd, tt.start, tt.end)

			if tt.wantDrop {
				if width != 0 {
					t.Fatalf("width = %v, want 0 (block should be dropped)", width)
				}
				return
			}
			if math.Abs(left-tt.wantLeft) > 0.001 {
				t.Errorf("left = %v, want %v", left, tt.wantLeft)
			}
			if math.Abs(width-tt.wantWidth) > 0.001 {
				t.Errorf("width = %v, want %v", width, tt.wantWidth)
			}
			if before != tt.wantBefore {
				t.Errorf("continuesBefore = %v, want %v", before, tt.wantBefore)
			}
			if after != tt.wantAfter {
				t.Errorf("continuesAfter = %v, want %v", after, tt.wantAfter)
			}
			// Nothing may overflow the track.
			if left < 0 || left+width > 100.001 {
				t.Errorf("block runs outside the day: left=%v width=%v", left, width)
			}
		})
	}
}

func TestBuildTimeline(t *testing.T) {
	dayStart, dayEnd := day, day.AddDate(0, 0, 1)

	buses := []domain.Bus{
		{ID: 1, Plate: "AA-1111", Status: domain.BusActive},
		{ID: 2, Plate: "BB-2222", Status: domain.BusActive},
		{ID: 3, Plate: "CC-3333", Status: domain.BusRetired}, // idle and retired
	}
	assignments := []domain.AssignmentDetail{
		{
			Assignment: domain.Assignment{TripID: 10, BusID: 1, ScheduledStart: hour(8), ScheduledEnd: hour(12), TripStatus: domain.TripPlanned},
			BusPlate:   "AA-1111", DriverName: "Driver A", Origin: "Riga", Destination: "Liepaja",
		},
		{
			// Cancelled: holds nothing, so it is not drawn.
			Assignment: domain.Assignment{TripID: 11, BusID: 2, ScheduledStart: hour(9), ScheduledEnd: hour(11), TripStatus: domain.TripCancelled},
			BusPlate:   "BB-2222", DriverName: "Driver B", Origin: "Riga", Destination: "Ventspils",
		},
	}
	trips := []domain.Trip{
		{ID: 10, Origin: "Riga", Destination: "Liepaja", ScheduledStart: hour(8), ScheduledEnd: hour(12), Status: domain.TripPlanned},
		{ID: 11, Origin: "Riga", Destination: "Ventspils", ScheduledStart: hour(9), ScheduledEnd: hour(11), Status: domain.TripCancelled},
		{ID: 12, Origin: "Riga", Destination: "Jelgava", ScheduledStart: hour(14), ScheduledEnd: hour(16), Status: domain.TripPlanned},
	}

	tl := buildTimeline(dayStart, dayEnd, buses, assignments, trips, time.UTC)

	if len(tl.Rows) != 2 {
		t.Fatalf("rows = %d, want 2 (the idle retired bus is hidden)", len(tl.Rows))
	}
	if len(tl.Rows[0].Blocks) != 1 {
		t.Fatalf("first row has %d blocks, want 1", len(tl.Rows[0].Blocks))
	}
	if len(tl.Rows[1].Blocks) != 0 {
		t.Errorf("a cancelled trip was drawn on the timeline")
	}
	if len(tl.Unassigned) != 1 || tl.Unassigned[0].ID != 12 {
		t.Errorf("unassigned = %v, want just trip 12", tl.Unassigned)
	}
	if len(tl.Hours) != 24 {
		t.Errorf("hours = %d, want 24", len(tl.Hours))
	}
	if tl.PrevDate != "2026-09-30" || tl.NextDate != "2026-10-02" {
		t.Errorf("navigation dates = %s / %s", tl.PrevDate, tl.NextDate)
	}

	// The generated stylesheet must position exactly the drawn blocks, and contain
	// nothing but generated ids and numbers.
	wantID := templates.BlockID(1, 10)
	if got := tl.CSS; got == "" {
		t.Fatal("no positioning CSS was generated")
	} else if !strings.Contains(got, "#"+wantID+"{left:33.3333%;width:16.6667%}") {
		t.Errorf("CSS = %q, want a rule for %s", got, wantID)
	}
	if strings.Contains(tl.CSS, "Riga") || strings.Contains(tl.CSS, "<") {
		t.Errorf("the generated stylesheet carries page data: %q", tl.CSS)
	}
}

// TestRetiredBusWithBookingIsShown: hiding it would hide a real conflict.
func TestRetiredBusWithBookingIsShown(t *testing.T) {
	buses := []domain.Bus{{ID: 9, Plate: "ZZ-9999", Status: domain.BusRetired}}
	assignments := []domain.AssignmentDetail{{
		Assignment: domain.Assignment{TripID: 5, BusID: 9, ScheduledStart: hour(8), ScheduledEnd: hour(10), TripStatus: domain.TripPlanned},
		BusPlate:   "ZZ-9999", DriverName: "Driver A", Origin: "Riga", Destination: "Liepaja",
	}}

	tl := buildTimeline(day, day.AddDate(0, 0, 1), buses, assignments, nil, time.UTC)
	if len(tl.Rows) != 1 || len(tl.Rows[0].Blocks) != 1 {
		t.Fatalf("a retired bus with a booking must still appear: %+v", tl.Rows)
	}
}
