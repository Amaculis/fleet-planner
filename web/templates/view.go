package templates

import (
	"html"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/i18n"
)

// View carries everything every page needs: the translator, the CSP nonce, the CSRF
// token, who is signed in, and the display time zone. Templates take it as their first
// argument instead of a growing parameter list.
//
// These types live in the templates package (not internal/http) because templ-generated
// code is imported by internal/http — the dependency only points one way.
type View struct {
	P        *i18n.Printer
	Nonce    string
	CSRF     string
	Identity domain.Identity
	Loc      *time.Location // display zone; storage is always UTC
	Path     string         // r.URL.Path, for highlighting the active nav link only

	// Message is an already-translated notice or error for this page.
	Message   string
	IsError   bool
	Conflicts []domain.Conflict // overlapping bookings, when a clash was refused
}

func (v View) T(key string, args ...any) string { return v.P.T(key, args...) }

// FmtTime renders a timestamp in the display zone. Storage stays UTC; formatting
// happens here, at the view boundary, and nowhere else.
func (v View) FmtTime(t time.Time) string {
	return t.In(v.location()).Format("2006-01-02 15:04")
}

// FmtClock is the short form used inside a day's listing.
func (v View) FmtClock(t time.Time) string {
	return t.In(v.location()).Format("15:04")
}

func (v View) FmtDate(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return t.In(v.location()).Format("2006-01-02")
}

func (v View) FmtTimePtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return v.FmtTime(*t)
}

// FormTime renders a timestamp for <input type="datetime-local">, which expects local
// wall-clock time with no zone.
func (v View) FormTime(t time.Time) string {
	return t.In(v.location()).Format("2006-01-02T15:04")
}

func (v View) FormDate(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(v.location()).Format("2006-01-02")
}

func (v View) location() *time.Location {
	if v.Loc == nil {
		return time.UTC
	}
	return v.Loc
}

// Str renders an optional string for display.
func (v View) Str(s *string) string {
	if s == nil || *s == "" {
		return "—"
	}
	return *s
}

// TripStatus, BusStatus, PayType and Role are enums: translated through message ids,
// never printed raw.
func (v View) TripStatus(s domain.TripStatus) string { return v.T("trip_status." + string(s)) }
func (v View) BusStatus(s domain.BusStatus) string   { return v.T("bus_status." + string(s)) }
func (v View) Role(r domain.Role) string             { return v.T("role." + string(r)) }

// BCP47 maps the app's own locale codes to full tags for the calendar island (see
// web/vue/main.js), which reads this through lx-ui's createLx() global config rather
// than as a component prop — see the comment in web/vue/CalendarIsland.vue.
func (v View) BCP47() string {
	switch v.P.Locale() {
	case "lv":
		return "lv-LV"
	case "ru":
		return "ru-RU"
	default:
		return "en-US"
	}
}

// TripStatusBadgeClass and BusStatusBadgeClass colour a status pill consistently
// everywhere it appears (tables, the trip page, the timeline legend).
func (v View) TripStatusBadgeClass(s domain.TripStatus) string {
	switch s {
	case domain.TripInProgress:
		return badgeEmerald
	case domain.TripCompleted:
		return badgeSlate
	case domain.TripCancelled:
		return badgeRed
	default:
		return badgeSky
	}
}

func (v View) BusStatusBadgeClass(s domain.BusStatus) string {
	switch s {
	case domain.BusMaintenance:
		return badgeAmber
	case domain.BusRetired:
		return badgeSlate
	default:
		return badgeEmerald
	}
}

const (
	badgeBase    = "inline-flex items-center rounded-full px-2.5 py-0.5 text-xs font-medium ring-1 ring-inset"
	badgeSky     = badgeBase + " bg-sky-50 text-sky-700 ring-sky-600/20"
	badgeEmerald = badgeBase + " bg-emerald-50 text-emerald-700 ring-emerald-600/20"
	badgeSlate   = badgeBase + " bg-slate-100 text-slate-600 ring-slate-500/10"
	badgeRed     = badgeBase + " bg-red-50 text-red-700 ring-red-600/10"
	badgeAmber   = badgeBase + " bg-amber-50 text-amber-700 ring-amber-600/20"
)

func (v View) PayType(p *domain.PayType) string {
	if p == nil {
		return "—"
	}
	return v.T("pay_type." + string(*p))
}

// CanManageFleet and CanManageUsers decide which controls to render. They mirror the
// route guards but are presentation only: every route is enforced server-side by
// RequireRole, so hiding a button is never what keeps anyone out.
func (v View) CanManageFleet() bool { return v.Identity.Role == domain.RoleAdmin }
func (v View) CanManageUsers() bool { return v.Identity.Role == domain.RoleAdmin }
func (v View) CanPlan() bool {
	return v.Identity.Role == domain.RoleAdmin || v.Identity.Role == domain.RoleDispatcher
}

// TripRow is a trip together with its assignment, as the planning list shows it.
type TripRow struct {
	Trip       domain.Trip
	Assignment *domain.AssignmentDetail
}

// TripDetail is the single-trip page: the trip, its current assignment and the
// candidates that can be picked.
type TripDetail struct {
	Trip       domain.Trip
	Assignment *domain.AssignmentDetail
	Buses      []domain.Bus
	Drivers    []domain.Driver
}

// UserForm is the new-user page: the drivers a driver-role account can be linked to.
type UserForm struct {
	Drivers []domain.Driver
}

// ---------------------------------------------------------------------------
// Timeline
// ---------------------------------------------------------------------------

// Timeline is one day of the schedule: rows of buses, blocks positioned by time.
type Timeline struct {
	Day        time.Time
	PrevDate   string
	NextDate   string
	Today      string
	Rows       []TimelineRow
	Unassigned []domain.Trip
	Hours      []int
	// CSS positions the blocks. It is generated from computed numbers and rendered in
	// a nonce'd <style> element, because the CSP allows no inline style attributes.
	CSS string
	// HasBlocks is false when nothing is scheduled on this day at all (distinct from
	// "no buses exist"), so the page can tell "nothing today" apart from "nothing ever".
	HasBlocks bool
	// NextTripDate, set only when HasBlocks is false, is the nearest upcoming day
	// (YYYY-MM-DD) that has a booking, so an empty day is never a dead end.
	NextTripDate string
}

type TimelineRow struct {
	Bus    domain.Bus
	Blocks []TimelineBlock
}

type TimelineBlock struct {
	TripID          int64
	Origin          string
	Destination     string
	DriverName      string
	Start           time.Time
	End             time.Time
	Status          domain.TripStatus
	LeftPct         float64
	WidthPct        float64
	ContinuesBefore bool
	ContinuesAfter  bool
}

// BlockID is the element id a generated CSS rule targets. Derived from two integers, so
// it is always a safe identifier.
func BlockID(busID, tripID int64) string {
	return "tb-" + strconv.FormatInt(busID, 10) + "-" + strconv.FormatInt(tripID, 10)
}

// BlockClass colours a block by status. Cancelled trips are never drawn.
func (b TimelineBlock) BlockClass() string {
	switch b.Status {
	case domain.TripInProgress:
		return "bg-emerald-600 hover:bg-emerald-700"
	case domain.TripCompleted:
		return "bg-slate-500 hover:bg-slate-600"
	default:
		return "bg-sky-700 hover:bg-sky-800"
	}
}

// Label is what fits inside the block; the full detail is in the title attribute.
func (b TimelineBlock) Label() string {
	return b.Origin + " → " + b.Destination
}

// TimelineStyle renders the generated positioning rules as a nonce'd <style> element.
//
// It is built here rather than in the .templ file because templ emits the content of a
// style element as literal text. The CSS comes from timelineCSS, which formats floats
// and integers only; the guard below is belt and braces so this can never become an
// injection point if that ever changes.
func TimelineStyle(nonce, css string) templ.Component {
	if strings.ContainsAny(css, "<>") {
		css = "" // refuse to render anything that could close the element
	}
	return templ.Raw(`<style nonce="` + html.EscapeString(nonce) + `">` + css + `</style>`)
}
