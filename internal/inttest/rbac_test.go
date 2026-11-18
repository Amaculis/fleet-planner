package inttest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
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

func (e *env) serve(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(e.server.Routes())
	t.Cleanup(srv.Close)
	return srv.URL
}

// TestRoleBoundaries walks every role over the API routes of each role group.
func TestRoleBoundaries(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)

	admin := e.newAdmin("admin@example.lv", testPassword)
	e.newDispatcher("dispatcher@example.lv", testPassword)
	driver, _ := e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)

	// One assigned trip so the endpoints have something to return.
	bus := e.newBus("AA-1111")
	trip := e.newTrip("Riga", "Liepaja", 8, 12)
	if _, err := e.assignments.Assign(ctx, e.identityFor(admin, nil), trip.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	base := e.serve(t)
	tripPath := "/api/trips/" + strconv.FormatInt(trip.ID, 10)
	busPath := "/api/buses/" + strconv.FormatInt(bus.ID, 10)
	driverPath := "/api/drivers/" + strconv.FormatInt(driver.ID, 10)

	tests := []struct {
		email string
		path  string
		want  int
	}{
		// Admin reaches everything.
		{"admin@example.lv", "/api/trips", http.StatusOK},
		{"admin@example.lv", tripPath, http.StatusOK},
		{"admin@example.lv", "/api/buses", http.StatusOK},
		{"admin@example.lv", busPath, http.StatusOK},
		{"admin@example.lv", "/api/drivers", http.StatusOK},
		{"admin@example.lv", driverPath, http.StatusOK},
		{"admin@example.lv", "/api/clients", http.StatusOK},
		{"admin@example.lv", "/api/users", http.StatusOK},

		// A dispatcher plans and can list the fleet, but cannot read single fleet records
		// (those carry pay and licence data) or manage accounts.
		{"dispatcher@example.lv", "/api/trips", http.StatusOK},
		{"dispatcher@example.lv", tripPath, http.StatusOK},
		{"dispatcher@example.lv", "/api/buses", http.StatusOK},
		{"dispatcher@example.lv", "/api/drivers", http.StatusOK},
		{"dispatcher@example.lv", "/api/clients", http.StatusOK},
		{"dispatcher@example.lv", busPath, http.StatusForbidden},
		{"dispatcher@example.lv", driverPath, http.StatusForbidden},
		{"dispatcher@example.lv", "/api/users", http.StatusForbidden},

		// A driver sees their own trips and nothing else.
		{"driver@example.lv", "/api/my/trips", http.StatusOK},
		{"driver@example.lv", "/api/trips", http.StatusForbidden},
		{"driver@example.lv", tripPath, http.StatusForbidden},
		{"driver@example.lv", "/api/buses", http.StatusForbidden},
		{"driver@example.lv", "/api/drivers", http.StatusForbidden},
		{"driver@example.lv", "/api/clients", http.StatusForbidden},
		{"driver@example.lv", "/api/users", http.StatusForbidden},

		// Planners have no driver view.
		{"admin@example.lv", "/api/my/trips", http.StatusForbidden},
		{"dispatcher@example.lv", "/api/my/trips", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.email+" GET "+tt.path, func(t *testing.T) {
			c := &apiClient{newClient(t, base)}
			c.apiLogin(tt.email)

			resp := c.get(tt.path)
			defer resp.Body.Close()
			if resp.StatusCode != tt.want {
				t.Fatalf("GET %s as %s: status %d, want %d", tt.path, tt.email, resp.StatusCode, tt.want)
			}
		})
	}
}

// TestAnonymousCannotReachAnything: every API route answers 401 without a session.
func TestAnonymousCannotReachAnything(t *testing.T) {
	e := newEnv(t)
	base := e.serve(t)

	for _, path := range []string{"/api/trips", "/api/trips/1", "/api/buses", "/api/drivers", "/api/clients", "/api/users", "/api/my/trips"} {
		c := newClient(t, base)
		resp := c.get(path)
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s anonymously: status %d, want 401", path, resp.StatusCode)
		}
	}
}

// TestOldUIAddressesLeadIntoThePortal: bookmarks to the retired server-rendered UI do
// not 404 or reach any data; they redirect into the portal.
func TestOldUIAddressesLeadIntoThePortal(t *testing.T) {
	e := newEnv(t)
	base := e.serve(t)

	for path, want := range map[string]string{"/": "/app/", "/login": "/app/login"} {
		resp := newClient(t, base).get(path)
		resp.Body.Close()
		if resp.StatusCode != http.StatusFound || resp.Header.Get("Location") != want {
			t.Errorf("GET %s: status %d, Location %q; want 302 to %s", path, resp.StatusCode, resp.Header.Get("Location"), want)
		}
	}

	// A worker the old UI registered on someone's phone is replaced by one that removes it.
	resp := newClient(t, base).get("/sw.js")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "unregister") {
		t.Errorf("GET /sw.js: status %d; want the self-unregistering worker", resp.StatusCode)
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
	c := &apiClient{newClient(t, base)}
	csrf := c.apiLogin("b@example.lv")

	t.Run("the list shows only the caller's own trips", func(t *testing.T) {
		body := readBody(t, c.get("/api/my/trips"))
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
		resp := c.postJSON("/api/my/trips/"+strconv.FormatInt(tripA.ID, 10)+"/start", csrf, struct{}{})
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
		resp := c.postJSON("/api/my/trips/"+strconv.FormatInt(tripA.ID, 10)+"/finish", csrf, struct{}{})
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
	c := &apiClient{newClient(t, base)}
	csrf := c.apiLogin("driver@example.lv")

	tripPath := "/api/my/trips/" + strconv.FormatInt(trip.ID, 10)

	resp := c.postJSON(tripPath+"/start", csrf, struct{}{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("starting own trip: status %d, want 204", resp.StatusCode)
	}

	started, err := e.trips.Get(ctx, actor, trip.ID)
	if err != nil {
		t.Fatalf("reading trip: %v", err)
	}
	if started.Status != domain.TripInProgress || started.ActualStart == nil {
		t.Fatalf("trip not started: status=%s actual_start=%v", started.Status, started.ActualStart)
	}

	resp = c.postJSON(tripPath+"/finish", csrf, struct{}{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("finishing own trip: status %d, want 204", resp.StatusCode)
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
	driverClient := &apiClient{newClient(t, base)}
	driverClient.apiLogin("janis@example.lv")
	if resp := driverClient.get("/api/my/trips"); resp.StatusCode != http.StatusOK {
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
	resp := driverClient.get("/api/my/trips")
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("an erased driver's live session still works: status %d", resp.StatusCode)
	}
	fresh := &apiClient{newClient(t, base)}
	loginResp := fresh.postJSON("/api/auth/login", fresh.me().CSRFToken, map[string]string{
		"email": "janis@example.lv", "password": testPassword,
	})
	loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("an erased driver could still sign in: status %d", loginResp.StatusCode)
	}

	if actions := e.auditActions("driver", driver.ID); len(actions) == 0 || actions[len(actions)-1] != "anonymize" {
		t.Errorf("erasure was not audited: %v", actions)
	}
}
