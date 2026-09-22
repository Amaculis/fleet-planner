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
