package inttest

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

type apiClientDTO struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	Phone      string `json:"phone"`
	IsActive   bool   `json:"isActive"`
	Anonymized bool   `json:"anonymized"`
}

type apiClientTripDTO struct {
	ID            int64  `json:"id"`
	ClientID      *int64 `json:"clientId"`
	InvoiceNumber string `json:"invoiceNumber"`
}

func (e *env) newClientRecord(actor domain.Identity, name string) domain.Client {
	e.t.Helper()
	phone := "+371 20000000"
	c, err := e.clients.Create(context.Background(), actor, domain.Client{Name: name, Phone: &phone, IsActive: true}, service.Meta{})
	if err != nil {
		e.t.Fatalf("creating client: %v", err)
	}
	return c
}

// TestClientRoleBoundaries: planners manage clients, only an admin may erase one, and
// a driver reaches nothing.
func TestClientRoleBoundaries(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	e.newDispatcher("dispatcher@example.lv", testPassword)
	e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)

	base := e.serve(t)
	admin := &apiClient{newClient(t, base)}
	adminCSRF := admin.apiLogin("admin@example.lv")
	dispatcher := &apiClient{newClient(t, base)}
	dispatcherCSRF := dispatcher.apiLogin("dispatcher@example.lv")
	driver := &apiClient{newClient(t, base)}
	driverCSRF := driver.apiLogin("driver@example.lv")

	resp := dispatcher.postJSON("/api/clients", dispatcherCSRF, map[string]any{
		"name": "Acme Tours", "phone": "+371 20000000", "email": "Info@Acme.lv",
	})
	created := decodeJSON[apiClientDTO](t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("dispatcher create: status %d, want 201", resp.StatusCode)
	}
	idStr := strconv.FormatInt(created.ID, 10)

	resp = dispatcher.putJSON("/api/clients/"+idStr, dispatcherCSRF, map[string]any{
		"name": "Acme Tours Ltd", "isActive": false,
	})
	updated := decodeJSON[apiClientDTO](t, resp)
	if resp.StatusCode != http.StatusOK || updated.Name != "Acme Tours Ltd" || updated.IsActive {
		t.Fatalf("dispatcher update: status %d, got %+v", resp.StatusCode, updated)
	}

	resp = dispatcher.postJSON("/api/clients/"+idStr+"/anonymize", dispatcherCSRF, map[string]any{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("dispatcher anonymize: status %d, want 403", resp.StatusCode)
	}

	for name, call := range map[string]func() *http.Response{
		"list":   func() *http.Response { return driver.get("/api/clients") },
		"get":    func() *http.Response { return driver.get("/api/clients/" + idStr) },
		"create": func() *http.Response { return driver.postJSON("/api/clients", driverCSRF, map[string]any{"name": "x"}) },
		"update": func() *http.Response {
			return driver.putJSON("/api/clients/"+idStr, driverCSRF, map[string]any{"name": "x"})
		},
		"anonymize": func() *http.Response {
			return driver.postJSON("/api/clients/"+idStr+"/anonymize", driverCSRF, map[string]any{})
		},
	} {
		r := call()
		r.Body.Close()
		if r.StatusCode != http.StatusForbidden {
			t.Errorf("driver %s: status %d, want 403", name, r.StatusCode)
		}
	}

	resp = admin.postJSON("/api/clients/"+idStr+"/anonymize", adminCSRF, map[string]any{})
	erased := decodeJSON[apiClientDTO](t, resp)
	if resp.StatusCode != http.StatusOK || !erased.Anonymized || erased.Name == "Acme Tours Ltd" || erased.Phone != "" {
		t.Fatalf("admin anonymize: status %d, got %+v", resp.StatusCode, erased)
	}

	// An erased client is frozen, and cannot be erased twice.
	resp = admin.putJSON("/api/clients/"+idStr, adminCSRF, map[string]any{"name": "Back again", "isActive": true})
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("update after erasure: status %d, want 409", resp.StatusCode)
	}
	resp = admin.postJSON("/api/clients/"+idStr+"/anonymize", adminCSRF, map[string]any{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("second anonymize: status %d, want 409", resp.StatusCode)
	}

	want := []string{"create", "update", "anonymize"}
	if got := e.auditActions("client", created.ID); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("audit actions = %v, want %v", got, want)
	}
}

// TestTripClientAndInvoiceNumber: the client and bill number round-trip through the
// API, validation holds, and an inactive client cannot be attached to new work.
func TestTripClientAndInvoiceNumber(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	client := e.newClientRecord(actor, "Acme Tours")
	retired := e.newClientRecord(actor, "Retired Ltd")
	retired.IsActive = false
	if _, err := e.clients.Update(ctx, actor, retired, service.Meta{}); err != nil {
		t.Fatalf("deactivating client: %v", err)
	}

	base := e.serve(t)
	api := &apiClient{newClient(t, base)}
	csrf := api.apiLogin("admin@example.lv")

	payload := func(extra map[string]any) map[string]any {
		body := map[string]any{
			"origin": "Riga", "destination": "Liepaja",
			"scheduledStart": at(8).Format("2006-01-02T15:04"),
			"scheduledEnd":   at(12).Format("2006-01-02T15:04"),
		}
		for k, v := range extra {
			body[k] = v
		}
		return body
	}

	resp := api.postJSON("/api/trips", csrf, payload(map[string]any{"clientId": client.ID, "invoiceNumber": "  INV-2026-001 "}))
	trip := decodeJSON[apiClientTripDTO](t, resp)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create trip: status %d", resp.StatusCode)
	}
	if trip.ClientID == nil || *trip.ClientID != client.ID || trip.InvoiceNumber != "INV-2026-001" {
		t.Fatalf("created trip = %+v, want client %d and trimmed bill number", trip, client.ID)
	}

	got := decodeJSON[apiClientTripDTO](t, api.get("/api/trips/"+strconv.FormatInt(trip.ID, 10)))
	if got.ClientID == nil || *got.ClientID != client.ID || got.InvoiceNumber != "INV-2026-001" {
		t.Errorf("GET trip = %+v", got)
	}

	for name, extra := range map[string]map[string]any{
		"unknown client":       {"clientId": 99999},
		"inactive client":      {"clientId": retired.ID},
		"overlong bill number": {"invoiceNumber": strings.Repeat("9", 51)},
	} {
		r := api.postJSON("/api/trips", csrf, payload(extra))
		r.Body.Close()
		if r.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, r.StatusCode)
		}
	}

	// A trip that already has a client keeps it after the client is retired, and
	// can be edited without being forced to drop it.
	deactivated := client
	deactivated.IsActive = false
	if _, err := e.clients.Update(ctx, actor, deactivated, service.Meta{}); err != nil {
		t.Fatalf("deactivating client: %v", err)
	}
	r := api.putJSON("/api/trips/"+strconv.FormatInt(trip.ID, 10), csrf,
		payload(map[string]any{"clientId": client.ID, "invoiceNumber": "INV-2026-002", "notes": "edited"}))
	kept := decodeJSON[apiClientTripDTO](t, r)
	if r.StatusCode != http.StatusOK || kept.ClientID == nil || *kept.ClientID != client.ID || kept.InvoiceNumber != "INV-2026-002" {
		t.Fatalf("edit with retired-but-kept client: status %d, %+v", r.StatusCode, kept)
	}

	// Clearing both is allowed.
	r = api.putJSON("/api/trips/"+strconv.FormatInt(trip.ID, 10), csrf, payload(map[string]any{"clientId": nil, "invoiceNumber": nil}))
	cleared := decodeJSON[apiClientTripDTO](t, r)
	if r.StatusCode != http.StatusOK || cleared.ClientID != nil || cleared.InvoiceNumber != "" {
		t.Fatalf("clearing: status %d, %+v", r.StatusCode, cleared)
	}
}

// TestDriverNeverSeesClientData: nothing client-related reaches a driver's own trips.
func TestDriverNeverSeesClientData(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	driver, _ := e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)
	bus := e.newBus("AA-1111")
	client := e.newClientRecord(actor, "Secret Client Ltd")

	invoice := "INV-SECRET-77"
	// The driver's list only shows recent and upcoming trips, so anchor to now.
	soon := time.Now().UTC().Add(48 * time.Hour).Truncate(time.Hour)
	trip, err := e.trips.Create(ctx, actor, domain.Trip{
		Origin: "Riga", Destination: "Liepaja", ScheduledStart: soon, ScheduledEnd: soon.Add(4 * time.Hour),
		PaymentStatus: domain.PaymentUnpaid, ClientID: &client.ID, InvoiceNumber: &invoice,
	}, service.Meta{})
	if err != nil {
		t.Fatalf("creating trip: %v", err)
	}
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, bus.ID, driver.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	base := e.serve(t)
	api := &apiClient{newClient(t, base)}
	api.apiLogin("driver@example.lv")

	resp := api.get("/api/my/trips")
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	text := string(body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(text, "Liepaja") {
		t.Fatalf("driver trips: status %d, body %s", resp.StatusCode, text)
	}
	for _, leak := range []string{"Secret Client", "INV-SECRET", "clientId", "invoiceNumber", "paymentStatus"} {
		if strings.Contains(text, leak) {
			t.Errorf("driver response leaks %q: %s", leak, text)
		}
	}
}

// TestSeriesClientAndInvoiceNumbers: the client is series-wide, the bill number is not.
func TestSeriesClientAndInvoiceNumbers(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	client := e.newClientRecord(actor, "Acme Tours")
	other := e.newClientRecord(actor, "Other Ltd")

	// Weekly on baseDay's weekday, about 4 occurrences.
	start := at(8)
	trips, err := e.tripSeries.Create(ctx, actor, domain.TripSeries{
		Origin: "Riga", Destination: "Liepaja",
		DaysOfWeek: []int32{int32(isoDay(start))},
		FirstStart: start, FirstEnd: start.Add(2 * time.Hour),
		EndsOn:        start.AddDate(0, 0, 22),
		PaymentStatus: domain.PaymentUnpaid, ClientID: &client.ID,
	}, time.UTC, service.Meta{})
	if err != nil {
		t.Fatalf("creating series: %v", err)
	}
	if len(trips) < 3 {
		t.Fatalf("expected several trips, got %d", len(trips))
	}
	for _, tr := range trips {
		if tr.ClientID == nil || *tr.ClientID != client.ID {
			t.Fatalf("trip %d client = %v, want %d", tr.ID, tr.ClientID, client.ID)
		}
		if tr.InvoiceNumber != nil {
			t.Fatalf("trip %d copied a bill number: %v", tr.ID, *tr.InvoiceNumber)
		}
	}

	// Give the second trip its own bill number, then edit "this and future" from the
	// first trip with a new bill number and a different client.
	second := trips[1]
	own := "OWN-2"
	second.InvoiceNumber = &own
	if _, err := e.trips.Update(ctx, actor, second, service.Meta{}); err != nil {
		t.Fatalf("updating second trip: %v", err)
	}

	first := trips[0]
	firstInvoice := "FIRST-1"
	first.InvoiceNumber = &firstInvoice
	first.ClientID = &other.ID
	if _, err := e.tripSeries.UpdateFuture(ctx, actor, first, time.UTC, service.Meta{}); err != nil {
		t.Fatalf("UpdateFuture: %v", err)
	}

	for i, tr := range trips {
		got, err := e.trips.Get(ctx, actor, tr.ID)
		if err != nil {
			t.Fatalf("reading trip %d: %v", tr.ID, err)
		}
		if got.ClientID == nil || *got.ClientID != other.ID {
			t.Errorf("trip %d client = %v, want %d", i, got.ClientID, other.ID)
		}
		switch i {
		case 0:
			if got.InvoiceNumber == nil || *got.InvoiceNumber != firstInvoice {
				t.Errorf("edited trip bill number = %v, want %s", got.InvoiceNumber, firstInvoice)
			}
		case 1:
			if got.InvoiceNumber == nil || *got.InvoiceNumber != own {
				t.Errorf("sibling trip lost its own bill number: %v", got.InvoiceNumber)
			}
		default:
			if got.InvoiceNumber != nil {
				t.Errorf("trip %d picked up a bill number: %v", i, *got.InvoiceNumber)
			}
		}
	}

	// The unknown-client rule applies to a series as well.
	missing := int64(99999)
	_, err = e.tripSeries.Create(ctx, actor, domain.TripSeries{
		Origin: "Riga", Destination: "Liepaja", DaysOfWeek: []int32{int32(isoDay(start))},
		FirstStart: start, FirstEnd: start.Add(2 * time.Hour), EndsOn: start.AddDate(0, 0, 22),
		PaymentStatus: domain.PaymentUnpaid, ClientID: &missing,
	}, time.UTC, service.Meta{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("series with unknown client: err = %v, want validation error", err)
	}
}

// TestLegacyTripUpdateKeepsClient: the frozen server-rendered edit form has no client
// field, so saving through it must not clear the client or bill number.
func TestLegacyTripUpdateKeepsClient(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	client := e.newClientRecord(actor, "Acme Tours")
	invoice := "INV-1"
	trip, err := e.trips.Create(ctx, actor, domain.Trip{
		Origin: "Riga", Destination: "Liepaja", ScheduledStart: at(8), ScheduledEnd: at(12),
		PaymentStatus: domain.PaymentUnpaid, ClientID: &client.ID, InvoiceNumber: &invoice,
	}, service.Meta{})
	if err != nil {
		t.Fatalf("creating trip: %v", err)
	}

	base := e.serve(t)
	c := newClient(t, base)
	c.login("admin@example.lv")
	tripPath := "/trips/" + strconv.FormatInt(trip.ID, 10)
	form := url.Values{
		"origin": {"Riga"}, "destination": {"Ventspils"},
		"scheduled_start": {at(8).Format("2006-01-02T15:04")},
		"scheduled_end":   {at(12).Format("2006-01-02T15:04")},
	}
	resp := c.post(tripPath+"/edit", tripPath, form)
	resp.Body.Close()

	after, err := e.trips.Get(ctx, actor, trip.ID)
	if err != nil {
		t.Fatalf("reading trip: %v", err)
	}
	if after.Destination != "Ventspils" {
		t.Fatalf("legacy update did not apply (status %d, destination %q)", resp.StatusCode, after.Destination)
	}
	if after.ClientID == nil || *after.ClientID != client.ID || after.InvoiceNumber == nil || *after.InvoiceNumber != invoice {
		t.Errorf("legacy update dropped client/bill number: %+v", after)
	}
}

// TestTripClientChangesAreAudited: attaching a client and bill number to a trip shows up
// in the audit log's before/after, so "who billed this to whom" is traceable.
func TestTripClientChangesAreAudited(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	client := e.newClientRecord(actor, "Acme Tours")

	trip, err := e.trips.Create(ctx, actor, domain.Trip{
		Origin: "Riga", Destination: "Liepaja", ScheduledStart: at(8), ScheduledEnd: at(12),
		PaymentStatus: domain.PaymentUnpaid,
	}, service.Meta{})
	if err != nil {
		t.Fatalf("creating trip: %v", err)
	}
	invoice := "INV-AUDIT-1"
	trip.ClientID, trip.InvoiceNumber = &client.ID, &invoice
	if _, err := e.trips.Update(ctx, actor, trip, service.Meta{}); err != nil {
		t.Fatalf("updating trip: %v", err)
	}

	var before, after map[string]any
	err = e.pool.QueryRow(ctx,
		`SELECT before, after FROM audit_log WHERE entity = 'trip' AND entity_id = $1 AND action = 'update' ORDER BY id DESC LIMIT 1`,
		trip.ID).Scan(&before, &after)
	if err != nil {
		t.Fatalf("reading trip update audit row: %v", err)
	}
	if _, ok := before["client_id"]; ok {
		t.Errorf("before snapshot has client_id: %v", before)
	}
	if _, ok := before["invoice_number"]; ok {
		t.Errorf("before snapshot has invoice_number: %v", before)
	}
	// JSON numbers decode as float64.
	if got, _ := after["client_id"].(float64); int64(got) != client.ID {
		t.Errorf("after client_id = %v, want %d", after["client_id"], client.ID)
	}
	if after["invoice_number"] != invoice {
		t.Errorf("after invoice_number = %v, want %s", after["invoice_number"], invoice)
	}
}

// TestErasedClientStaysLinkedToTrips: GDPR erasure anonymizes in place, so trips keep
// pointing at the client row and can still be edited without dropping it.
func TestErasedClientStaysLinkedToTrips(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	client := e.newClientRecord(actor, "Erase Me Ltd")
	invoice := "INV-ERASE-1"
	trip, err := e.trips.Create(ctx, actor, domain.Trip{
		Origin: "Riga", Destination: "Liepaja", ScheduledStart: at(8), ScheduledEnd: at(12),
		PaymentStatus: domain.PaymentUnpaid, ClientID: &client.ID, InvoiceNumber: &invoice,
	}, service.Meta{})
	if err != nil {
		t.Fatalf("creating trip: %v", err)
	}

	erased, err := e.clients.Anonymize(ctx, actor, client.ID, service.Meta{})
	if err != nil {
		t.Fatalf("anonymizing: %v", err)
	}
	if erased.Name == "Erase Me Ltd" || erased.Phone != nil || erased.IsActive {
		t.Fatalf("client not erased: %+v", erased)
	}

	after, err := e.trips.Get(ctx, actor, trip.ID)
	if err != nil {
		t.Fatalf("reading trip: %v", err)
	}
	if after.ClientID == nil || *after.ClientID != client.ID || after.InvoiceNumber == nil || *after.InvoiceNumber != invoice {
		t.Fatalf("erasure broke the trip link: %+v", after)
	}

	edited := "edited after erasure"
	after.Notes = &edited
	if _, err := e.trips.Update(ctx, actor, after, service.Meta{}); err != nil {
		t.Errorf("editing a trip that keeps its erased client: %v", err)
	}

	// But an erased client can't be attached to other work.
	_, err = e.trips.Create(ctx, actor, domain.Trip{
		Origin: "Riga", Destination: "Cesis", ScheduledStart: at(14), ScheduledEnd: at(16),
		PaymentStatus: domain.PaymentUnpaid, ClientID: &client.ID,
	}, service.Meta{})
	if !errors.Is(err, domain.ErrValidation) {
		t.Errorf("new trip with erased client: err = %v, want validation error", err)
	}
}

// TestClientValidationAtTheAPI: the server rejects bad client input on its own, not only
// the form.
func TestClientValidationAtTheAPI(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	base := e.serve(t)
	api := &apiClient{newClient(t, base)}
	csrf := api.apiLogin("admin@example.lv")

	for name, body := range map[string]map[string]any{
		"missing name":     {"name": "   "},
		"bad email":        {"name": "Acme", "email": "not-an-email"},
		"bad phone":        {"name": "Acme", "phone": "call me"},
		"overlong name":    {"name": strings.Repeat("a", 201)},
		"overlong notes":   {"name": "Acme", "notes": strings.Repeat("n", 2001)},
		"unknown field":    {"name": "Acme", "vatNumber": "LV123"},
		"overlong address": {"name": "Acme", "address": strings.Repeat("x", 501)},
	} {
		r := api.postJSON("/api/clients", csrf, body)
		r.Body.Close()
		if r.StatusCode != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", name, r.StatusCode)
		}
	}
	var audited int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM audit_log WHERE entity = 'client'`).Scan(&audited); err != nil {
		t.Fatalf("counting audit rows: %v", err)
	}
	if audited != 0 {
		t.Errorf("rejected input produced %d client audit rows", audited)
	}
}

func isoDay(t time.Time) int {
	if t.Weekday() == time.Sunday {
		return 7
	}
	return int(t.Weekday())
}
