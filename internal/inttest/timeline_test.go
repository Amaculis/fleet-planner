package inttest

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/buscompany/bus_fleet/internal/service"
)

// TestTimelineRendersTheDay drives the real page: a planner sees each bus as a row with
// its trips positioned, and the trips nobody is driving yet listed separately.
func TestTimelineRendersTheDay(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)

	busA := e.newBus("AA-1111")
	e.newBus("BB-2222")
	driver := e.newDriver("Driver A")

	assigned := e.newTrip("Riga", "Liepaja", 8, 12)
	e.newTrip("Riga", "Jelgava", 14, 16) // left unassigned on purpose
	if _, err := e.assignments.Assign(ctx, actor, assigned.ID, busA.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	base := e.serve(t)
	c := newClient(t, base)
	c.login("admin@example.lv")

	// baseDay is the day the fixtures live on.
	body := c.body("/timeline?date=" + baseDay.Format("2006-01-02"))

	for _, want := range []string{
		"AA-1111", "BB-2222", // every bus is a row
		"Liepaja",  // the assigned trip is drawn
		"Driver A", // with its driver
		"Jelgava",  // the unassigned trip is called out
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the timeline is missing %q", want)
		}
	}

	// The block is positioned by the generated stylesheet, not an inline style.
	if !strings.Contains(body, "left:33.3333%;width:16.6667%") {
		t.Error("the 08:00-12:00 block is not positioned across a third of the day")
	}
	if strings.Contains(body, "style=\"left") {
		t.Error("a block used an inline style attribute, which the CSP forbids")
	}
	if strings.Contains(body, "Nothing scheduled on this day.") {
		t.Error("the empty-day hint showed on a day that has a trip")
	}

	// A different day shows no blocks.
	empty := c.body("/timeline?date=" + baseDay.AddDate(0, 0, 7).Format("2006-01-02"))
	if strings.Contains(empty, "Liepaja") {
		t.Error("a trip appeared on a day it does not belong to")
	}

	// A junk date falls back to today rather than erroring.
	resp := c.get("/timeline?date=not-a-date")
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("a malformed date gave status %d, want 200", resp.StatusCode)
	}
}

// TestTimelineEmptyDaySuggestsNextTrip: a dispatcher opening the timeline on a day with
// nothing scheduled must not be left guessing which day to click to next — the page
// itself points at the nearest day that has a booking.
func TestTimelineEmptyDaySuggestsNextTrip(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	bus := e.newBus("AA-1111")
	driver := e.newDriver("Driver A")

	// A trip three days out, assigned so it counts as a real booking.
	future := e.newTrip("Riga", "Moscow", 24*3+8, 24*3+12)
	if _, err := e.assignments.Assign(ctx, actor, future.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	base := e.serve(t)
	c := newClient(t, base)
	c.login("admin@example.lv")

	// baseDay itself has nothing on it.
	body := c.body("/timeline?date=" + baseDay.Format("2006-01-02"))

	wantDate := baseDay.AddDate(0, 0, 3).Format("2006-01-02")
	if !strings.Contains(body, "Nothing scheduled on this day.") {
		t.Error("an empty day gave no indication that it was empty")
	}
	if !strings.Contains(body, "/timeline?date="+wantDate) {
		t.Errorf("the empty-day hint does not link to %s, the day with the next trip", wantDate)
	}

	// A day with nothing booked anywhere in the future gets no dangling suggestion.
	farBody := c.body("/timeline?date=" + baseDay.AddDate(0, 0, 30).Format("2006-01-02"))
	if !strings.Contains(farBody, "Nothing scheduled on this day.") {
		t.Error("a day after the last trip should still say it is empty")
	}
	if strings.Count(farBody, "timeline?date=") > 3 {
		// prev/today/next nav links only — no extra "jump to next trip" link.
		t.Error("a day with no future trips at all should not suggest a next one")
	}
}

// TestTimelineIsClosedToDrivers: the planning view is not part of a driver's world.
func TestTimelineIsClosedToDrivers(t *testing.T) {
	e := newEnv(t)
	e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)

	base := e.serve(t)
	c := newClient(t, base)
	c.login("driver@example.lv")

	resp := c.get("/timeline")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("a driver reached the timeline: status %d, want 403", resp.StatusCode)
	}
}

// TestCalendarIslandProgressiveEnhancement checks the one contract that matters for the
// lx-ui island: the page must be fully usable before any JavaScript runs. It cannot
// drive an actual browser (no headless browser is available in this environment), so it
// verifies the HTML the server sends and that the island's assets are actually served —
// not that the widget renders or behaves once mounted. See docs/lx-ui-integration.md.
func TestCalendarIslandProgressiveEnhancement(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	c := newClient(t, base)
	c.login("admin@example.lv")

	body := c.body("/timeline")

	if !strings.Contains(body, `id="date" name="date" type="date"`) {
		t.Fatal("the real, working date input is missing — without it, a failed island mount leaves no way to pick a date")
	}
	if !strings.Contains(body, `data-kind="calendar"`) {
		t.Fatal("the calendar island's mount point is missing")
	}
	// The mount point must start hidden: main.js only reveals it after a successful
	// mount, so a page where the script never runs must show the native input instead.
	if idx := strings.Index(body, `data-kind="calendar"`); idx == -1 || !strings.Contains(body[max(0, idx-40):idx], "hidden") {
		t.Error("the calendar island's mount point does not start hidden")
	}
	if !strings.Contains(body, `<link rel="stylesheet" href="/static/css/calendar-island.css"`) {
		t.Error("the island's stylesheet is not linked")
	}
	if !strings.Contains(body, `src="/static/js/calendar-island.js"`) {
		t.Error("the island's script is not linked")
	}
	// The script must carry the same per-request CSP nonce as every other script on
	// the page — the CSP allows no unnonced script to execute.
	if !strings.Contains(body, `type="module" nonce=`) {
		t.Error("the island's script tag carries no nonce and would be blocked by the CSP")
	}

	for _, asset := range []string{"/static/js/calendar-island.js", "/static/css/calendar-island.css"} {
		resp := c.get(asset)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET %s: status %d, want 200 (run `npm run build:calendar` if this is a fresh checkout)", asset, resp.StatusCode)
		}
	}
}

// TestPWAEndpoints: the driver PWA needs its worker at the root and an offline page
// that works without a session.
func TestPWAEndpoints(t *testing.T) {
	e := newEnv(t)
	base := e.serve(t)
	c := newClient(t, base)

	t.Run("the service worker is served from the root", func(t *testing.T) {
		resp := c.get("/sw.js")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d, want 200", resp.StatusCode)
		}
		if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/javascript") {
			t.Errorf("Content-Type = %q", ct)
		}
		if allowed := resp.Header.Get("Service-Worker-Allowed"); allowed != "/" {
			t.Errorf("Service-Worker-Allowed = %q, want /", allowed)
		}
		if cache := resp.Header.Get("Cache-Control"); !strings.Contains(cache, "no-cache") {
			t.Errorf("Cache-Control = %q, want no-cache so updates are picked up", cache)
		}
	})

	t.Run("the offline page needs no session", func(t *testing.T) {
		resp := c.get("/offline")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("status %d, want 200", resp.StatusCode)
		}
	})

	t.Run("the manifest is served", func(t *testing.T) {
		body := c.body("/static/manifest.webmanifest")
		for _, want := range []string{"\"start_url\": \"/my/trips\"", "icon-512.png", "standalone"} {
			if !strings.Contains(body, want) {
				t.Errorf("manifest is missing %q", want)
			}
		}
	})

	t.Run("the icons are real PNGs", func(t *testing.T) {
		for _, icon := range []string{"icon-192.png", "icon-512.png", "apple-touch-icon.png"} {
			body := c.body("/static/icons/" + icon)
			if !strings.HasPrefix(body, "\x89PNG\r\n\x1a\n") {
				t.Errorf("%s is not a PNG (%d bytes)", icon, len(body))
			}
		}
	})

	// lx-ui's pre-compiled dist bundle requests its own lazy component chunks at the
	// site root (/js/calendar-assets/..., /css/...) rather than under /static/ — see
	// docs/lx-ui-integration.md. The same embedded tree is mirrored at root so those
	// requests resolve regardless of which URL scheme lx-ui's runtime actually uses.
	t.Run("the static tree is mirrored at /js/ and /css/ for lx-ui's own chunk loader", func(t *testing.T) {
		resp := c.get("/js/calendar-island.js")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /js/calendar-island.js: status %d, want 200", resp.StatusCode)
		}
		resp2 := c.get("/css/calendar-island.css")
		defer resp2.Body.Close()
		if resp2.StatusCode != http.StatusOK {
			t.Errorf("GET /css/calendar-island.css: status %d, want 200", resp2.StatusCode)
		}
		resp3 := c.get("/lx-fonts/IBMPlexSansVar.ttf")
		defer resp3.Body.Close()
		if resp3.StatusCode != http.StatusOK {
			t.Errorf("GET /lx-fonts/IBMPlexSansVar.ttf: status %d, want 200", resp3.StatusCode)
		}
	})
}

// TestDriverViewOnAPhone: the driver's page carries the actions a thumb can hit, and
// the trip id in the action always belongs to the signed-in driver.
func TestDriverViewOnAPhone(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	driver, _ := e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)
	bus := e.newBus("AA-1111")
	trip := e.newTrip("Riga", "Liepaja", 8, 12)
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	base := e.serve(t)
	c := newClient(t, base)
	c.login("driver@example.lv")

	body := c.body("/my/trips")
	action := "/my/trips/" + strconv.FormatInt(trip.ID, 10) + "/start"
	if !strings.Contains(body, action) {
		t.Errorf("the start action is missing from the driver view")
	}
	if !strings.Contains(body, "AA-1111") {
		t.Error("the driver cannot see which bus to take")
	}
	if !strings.Contains(body, "manifest.webmanifest") {
		t.Error("the driver page does not link the PWA manifest")
	}
}
