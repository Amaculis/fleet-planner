package inttest

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"
)

// apiLogin signs c in and returns the session-bound CSRF token every subsequent
// mutating call must carry.
func (c *apiClient) apiLogin(email string) string {
	c.t.Helper()
	anon := c.me()
	resp := c.postJSON("/api/auth/login", anon.CSRFToken, map[string]string{
		"email": email, "password": testPassword,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.t.Fatalf("login as %s: status %d, want 200", email, resp.StatusCode)
	}
	var out meResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		c.t.Fatalf("decoding login response: %v", err)
	}
	return out.CSRFToken
}

func decodeJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var out T
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatalf("decoding response: %v", err)
	}
	return out
}

type apiBusDTO struct {
	ID     int64  `json:"id"`
	Plate  string `json:"plate"`
	Status string `json:"status"`
}

// TestAPIBusCRUD drives the full bus lifecycle through the JSON API: create, read,
// update, delete, and confirms a dispatcher can list but not mutate.
func TestAPIBusCRUD(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	e.newDispatcher("dispatcher@example.lv", testPassword)

	base := e.serve(t)
	admin := &apiClient{newClient(t, base)}
	csrf := admin.apiLogin("admin@example.lv")

	createResp := admin.postJSON("/api/buses", csrf, map[string]any{
		"plate": "CC-3333", "model": "Setra S515", "seats": 50, "status": "active",
	})
	created := decodeJSON[apiBusDTO](t, createResp)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create: status %d", createResp.StatusCode)
	}
	if created.Plate != "CC-3333" {
		t.Errorf("plate = %q", created.Plate)
	}

	idStr := strconv.FormatInt(created.ID, 10)
	getResp := admin.get("/api/buses/" + idStr)
	got := decodeJSON[apiBusDTO](t, getResp)
	if got.ID != created.ID {
		t.Fatalf("GET returned id %d, want %d", got.ID, created.ID)
	}

	updateResp := admin.putJSON("/api/buses/"+idStr, csrf, map[string]any{
		"plate": "CC-3333", "model": "Setra S515", "seats": 55, "status": "maintenance",
	})
	updated := decodeJSON[apiBusDTO](t, updateResp)
	if updated.Status != "maintenance" {
		t.Errorf("status after update = %q, want maintenance", updated.Status)
	}

	deleteResp := admin.deleteJSON("/api/buses/"+idStr, csrf)
	deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete: status %d, want 204", deleteResp.StatusCode)
	}

	dispatcher := &apiClient{newClient(t, base)}
	dCsrf := dispatcher.apiLogin("dispatcher@example.lv")

	listResp := dispatcher.get("/api/buses")
	defer listResp.Body.Close()
	if listResp.StatusCode != http.StatusOK {
		t.Errorf("dispatcher list buses: status %d, want 200", listResp.StatusCode)
	}

	forbiddenResp := dispatcher.postJSON("/api/buses", dCsrf, map[string]any{
		"plate": "DD-4444", "model": "x", "seats": 10, "status": "active",
	})
	forbiddenResp.Body.Close()
	if forbiddenResp.StatusCode != http.StatusForbidden {
		t.Errorf("dispatcher create bus: status %d, want 403", forbiddenResp.StatusCode)
	}
}

type apiDriverDTO struct {
	ID         int64  `json:"id"`
	FullName   string `json:"fullName"`
	IsActive   bool   `json:"isActive"`
	Anonymized bool   `json:"anonymized"`
}

// TestAPIDriverAnonymize: the GDPR erasure action works through the JSON API and its
// result is reflected immediately in the response.
func TestAPIDriverAnonymize(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	driver := e.newDriver("Driver A")

	base := e.serve(t)
	admin := &apiClient{newClient(t, base)}
	csrf := admin.apiLogin("admin@example.lv")

	idStr := strconv.FormatInt(driver.ID, 10)
	resp := admin.postJSON("/api/drivers/"+idStr+"/anonymize", csrf, map[string]any{})
	out := decodeJSON[apiDriverDTO](t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("anonymize: status %d", resp.StatusCode)
	}
	if !out.Anonymized {
		t.Error("driver not reported as anonymized")
	}
	if out.FullName == "Driver A" {
		t.Error("the driver's name is still present after anonymization")
	}
}

type apiTripDTO struct {
	ID         int64 `json:"id"`
	Status     string `json:"status"`
	Assignment *struct {
		BusPlate   string `json:"busPlate"`
		DriverName string `json:"driverName"`
	} `json:"assignment"`
}

// TestAPITripAssignFlow: create a trip, book a bus+driver, see the assignment on the
// detail response, then unassign and confirm it's gone. A double-booking attempt
// through the same endpoint gets the conflict shape, not a generic error.
func TestAPITripAssignFlow(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	bus := e.newBus("EE-5555")
	driver := e.newDriver("Driver B")

	base := e.serve(t)
	admin := &apiClient{newClient(t, base)}
	csrf := admin.apiLogin("admin@example.lv")

	createResp := admin.postJSON("/api/trips", csrf, map[string]any{
		"origin": "Riga", "destination": "Liepaja",
		"scheduledStart": at(8).Format("2006-01-02T15:04"),
		"scheduledEnd":   at(12).Format("2006-01-02T15:04"),
	})
	trip := decodeJSON[apiTripDTO](t, createResp)
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create trip: status %d", createResp.StatusCode)
	}
	idStr := strconv.FormatInt(trip.ID, 10)

	assignResp := admin.postJSON("/api/trips/"+idStr+"/assign", csrf, map[string]any{
		"busId": bus.ID, "driverId": driver.ID,
	})
	assigned := decodeJSON[apiTripDTO](t, assignResp)
	if assignResp.StatusCode != http.StatusOK {
		t.Fatalf("assign: status %d", assignResp.StatusCode)
	}
	if assigned.Assignment == nil || assigned.Assignment.BusPlate != "EE-5555" || assigned.Assignment.DriverName != "Driver B" {
		t.Fatalf("assignment = %+v, want EE-5555 / Driver B", assigned.Assignment)
	}

	// A second, overlapping trip cannot double-book the same bus.
	clashResp := admin.postJSON("/api/trips", csrf, map[string]any{
		"origin": "Riga", "destination": "Jelgava",
		"scheduledStart": at(10).Format("2006-01-02T15:04"),
		"scheduledEnd":   at(14).Format("2006-01-02T15:04"),
	})
	clashTrip := decodeJSON[apiTripDTO](t, clashResp)
	clashIDStr := strconv.FormatInt(clashTrip.ID, 10)

	clashAssignResp := admin.postJSON("/api/trips/"+clashIDStr+"/assign", csrf, map[string]any{
		"busId": bus.ID, "driverId": driver.ID,
	})
	defer clashAssignResp.Body.Close()
	if clashAssignResp.StatusCode != http.StatusConflict {
		t.Fatalf("clashing assign: status %d, want 409", clashAssignResp.StatusCode)
	}

	unassignResp := admin.postJSON("/api/trips/"+idStr+"/unassign", csrf, map[string]any{})
	unassigned := decodeJSON[apiTripDTO](t, unassignResp)
	if unassigned.Assignment != nil {
		t.Error("assignment still present after unassign")
	}
}

type apiUserListDTO struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	IsActive bool   `json:"isActive"`
}

// TestAPIUserLifecycle: create a dispatcher account, deactivate it, and confirm it
// can no longer sign in — the same server-side enforcement the HTML flow relies on.
func TestAPIUserLifecycle(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	admin := &apiClient{newClient(t, base)}
	csrf := admin.apiLogin("admin@example.lv")

	createResp := admin.postJSON("/api/users", csrf, map[string]any{
		"email": "newdispatcher@example.lv", "password": testPassword, "role": "dispatcher",
	})
	defer createResp.Body.Close()
	if createResp.StatusCode != http.StatusCreated {
		t.Fatalf("create user: status %d", createResp.StatusCode)
	}

	listResp := admin.get("/api/users")
	users := decodeJSON[[]apiUserListDTO](t, listResp)
	var newID int64
	for _, u := range users {
		if u.Email == "newdispatcher@example.lv" {
			newID = u.ID
		}
	}
	if newID == 0 {
		t.Fatal("new user not found in the list")
	}

	deactivateResp := admin.postJSON("/api/users/"+strconv.FormatInt(newID, 10)+"/deactivate", csrf, map[string]any{})
	deactivateResp.Body.Close()
	if deactivateResp.StatusCode != http.StatusNoContent {
		t.Fatalf("deactivate: status %d, want 204", deactivateResp.StatusCode)
	}

	other := &apiClient{newClient(t, base)}
	anon := other.me()
	loginResp := other.postJSON("/api/auth/login", anon.CSRFToken, map[string]string{
		"email": "newdispatcher@example.lv", "password": testPassword,
	})
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("deactivated user logged in: status %d, want 401", loginResp.StatusCode)
	}
}
