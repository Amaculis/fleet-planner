package inttest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

// These tests drive the real router — middleware, CSRF, RBAC, services, database — and
// assert what each role can reach. Nothing is stubbed: if a route were mounted outside
// its guard, these would catch it.

const testPassword = "correct-horse-battery-staple"

var csrfPattern = regexp.MustCompile(`name="csrf_token" value="([^"]+)"`)

// client is a browser-like client: it keeps cookies and does not follow redirects, so
// tests can assert on the redirect itself.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func newClient(t *testing.T, base string) *client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatalf("creating cookie jar: %v", err)
	}
	return &client{
		t:    t,
		base: base,
		http: &http.Client{
			Jar:           jar,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
}

func (c *client) get(path string) *http.Response {
	c.t.Helper()
	resp, err := c.http.Get(c.base + path)
	if err != nil {
		c.t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func (c *client) body(path string) string {
	c.t.Helper()
	resp := c.get(path)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("reading %s: %v", path, err)
	}
	return string(body)
}

// post sends a form with the CSRF token taken from the given page, exactly as a browser
// would after rendering it.
func (c *client) post(tokenFrom, path string, form url.Values) *http.Response {
	c.t.Helper()
	form.Set("csrf_token", c.csrfToken(tokenFrom))
	resp, err := c.http.PostForm(c.base+path, form)
	if err != nil {
		c.t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func (c *client) csrfToken(page string) string {
	c.t.Helper()
	match := csrfPattern.FindStringSubmatch(c.body(page))
	if match == nil {
		c.t.Fatalf("no CSRF token on %s", page)
	}
	return match[1]
}

func (c *client) login(email string) {
	c.t.Helper()
	resp := c.post("/login", "/login", url.Values{"email": {email}, "password": {testPassword}})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		c.t.Fatalf("login as %s: status %d, want 303", email, resp.StatusCode)
	}
}

func (e *env) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(e.server.Routes())
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestRoleBoundaries walks every role over every route group.
func TestRoleBoundaries(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	admin := e.newAdmin("admin@example.lv", testPassword)
	e.newDispatcher("dispatcher@example.lv", testPassword)
	driver, _ := e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)

	// One assigned trip so the pages have something to render.
	bus := e.newBus("AA-1111")
	trip := e.newTrip("Riga", "Liepaja", 8, 12)
	if _, err := e.assignments.Assign(ctx, e.identityFor(admin, nil), trip.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	base := e.serve(t)
	tripPath := "/trips/" + strconv.FormatInt(trip.ID, 10)

	tests := []struct {
		email string
		path  string
		want  int
	}{
		// Admin reaches everything.
		{"admin@example.lv", "/trips", http.StatusOK},
		{"admin@example.lv", tripPath, http.StatusOK},
		{"admin@example.lv", "/buses", http.StatusOK},
		{"admin@example.lv", "/buses/new", http.StatusOK},
		{"admin@example.lv", "/drivers", http.StatusOK},
		{"admin@example.lv", "/drivers/new", http.StatusOK},
		{"admin@example.lv", "/users", http.StatusOK},
		{"admin@example.lv", "/users/new", http.StatusOK},

		// A dispatcher plans, but cannot manage the fleet or accounts.
		{"dispatcher@example.lv", "/trips", http.StatusOK},
		{"dispatcher@example.lv", tripPath, http.StatusOK},
		{"dispatcher@example.lv", "/buses", http.StatusOK},
		{"dispatcher@example.lv", "/drivers", http.StatusOK},
		{"dispatcher@example.lv", "/buses/new", http.StatusForbidden},
		{"dispatcher@example.lv", "/drivers/new", http.StatusForbidden},
		{"dispatcher@example.lv", "/users", http.StatusForbidden},
		{"dispatcher@example.lv", "/users/new", http.StatusForbidden},

		// A driver sees their own trips and nothing else.
		{"driver@example.lv", "/my/trips", http.StatusOK},
		{"driver@example.lv", "/trips", http.StatusForbidden},
		{"driver@example.lv", tripPath, http.StatusForbidden},
		{"driver@example.lv", "/buses", http.StatusForbidden},
		{"driver@example.lv", "/drivers", http.StatusForbidden},
		{"driver@example.lv", "/users", http.StatusForbidden},
		{"driver@example.lv", "/users/new", http.StatusForbidden},

		// Planners have no driver view.
		{"admin@example.lv", "/my/trips", http.StatusForbidden},
		{"dispatcher@example.lv", "/my/trips", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.email+" GET "+tt.path, func(t *testing.T) {
			c := newClient(t, base)
			c.login(tt.email)

			resp := c.get(tt.path)
			defer resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Fatalf("GET %s as %s: status %d, want %d", tt.path, tt.email, resp.StatusCode, tt.want)
			}
		})
	}
}

// TestAnonymousCannotReachAnything: every non-public route redirects to the login page.
func TestAnonymousCannotReachAnything(t *testing.T) {
	e := newEnv(t)
	base := e.serve(t)

	for _, path := range []string{"/", "/trips", "/trips/1", "/buses", "/drivers", "/users", "/my/trips"} {
		c := newClient(t, base)
		resp := c.get(path)
		resp.Body.Close()
		if resp.StatusCode != http.StatusSeeOther {
			t.Errorf("GET %s anonymously: status %d, want 303", path, resp.StatusCode)
		}
		if location := resp.Header.Get("Location"); location != "/login" {
			t.Errorf("GET %s anonymously redirected to %q, want /login", path, location)
		}
	}
}

// TestDriverCannotReachAnotherDriversData is the isolation requirement: driver B must
// not be able to see or touch driver A's trip, even with the real trip id in hand.
func TestDriverCannotReachAnotherDriversData(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	driverA, driverAUser := e.newDriverWithLogin("Driver A", "a@example.lv", testPassword)
	driverB, _ := e.newDriverWithLogin("Driver B", "b@example.lv", testPassword)

	busA := e.newBus("AA-1111")
	busB := e.newBus("BB-2222")
	tripA := e.newTrip("Riga", "Liepaja", 8, 12)
	tripB := e.newTrip("Riga", "Ventspils", 8, 12)

	if _, err := e.assignments.Assign(ctx, actor, tripA.ID, busA.ID, driverA.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning driver A: %v", err)
	}
	if _, err := e.assignments.Assign(ctx, actor, tripB.ID, busB.ID, driverB.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning driver B: %v", err)
	}

	base := e.serve(t)
	c := newClient(t, base)
	c.login("b@example.lv")

	t.Run("the list shows only the caller's own trips", func(t *testing.T) {
		body := c.body("/my/trips")
		if !strings.Contains(body, "Ventspils") {
			t.Error("driver B cannot see their own trip")
		}
		if strings.Contains(body, "Liepaja") {
			t.Error("driver B can see driver A's trip")
		}
		if strings.Contains(body, busA.Plate) {
			t.Error("driver B can see the bus from driver A's trip")
		}
	})

	t.Run("starting another driver's trip is refused", func(t *testing.T) {
		resp := c.post("/my/trips", "/my/trips/"+strconv.FormatInt(tripA.ID, 10)+"/start", url.Values{})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d, want 404", resp.StatusCode)
		}

		// And driver A's trip is untouched.
		after, err := e.trips.Get(ctx, actor, tripA.ID)
		if err != nil {
			t.Fatalf("reading trip A: %v", err)
		}
		if after.Status != domain.TripPlanned || after.ActualStart != nil {
			t.Fatalf("driver B modified driver A's trip: status=%s actual_start=%v", after.Status, after.ActualStart)
		}
	})

	t.Run("finishing another driver's trip is refused", func(t *testing.T) {
		if _, err := e.trips.StartMine(ctx, e.identityFor(driverAUser, &driverA.ID), tripA.ID, service.Meta{}); err != nil {
			t.Fatalf("driver A starting their own trip: %v", err)
		}
		resp := c.post("/my/trips", "/my/trips/"+strconv.FormatInt(tripA.ID, 10)+"/finish", url.Values{})
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("status %d, want 404", resp.StatusCode)
		}
		after, err := e.trips.Get(ctx, actor, tripA.ID)
		if err != nil {
			t.Fatalf("reading trip A: %v", err)
		}
		if after.ActualEnd != nil {
			t.Fatal("driver B closed driver A's trip")
		}
	})

	t.Run("the service refuses even when handed another driver's id directly", func(t *testing.T) {
		// Driver B's identity, driver A's trip: the ownership filter is inside the SQL.
		identityB := domain.Identity{UserID: 99, Role: domain.RoleDriver, DriverID: &driverB.ID}
		if _, err := e.trips.StartMine(ctx, identityB, tripA.ID, service.Meta{}); !errors.Is(err, domain.ErrNotFound) {
			t.Fatalf("expected not-found, got: %v", err)
		}

		// A driver identity with no linked driver record is forbidden, not "all drivers".
		orphan := domain.Identity{UserID: 98, Role: domain.RoleDriver}
		if _, err := e.trips.ListMine(ctx, orphan, baseDay); !errors.Is(err, domain.ErrForbidden) {
			t.Fatalf("expected forbidden, got: %v", err)
		}
	})
}

// TestDriverOwnTripLifecycle: the happy path a driver actually uses, and the audit trail
// it leaves behind (payroll-seam).
func TestDriverOwnTripLifecycle(t *testing.T) {
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

	tripPath := "/my/trips/" + strconv.FormatInt(trip.ID, 10)

	resp := c.post("/my/trips", tripPath+"/start", url.Values{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("starting own trip: status %d, want 303", resp.StatusCode)
	}

	started, err := e.trips.Get(ctx, actor, trip.ID)
	if err != nil {
		t.Fatalf("reading trip: %v", err)
	}
	if started.Status != domain.TripInProgress || started.ActualStart == nil {
		t.Fatalf("trip not started: status=%s actual_start=%v", started.Status, started.ActualStart)
	}

	resp = c.post("/my/trips", tripPath+"/finish", url.Values{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("finishing own trip: status %d, want 303", resp.StatusCode)
	}

	finished, err := e.trips.Get(ctx, actor, trip.ID)
	if err != nil {
		t.Fatalf("reading trip: %v", err)
	}
	if finished.Status != domain.TripCompleted || finished.ActualEnd == nil {
		t.Fatalf("trip not finished: status=%s actual_end=%v", finished.Status, finished.ActualEnd)
	}
	// payroll-seam: the worked window is now recorded against this driver.
	if finished.WorkedDuration() <= 0 {
		t.Error("the completed trip has no worked duration for payroll to use")
	}

	actions := e.auditActions("trip", trip.ID)
	if len(actions) < 2 || actions[len(actions)-2] != "start" || actions[len(actions)-1] != "finish" {
		t.Errorf("audit actions = %v, want ... start finish", actions)
	}
}

// TestDriverErasureKeepsHistory covers the GDPR path: personal data goes, the trip
// history and the payroll linkage stay, and the driver's login stops working.
func TestDriverErasureKeepsHistory(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	driver, _ := e.newDriverWithLogin("Jānis Bērziņš", "janis@example.lv", testPassword)

	bus := e.newBus("AA-1111")
	trip := e.newTrip("Riga", "Liepaja", 8, 12)
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	base := e.serve(t)
	driverClient := newClient(t, base)
	driverClient.login("janis@example.lv")
	if resp := driverClient.get("/my/trips"); resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		t.Fatalf("driver cannot reach their trips before erasure: %d", resp.StatusCode)
	}

	erased, err := e.fleet.AnonymizeDriver(ctx, actor, driver.ID, service.Meta{})
	if err != nil {
		t.Fatalf("anonymizing: %v", err)
	}

	if erased.Phone != nil || erased.LicenseNumber != nil || erased.LicenseExpiry != nil {
		t.Error("personal data survived erasure")
	}
	if strings.Contains(erased.FullName, "Bērziņš") {
		t.Error("the name survived erasure")
	}
	if !erased.IsAnonymized() || erased.IsActive {
		t.Error("an erased driver must be marked anonymised and inactive")
	}

	// The assignment — and therefore the worked time payroll needs — still points at
	// the same driver row.
	assignment, err := e.assignments.GetForTrip(ctx, actor, trip.ID)
	if err != nil {
		t.Fatalf("the assignment did not survive erasure: %v", err)
	}
	if assignment.DriverID != driver.ID {
		t.Fatalf("assignment now points at driver %d, want %d", assignment.DriverID, driver.ID)
	}

	// The login is dead: the session was revoked and the account deactivated.
	resp := driverClient.get("/my/trips")
	resp.Body.Close()
	if resp.StatusCode != http.StatusSeeOther {
		t.Fatalf("an erased driver's live session still works: status %d", resp.StatusCode)
	}
	fresh := newClient(t, base)
	loginResp := fresh.post("/login", "/login", url.Values{"email": {"janis@example.lv"}, "password": {testPassword}})
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("an erased driver could still sign in: status %d", loginResp.StatusCode)
	}

	if actions := e.auditActions("driver", driver.ID); len(actions) == 0 || actions[len(actions)-1] != "anonymize" {
		t.Errorf("erasure was not audited: %v", actions)
	}
}
