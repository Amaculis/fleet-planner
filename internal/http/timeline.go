package http

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/web/templates"
)

// The timeline: one day, rows of buses, trip blocks positioned by time.
//
// Positions are computed here, server-side, and emitted as a nonce'd <style> block
// rather than inline style attributes — the CSP allows no unsafe-inline, and a
// dispatcher's schedule is not worth weakening it for. The rules are built from floats
// and integers this package computes; no request data reaches the stylesheet.

const dayHours = 24

func (s *Server) handleTimeline(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	identity := MustIdentity(ctx)

	day := s.timelineDay(r)
	dayStart := day
	dayEnd := day.AddDate(0, 0, 1)

	buses, err := s.fleet.ListBuses(ctx, identity)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	assignments, err := s.assignments.ListInRange(ctx, identity, dayStart, dayEnd)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	trips, err := s.trips.ListInRange(ctx, identity, dayStart, dayEnd)
	if err != nil {
		s.abort(w, r, err)
		return
	}

	timeline := buildTimeline(dayStart, dayEnd, buses, assignments, trips, s.cfg.Location)
	s.render(w, r, http.StatusOK, templates.TimelinePage(s.view(r), timeline))
}

// timelineDay reads ?date=YYYY-MM-DD, defaulting to today in the display zone. An
// unparseable value falls back to today rather than erroring: it is a view, not a form.
func (s *Server) timelineDay(r *http.Request) time.Time {
	loc := s.cfg.Location
	if loc == nil {
		loc = time.UTC
	}
	now := time.Now().In(loc)
	day := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	if v := r.URL.Query().Get("date"); v != "" {
		if parsed, err := time.ParseInLocation("2006-01-02", v, loc); err == nil {
			day = parsed
		}
	}
	return day
}

// buildTimeline turns a day's assignments into positioned blocks, one row per bus.
// Buses that are retired and idle that day are left out; a retired bus that still has a
// booking is shown, because hiding it would hide a real conflict.
func buildTimeline(
	dayStart, dayEnd time.Time,
	buses []domain.Bus,
	assignments []domain.AssignmentDetail,
	trips []domain.Trip,
	loc *time.Location,
) templates.Timeline {
	if loc == nil {
		loc = time.UTC
	}

	blocksByBus := make(map[int64][]templates.TimelineBlock, len(buses))
	assignedTrips := make(map[int64]bool, len(assignments))

	for _, a := range assignments {
		assignedTrips[a.TripID] = true
		// A cancelled trip holds nothing and is not drawn.
		if a.TripStatus == domain.TripCancelled {
			continue
		}
		left, width, before, after := blockGeometry(dayStart, dayEnd, a.ScheduledStart, a.ScheduledEnd)
		if width <= 0 {
			continue
		}
		blocksByBus[a.BusID] = append(blocksByBus[a.BusID], templates.TimelineBlock{
			TripID:          a.TripID,
			Origin:          a.Origin,
			Destination:     a.Destination,
			DriverName:      a.DriverName,
			Start:           a.ScheduledStart,
			End:             a.ScheduledEnd,
			Status:          a.TripStatus,
			LeftPct:         left,
			WidthPct:        width,
			ContinuesBefore: before,
			ContinuesAfter:  after,
		})
	}

	rows := make([]templates.TimelineRow, 0, len(buses))
	for _, bus := range buses {
		blocks := blocksByBus[bus.ID]
		if len(blocks) == 0 && bus.Status == domain.BusRetired {
			continue
		}
		rows = append(rows, templates.TimelineRow{Bus: bus, Blocks: blocks})
	}

	// Trips nobody is driving yet: the dispatcher's actual to-do list.
	unassigned := make([]domain.Trip, 0)
	for _, trip := range trips {
		if !assignedTrips[trip.ID] && trip.Status != domain.TripCancelled {
			unassigned = append(unassigned, trip)
		}
	}

	hours := make([]int, 0, dayHours)
	for h := 0; h < dayHours; h++ {
		hours = append(hours, h)
	}

	return templates.Timeline{
		Day:        dayStart,
		PrevDate:   dayStart.AddDate(0, 0, -1).In(loc).Format("2006-01-02"),
		NextDate:   dayStart.AddDate(0, 0, 1).In(loc).Format("2006-01-02"),
		Today:      time.Now().In(loc).Format("2006-01-02"),
		Rows:       rows,
		Unassigned: unassigned,
		Hours:      hours,
		CSS:        timelineCSS(rows),
	}
}

// blockGeometry clamps a trip to the visible day and returns its position as
// percentages of the day, plus whether it runs past either edge.
//
// A trip entirely outside the day gets width 0 and is dropped by the caller.
func blockGeometry(dayStart, dayEnd, start, end time.Time) (leftPct, widthPct float64, continuesBefore, continuesAfter bool) {
	span := dayEnd.Sub(dayStart)
	if span <= 0 || !end.After(start) {
		return 0, 0, false, false
	}

	continuesBefore = start.Before(dayStart)
	continuesAfter = end.After(dayEnd)

	visibleStart, visibleEnd := start, end
	if continuesBefore {
		visibleStart = dayStart
	}
	if continuesAfter {
		visibleEnd = dayEnd
	}
	if !visibleEnd.After(visibleStart) {
		return 0, 0, false, false // no overlap with this day at all
	}

	leftPct = float64(visibleStart.Sub(dayStart)) / float64(span) * 100
	widthPct = float64(visibleEnd.Sub(visibleStart)) / float64(span) * 100

	// A very short trip still needs to be clickable.
	const minWidthPct = 1.2
	if widthPct < minWidthPct {
		widthPct = minWidthPct
	}
	if leftPct+widthPct > 100 {
		widthPct = 100 - leftPct
	}
	return leftPct, widthPct, continuesBefore, continuesAfter
}

// timelineCSS builds the per-request stylesheet positioning each block. Only generated
// numbers go in, so there is nothing here for a user to inject.
func timelineCSS(rows []templates.TimelineRow) string {
	var b strings.Builder
	for _, row := range rows {
		for _, block := range row.Blocks {
			fmt.Fprintf(&b, "#%s{left:%.4f%%;width:%.4f%%}\n",
				templates.BlockID(row.Bus.ID, block.TripID), block.LeftPct, block.WidthPct)
		}
	}
	return b.String()
}
