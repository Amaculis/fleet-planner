package inttest

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

// messageKey returns the i18n key of a service MessageError, or "".
func messageKey(err error) string {
	if msg, ok := service.AsMessage(err); ok {
		return msg.Key
	}
	return ""
}

// TestManualStatusKeepsWorkedTimeConsistent: a planner moving a trip by hand must leave
// actual_start/actual_end (payroll-seam) matching the status, so the driver can still
// finish it and payroll never sees a completed trip without times.
func TestManualStatusKeepsWorkedTimeConsistent(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	dispatcher := e.newDispatcher("dispatcher@example.lv", testPassword)
	actor := e.identityFor(dispatcher, nil)
	driver, login := e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)
	driverActor := e.identityFor(login, &driver.ID)
	bus := e.newBus("AA-1111")
	trip := e.newTrip("Riga", "Liepaja", 8, 12)
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	started, err := e.trips.SetStatus(ctx, actor, trip.ID, domain.TripInProgress, service.Meta{})
	if err != nil {
		t.Fatalf("planner starting the trip: %v", err)
	}
	if started.ActualStart == nil || started.ActualEnd != nil {
		t.Fatalf("planned -> in_progress must record a start only, got start=%v end=%v", started.ActualStart, started.ActualEnd)
	}

	// The driver can now finish it — this used to fail on the actual_range CHECK.
	finished, err := e.trips.FinishMine(ctx, driverActor, trip.ID, service.Meta{})
	if err != nil {
		t.Fatalf("driver finishing a planner-started trip: %v", err)
	}
	if finished.WorkedDuration() <= 0 {
		t.Fatalf("finished trip has no worked time: %v", finished.WorkedDuration())
	}

	// Reopen clears the end only.
	reopened, err := e.trips.SetStatus(ctx, actor, trip.ID, domain.TripInProgress, service.Meta{})
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	if reopened.ActualStart == nil || reopened.ActualEnd != nil {
		t.Fatalf("completed -> in_progress must keep the start and clear the end")
	}

	// Planner completes: end recorded.
	completed, err := e.trips.SetStatus(ctx, actor, trip.ID, domain.TripCompleted, service.Meta{})
	if err != nil {
		t.Fatalf("planner completing: %v", err)
	}
	if completed.ActualEnd == nil {
		t.Fatal("in_progress -> completed must record an end")
	}

	// Cancel mid-trip, then un-cancel: back to a clean planned trip.
	other := e.newTrip("Riga", "Cesis", 14, 16)
	if _, err := e.trips.SetStatus(ctx, actor, other.ID, domain.TripInProgress, service.Meta{}); err != nil {
		t.Fatalf("starting second trip: %v", err)
	}
	cancelled, err := e.trips.SetStatus(ctx, actor, other.ID, domain.TripCancelled, service.Meta{})
	if err != nil {
		t.Fatalf("cancelling a started trip: %v", err)
	}
	if cancelled.ActualStart == nil || cancelled.ActualEnd == nil {
		t.Fatal("cancelling a started trip must keep the time driven")
	}
	replanned, err := e.trips.SetStatus(ctx, actor, other.ID, domain.TripPlanned, service.Meta{})
	if err != nil {
		t.Fatalf("un-cancelling a trip that had started: %v", err)
	}
	if replanned.ActualStart != nil || replanned.ActualEnd != nil {
		t.Fatal("cancelled -> planned must clear the worked-time window")
	}
}

// TestTripInProgressKeepsItsDriver: once started, the driver's worked time must not be
// handed to someone else or orphaned. A bus swap stays possible.
func TestTripInProgressKeepsItsDriver(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	dispatcher := e.newDispatcher("dispatcher@example.lv", testPassword)
	actor := e.identityFor(dispatcher, nil)
	admin := e.newAdmin("admin@example.lv", testPassword)
	adminActor := e.identityFor(admin, nil)

	bus, otherBus := e.newBus("AA-1111"), e.newBus("BB-2222")
	driver, otherDriver := e.newDriver("Driver A"), e.newDriver("Driver B")
	trip := e.newTrip("Riga", "Liepaja", 8, 12)
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}
	if _, err := e.trips.SetStatus(ctx, actor, trip.ID, domain.TripInProgress, service.Meta{}); err != nil {
		t.Fatalf("starting: %v", err)
	}

	if err := e.assignments.Unassign(ctx, actor, trip.ID, service.Meta{}); messageKey(err) != "error.trip_in_progress_keeps_driver" {
		t.Errorf("unassigning a trip in progress should be refused, got: %v", err)
	}
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, bus.ID, otherDriver.ID, service.Meta{}); messageKey(err) != "error.trip_in_progress_keeps_driver" {
		t.Errorf("a dispatcher swapping the driver mid-trip should be refused, got: %v", err)
	}
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, otherBus.ID, driver.ID, service.Meta{}); err != nil {
		t.Errorf("swapping the bus mid-trip should be allowed: %v", err)
	}
	if _, err := e.assignments.Assign(ctx, adminActor, trip.ID, otherBus.ID, otherDriver.ID, service.Meta{}); err != nil {
		t.Errorf("an admin correcting the driver should be allowed: %v", err)
	}
}

// TestDriverStartFinishAreIdempotentAndExplained: a double tap says "already started"
// instead of "not found", and a stale in-progress trip never drops off the list.
func TestDriverStartFinishAreIdempotentAndExplained(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	dispatcher := e.newDispatcher("dispatcher@example.lv", testPassword)
	actor := e.identityFor(dispatcher, nil)
	driver, login := e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)
	me := e.identityFor(login, &driver.ID)
	bus := e.newBus("AA-1111")
	trip := e.newTrip("Riga", "Liepaja", 8, 12)
	cancelled := e.newTrip("Riga", "Cesis", 14, 16)
	for _, tr := range []domain.Trip{trip, cancelled} {
		if _, err := e.assignments.Assign(ctx, actor, tr.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
			t.Fatalf("assigning: %v", err)
		}
	}
	if _, err := e.trips.SetStatus(ctx, actor, cancelled.ID, domain.TripCancelled, service.Meta{}); err != nil {
		t.Fatalf("cancelling: %v", err)
	}

	if _, err := e.trips.FinishMine(ctx, me, trip.ID, service.Meta{}); messageKey(err) != "error.trip_not_started_yet" {
		t.Errorf("finishing an unstarted trip: got %v", err)
	}
	if _, err := e.trips.StartMine(ctx, me, trip.ID, service.Meta{}); err != nil {
		t.Fatalf("starting: %v", err)
	}
	if _, err := e.trips.StartMine(ctx, me, trip.ID, service.Meta{}); messageKey(err) != "error.trip_already_started" {
		t.Errorf("second start: got %v", err)
	}
	if _, err := e.trips.StartMine(ctx, me, cancelled.ID, service.Meta{}); messageKey(err) != "error.trip_cancelled_by_planner" {
		t.Errorf("starting a cancelled trip: got %v", err)
	}

	// Long after its window, the in-progress trip is still listed; the cancelled one
	// shows up too, so the driver learns about the cancellation.
	mine, err := e.trips.ListMine(ctx, me, at(0).AddDate(0, 0, 30))
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	if len(mine) != 1 || mine[0].ID != trip.ID {
		t.Fatalf("an old in-progress trip must stay listed, got %d trips", len(mine))
	}
	mine, err = e.trips.ListMine(ctx, me, at(0).Add(-time.Hour))
	if err != nil {
		t.Fatalf("listing: %v", err)
	}
	var sawCancelled bool
	for _, tr := range mine {
		sawCancelled = sawCancelled || (tr.ID == cancelled.ID && tr.Status == domain.TripCancelled)
	}
	if !sawCancelled {
		t.Error("a cancelled trip should stay visible to its driver")
	}
}

// TestRescheduleCancelledTripIgnoresOldBooking: a cancelled trip holds no booking, so
// moving it onto an occupied slot is not a clash (un-cancelling re-checks).
func TestRescheduleCancelledTripIgnoresOldBooking(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	dispatcher := e.newDispatcher("dispatcher@example.lv", testPassword)
	actor := e.identityFor(dispatcher, nil)
	bus, driver := e.newBus("AA-1111"), e.newDriver("Driver A")
	busy := e.newTrip("Riga", "Liepaja", 8, 12)
	moving := e.newTrip("Riga", "Cesis", 14, 16)
	for _, tr := range []domain.Trip{busy, moving} {
		if _, err := e.assignments.Assign(ctx, actor, tr.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
			t.Fatalf("assigning: %v", err)
		}
	}
	if _, err := e.trips.SetStatus(ctx, actor, moving.ID, domain.TripCancelled, service.Meta{}); err != nil {
		t.Fatalf("cancelling: %v", err)
	}
	moving.ScheduledStart, moving.ScheduledEnd = at(9), at(11)
	moving.Status = domain.TripCancelled
	if _, err := e.trips.Update(ctx, actor, moving, service.Meta{}); err != nil {
		t.Fatalf("rescheduling a cancelled trip onto a busy slot: %v", err)
	}
	if _, err := e.trips.SetStatus(ctx, actor, moving.ID, domain.TripPlanned, service.Meta{}); !errors.Is(err, domain.ErrTimeConflict) {
		t.Errorf("un-cancelling into the clash must be refused, got: %v", err)
	}
}

// auditText returns every before/after snapshot for one entity as text.
func (e *env) auditText(entity string, id int64) string {
	e.t.Helper()
	var out string
	err := e.pool.QueryRow(context.Background(),
		`SELECT coalesce(string_agg(coalesce(before::text, '') || coalesce(after::text, ''), ' '), '')
		 FROM audit_log WHERE entity = $1 AND entity_id = $2`, entity, id).Scan(&out)
	if err != nil {
		e.t.Fatalf("reading audit snapshots: %v", err)
	}
	return out
}

// TestErasureRedactsAuditTrail: GDPR erasure reaches the audit log and the login.
func TestErasureRedactsAuditTrail(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)

	phone, licence := "+371 2000 0000", "LV-998877"
	driver, err := e.fleet.CreateDriver(ctx, actor, domain.Driver{
		FullName: "Jānis Bērziņš", Phone: &phone, LicenseNumber: &licence, IsActive: true,
	}, service.Meta{})
	if err != nil {
		t.Fatalf("creating driver: %v", err)
	}
	user, err := e.users.Create(ctx, actor, "janis@example.lv", testPassword, domain.RoleDriver, &driver.ID, service.Meta{})
	if err != nil {
		t.Fatalf("creating login: %v", err)
	}
	driver.FullName = "Jānis Bērziņš-Ozols"
	if _, err := e.fleet.UpdateDriver(ctx, actor, driver, service.Meta{}); err != nil {
		t.Fatalf("updating driver: %v", err)
	}

	if _, err := e.fleet.AnonymizeDriver(ctx, actor, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("anonymizing: %v", err)
	}

	trail := e.auditText("driver", driver.ID) + e.auditText("user", user.ID)
	for _, pii := range []string{"Bērziņš", phone, licence, "janis@example.lv"} {
		if strings.Contains(trail, pii) {
			t.Errorf("audit log still contains %q after erasure", pii)
		}
	}
	if got := e.auditActions("driver", driver.ID); len(got) != 3 {
		t.Errorf("erasure must keep the audit rows themselves, got actions %v", got)
	}

	login, err := e.repo.GetUserByID(ctx, user.ID)
	if err != nil {
		t.Fatalf("reading login: %v", err)
	}
	if strings.Contains(login.Email, "janis") || login.IsActive {
		t.Errorf("the login must be anonymised and inactive, got %q active=%v", login.Email, login.IsActive)
	}
	if _, err := e.users.SetActive(ctx, actor, user.ID, true, service.Meta{}); messageKey(err) != "error.driver_login_inactive_driver" {
		t.Errorf("re-enabling an erased driver's login must be refused, got: %v", err)
	}

	// Clients too.
	email := "client@example.lv"
	client, err := e.clients.Create(ctx, actor, domain.Client{Name: "SIA Ceļotājs", Email: &email, IsActive: true}, service.Meta{})
	if err != nil {
		t.Fatalf("creating client: %v", err)
	}
	if _, err := e.clients.Anonymize(ctx, actor, client.ID, service.Meta{}); err != nil {
		t.Fatalf("anonymizing client: %v", err)
	}
	if trail := e.auditText("client", client.ID); strings.Contains(trail, "Ceļotājs") || strings.Contains(trail, email) {
		t.Errorf("client audit trail still holds personal data: %s", trail)
	}
}

// TestDeactivatingDriverDeactivatesLogin: an inactive driver cannot keep signing in.
func TestDeactivatingDriverDeactivatesLogin(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	driver, login := e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)

	driver.IsActive = false
	if _, err := e.fleet.UpdateDriver(ctx, actor, driver, service.Meta{}); err != nil {
		t.Fatalf("deactivating driver: %v", err)
	}
	user, err := e.repo.GetUserByID(ctx, login.ID)
	if err != nil {
		t.Fatalf("reading login: %v", err)
	}
	if user.IsActive {
		t.Fatal("the driver's login must be deactivated with the driver")
	}
	if _, err := e.users.SetActive(ctx, actor, login.ID, true, service.Meta{}); messageKey(err) != "error.driver_login_inactive_driver" {
		t.Errorf("re-enabling the login of an inactive driver must be refused, got: %v", err)
	}
}

// TestDispatcherSeesNoPayOrLicenceNumber: data minimization in the service layer.
func TestDispatcherSeesNoPayOrLicenceNumber(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	dispatcher := e.newDispatcher("dispatcher@example.lv", testPassword)
	rate, licence, pay := "12.50", "LV-1", domain.PayHourly
	driver, err := e.fleet.CreateDriver(ctx, e.identityFor(admin, nil), domain.Driver{
		FullName: "Driver A", HourlyRate: &rate, PayType: &pay, LicenseNumber: &licence, IsActive: true,
	}, service.Meta{})
	if err != nil {
		t.Fatalf("creating driver: %v", err)
	}

	disp := e.identityFor(dispatcher, nil)
	list, err := e.fleet.ListDrivers(ctx, disp)
	if err != nil || len(list) != 1 {
		t.Fatalf("listing drivers: %v (%d)", err, len(list))
	}
	got, err := e.fleet.GetDriver(ctx, disp, driver.ID)
	if err != nil {
		t.Fatalf("reading driver: %v", err)
	}
	for _, d := range []domain.Driver{list[0], got} {
		if d.HourlyRate != nil || d.PayType != nil || d.LicenseNumber != nil {
			t.Errorf("dispatcher must not see pay or licence number: %+v", d)
		}
	}

	full, err := e.fleet.GetDriver(ctx, e.identityFor(admin, nil), driver.ID)
	if err != nil || full.HourlyRate == nil || full.LicenseNumber == nil {
		t.Errorf("admin must still see everything: %+v %v", full, err)
	}

	// Through the API as well.
	base := e.serve(t)
	c := &apiClient{newClient(t, base)}
	c.apiLogin("dispatcher@example.lv")
	body := readBody(t, c.get("/api/drivers"))
	if strings.Contains(body, "12.50") || strings.Contains(body, "LV-1") {
		t.Errorf("/api/drivers leaks pay or licence number to a dispatcher: %s", body)
	}
}

// TestAuditRetentionPurgesOnlyOldEntries: two-year retention, recent history untouched.
func TestAuditRetentionPurgesOnlyOldEntries(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	_, err := e.pool.Exec(ctx, `
		INSERT INTO audit_log (action, entity, created_at) VALUES
		  ('login', 'user', now() - interval '2 years 1 day'),
		  ('login', 'user', now() - interval '1 year 11 months'),
		  ('login', 'user', now())`)
	if err != nil {
		t.Fatalf("seeding audit rows: %v", err)
	}
	n, err := e.repo.PurgeOldAuditLog(ctx)
	if err != nil {
		t.Fatalf("purging: %v", err)
	}
	if n != 1 {
		t.Errorf("purged %d rows, want 1", n)
	}
	var left int
	if err := e.pool.QueryRow(ctx, `SELECT count(*) FROM audit_log`).Scan(&left); err != nil {
		t.Fatalf("counting: %v", err)
	}
	if left != 2 {
		t.Errorf("%d rows left, want 2", left)
	}
}

// TestDuplicateValuesAreNamed: a duplicate plate says so, not "conflicts with state".
func TestDuplicateValuesAreNamed(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	e.newBus("AA-1111")
	base := e.serve(t)
	c := &apiClient{newClient(t, base)}
	csrf := c.apiLogin("admin@example.lv")
	resp := c.postJSON("/api/buses", csrf, map[string]any{
		"plate": "AA-1111", "model": "Setra", "seats": 50, "status": "active", "type": "tourist",
	})
	body := readBody(t, resp)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(body, "plate") {
		t.Errorf("duplicate plate: status %d body %s", resp.StatusCode, body)
	}
}

// TestLoginThrottledPerAccount: failed attempts on one account are capped no matter
// how many IPs they come from (the suite's per-IP limiter is effectively off).
func TestLoginThrottledPerAccount(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	base := e.serve(t)
	c := &apiClient{newClient(t, base)}
	csrf := c.me().CSRFToken

	for i := 0; i < 10; i++ {
		resp := c.postJSON("/api/auth/login", csrf, map[string]string{"email": "admin@example.lv", "password": "wrong-password-123"})
		resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("attempt %d: status %d, want 401", i+1, resp.StatusCode)
		}
	}
	resp := c.postJSON("/api/auth/login", csrf, map[string]string{"email": "admin@example.lv", "password": testPassword})
	resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("after 10 failures even the right password must wait: status %d, want 429", resp.StatusCode)
	}

	// Another account is unaffected.
	e.newDispatcher("dispatcher@example.lv", testPassword)
	other := &apiClient{newClient(t, base)}
	other.apiLogin("dispatcher@example.lv")
}
