package inttest

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

// publicRoutes are the only routes an anonymous caller may reach. Everything else the
// router serves must demand a session; adding a route to this list is a security
// decision and should be reviewed as one.
var publicRoutes = map[string]bool{
	"GET /healthz":         true,
	"GET /sw.js":           true,
	"GET /offline":         true,
	"GET /login":           true,
	"POST /login":          true,
	"GET /api/auth/me":     true,
	"POST /api/auth/login": true,
}

var routeParam = regexp.MustCompile(`\{[^}]+\}`)

type route struct{ method, path string }

// appRoutes lists every concrete route the router serves, with path parameters filled
// in. Wildcard mounts (static files and the SPA bundle) are left out: they serve files,
// not data.
func appRoutes(t *testing.T, e *env) []route {
	t.Helper()
	mux, ok := e.server.Routes().(chi.Routes)
	if !ok {
		t.Fatal("Routes() is not a chi router; this test can no longer enumerate routes")
	}
	var out []route
	err := chi.Walk(mux, func(method, pattern string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		if strings.Contains(pattern, "*") {
			return nil
		}
		out = append(out, route{method, routeParam.ReplaceAllString(pattern, "1")})
		return nil
	})
	if err != nil {
		t.Fatalf("walking routes: %v", err)
	}
	if len(out) < 50 {
		t.Fatalf("only %d routes found; the walk is not seeing the whole router", len(out))
	}
	return out
}

func (c *client) do(method, path, csrfToken string) *http.Response {
	c.t.Helper()
	req, err := http.NewRequest(method, c.base+path, strings.NewReader("{}"))
	if err != nil {
		c.t.Fatalf("building request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if csrfToken != "" {
		req.Header.Set("X-CSRF-Token", csrfToken)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", method, path, err)
	}
	resp.Body.Close()
	return resp
}

// TestEveryRouteRequiresAuth sweeps the whole router, so a newly added route that
// forgets RequireAuth fails here without anyone having to remember to test it.
func TestEveryRouteRequiresAuth(t *testing.T) {
	e := newEnv(t)
	base := e.serve(t)
	anon := &apiClient{newClient(t, base)}
	// A valid anonymous token gets non-GET requests past CSRF, so what is tested is the
	// authentication check itself.
	token := anon.me().CSRFToken

	for _, rt := range appRoutes(t, e) {
		if publicRoutes[rt.method+" "+rt.path] {
			continue
		}
		resp := anon.do(rt.method, rt.path, token)
		switch {
		case resp.StatusCode == http.StatusUnauthorized:
		case resp.StatusCode == http.StatusSeeOther && resp.Header.Get("Location") == "/login":
		default:
			t.Errorf("%s %s as anonymous: status %d (Location %q), want 401 or a redirect to /login",
				rt.method, rt.path, resp.StatusCode, resp.Header.Get("Location"))
		}
	}
}

// TestEveryMutationRequiresCSRF: no state-changing route accepts a request without
// this session's own CSRF token, including a valid token stolen from another session.
func TestEveryMutationRequiresCSRF(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	e.newDispatcher("dispatcher@example.lv", testPassword)
	base := e.serve(t)

	admin := &apiClient{newClient(t, base)}
	admin.apiLogin("admin@example.lv")
	other := &apiClient{newClient(t, base)}
	foreignToken := other.apiLogin("dispatcher@example.lv")

	checked := 0
	for _, rt := range appRoutes(t, e) {
		if rt.method == http.MethodGet || rt.method == http.MethodHead || rt.method == http.MethodOptions {
			continue
		}
		checked++
		for name, token := range map[string]string{"no token": "", "another session's token": foreignToken} {
			if resp := admin.do(rt.method, rt.path, token); resp.StatusCode != http.StatusForbidden {
				t.Errorf("%s %s with %s: status %d, want 403", rt.method, rt.path, name, resp.StatusCode)
			}
		}
	}
	if checked < 30 {
		t.Fatalf("only %d state-changing routes found", checked)
	}

	// The login form path must also refuse a form post that carries no token.
	resp, err := newClient(t, base).http.PostForm(base+"/login", url.Values{
		"email": {"admin@example.lv"}, "password": {testPassword},
	})
	if err != nil {
		t.Fatalf("posting login form: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("form login without token: status %d, want 403", resp.StatusCode)
	}
}

func sessionCookie(t *testing.T, jar http.CookieJar, base string) string {
	t.Helper()
	u, _ := url.Parse(base)
	for _, c := range jar.Cookies(u) {
		if c.Name == "fleet_session" {
			return c.Value
		}
	}
	return ""
}

// TestLoginDefeatsSessionFixation: a session id planted before login is never adopted,
// and every login mints a fresh one.
func TestLoginDefeatsSessionFixation(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	base := e.serve(t)
	u, _ := url.Parse(base)

	const planted = "attacker-chosen-session-id"
	victim := &apiClient{newClient(t, base)}
	victim.http.Jar.SetCookies(u, []*http.Cookie{{Name: "fleet_session", Value: planted, Path: "/"}})
	victim.apiLogin("admin@example.lv")

	first := sessionCookie(t, victim.http.Jar, base)
	if first == "" || first == planted {
		t.Fatalf("session after login = %q, want a fresh server-minted id", first)
	}

	attacker := &apiClient{newClient(t, base)}
	attacker.http.Jar.SetCookies(u, []*http.Cookie{{Name: "fleet_session", Value: planted, Path: "/"}})
	if attacker.me().Authenticated {
		t.Fatal("the planted session id became authenticated")
	}

	victim.apiLogin("admin@example.lv")
	if second := sessionCookie(t, victim.http.Jar, base); second == "" || second == first {
		t.Errorf("logging in again reused session id %q", first)
	}
}

// TestRevokedAccountsLoseAccessImmediately: deactivating a user or resetting their
// password ends the sessions they already hold, not just future logins.
func TestRevokedAccountsLoseAccessImmediately(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	deactivated := e.newDispatcher("gone@example.lv", testPassword)
	reset := e.newDispatcher("reset@example.lv", testPassword)
	base := e.serve(t)

	admin := &apiClient{newClient(t, base)}
	adminCSRF := admin.apiLogin("admin@example.lv")
	goneClient := &apiClient{newClient(t, base)}
	goneClient.apiLogin("gone@example.lv")
	resetClient := &apiClient{newClient(t, base)}
	resetClient.apiLogin("reset@example.lv")

	resp := admin.postJSON("/api/users/"+strconv.FormatInt(deactivated.ID, 10)+"/deactivate", adminCSRF, map[string]any{})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("deactivate: status %d", resp.StatusCode)
	}
	resp = admin.postJSON("/api/users/"+strconv.FormatInt(reset.ID, 10)+"/password", adminCSRF,
		map[string]string{"password": "a-brand-new-long-password"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		t.Fatalf("set password: status %d", resp.StatusCode)
	}

	for name, c := range map[string]*apiClient{"deactivated": goneClient, "password reset": resetClient} {
		if r := c.get("/api/trips"); r.StatusCode != http.StatusUnauthorized {
			t.Errorf("%s user's old session: status %d, want 401", name, r.StatusCode)
		} else {
			r.Body.Close()
		}
	}

	// A deactivated account cannot log back in, and the old password is dead after a reset.
	for email, password := range map[string]string{"gone@example.lv": testPassword, "reset@example.lv": testPassword} {
		c := &apiClient{newClient(t, base)}
		r := c.postJSON("/api/auth/login", c.me().CSRFToken, map[string]string{"email": email, "password": password})
		r.Body.Close()
		if r.StatusCode != http.StatusUnauthorized {
			t.Errorf("login as %s: status %d, want 401", email, r.StatusCode)
		}
	}
}

// TestDriverAPICannotTouchAnotherDriversTrip: ownership is checked by the session's
// driver, not by the trip id in the URL.
func TestDriverAPICannotTouchAnotherDriversTrip(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	driverA, _ := e.newDriverWithLogin("Driver A", "a@example.lv", testPassword)
	e.newDriverWithLogin("Driver B", "b@example.lv", testPassword)
	bus := e.newBus("AA-1111")

	soon := time.Now().UTC().Add(time.Hour).Truncate(time.Minute)
	trip, err := e.trips.Create(ctx, actor, domain.Trip{
		Origin: "Riga", Destination: "Liepaja", ScheduledStart: soon, ScheduledEnd: soon.Add(3 * time.Hour),
		PaymentStatus: domain.PaymentUnpaid,
	}, service.Meta{})
	if err != nil {
		t.Fatalf("creating trip: %v", err)
	}
	if _, err := e.assignments.Assign(ctx, actor, trip.ID, bus.ID, driverA.ID, service.Meta{}); err != nil {
		t.Fatalf("assigning: %v", err)
	}

	base := e.serve(t)
	b := &apiClient{newClient(t, base)}
	csrf := b.apiLogin("b@example.lv")

	idStr := strconv.FormatInt(trip.ID, 10)
	for _, action := range []string{"start", "finish"} {
		resp := b.postJSON("/api/my/trips/"+idStr+"/"+action, csrf, map[string]any{})
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden && resp.StatusCode != http.StatusNotFound {
			t.Errorf("driver B %s on driver A's trip: status %d, want 403 or 404", action, resp.StatusCode)
		}
	}
	got, err := e.trips.Get(ctx, actor, trip.ID)
	if err != nil {
		t.Fatalf("reading trip: %v", err)
	}
	if got.Status != domain.TripPlanned || got.ActualStart != nil {
		t.Errorf("another driver changed the trip: status %s, actual start %v", got.Status, got.ActualStart)
	}

	for _, path := range []string{"/api/trips/" + idStr, "/api/drivers/" + strconv.FormatInt(driverA.ID, 10), "/api/users"} {
		if r := b.get(path); r.StatusCode != http.StatusForbidden {
			t.Errorf("driver GET %s: status %d, want 403", path, r.StatusCode)
		} else {
			r.Body.Close()
		}
	}
}

// TestPrivilegeEscalationIsRejected: a dispatcher cannot mint or take over an admin
// account, and request bodies cannot smuggle in fields the endpoint does not accept.
func TestPrivilegeEscalationIsRejected(t *testing.T) {
	ctx := context.Background()
	e := newEnv(t)
	admin := e.newAdmin("admin@example.lv", testPassword)
	e.newDispatcher("dispatcher@example.lv", testPassword)
	actor := e.identityFor(admin, nil)
	base := e.serve(t)

	d := &apiClient{newClient(t, base)}
	csrf := d.apiLogin("dispatcher@example.lv")
	adminID := strconv.FormatInt(admin.ID, 10)

	for name, call := range map[string]func() *http.Response{
		"create admin": func() *http.Response {
			return d.postJSON("/api/users", csrf, map[string]any{"email": "evil@example.lv", "password": "a-very-long-password", "role": "admin"})
		},
		"reset admin password": func() *http.Response {
			return d.postJSON("/api/users/"+adminID+"/password", csrf, map[string]string{"password": "a-very-long-password"})
		},
		"deactivate admin": func() *http.Response {
			return d.postJSON("/api/users/"+adminID+"/deactivate", csrf, map[string]any{})
		},
	} {
		r := call()
		r.Body.Close()
		if r.StatusCode != http.StatusForbidden {
			t.Errorf("dispatcher %s: status %d, want 403", name, r.StatusCode)
		}
	}
	if users, err := e.repo.ListUsers(ctx); err == nil {
		for _, u := range users {
			if u.Email == "evil@example.lv" {
				t.Error("a dispatcher managed to create a user")
			}
		}
	}

	// Mass assignment: a trip update may not set its own status, and a login may not
	// carry a role.
	trip, err := e.trips.Create(ctx, actor, domain.Trip{
		Origin: "Riga", Destination: "Liepaja", ScheduledStart: at(8), ScheduledEnd: at(12),
		PaymentStatus: domain.PaymentUnpaid,
	}, service.Meta{})
	if err != nil {
		t.Fatalf("creating trip: %v", err)
	}
	r := d.putJSON("/api/trips/"+strconv.FormatInt(trip.ID, 10), csrf, map[string]any{
		"origin": "Riga", "destination": "Liepaja", "paymentStatus": "unpaid",
		"scheduledStart": at(8).Format("2006-01-02T15:04"), "scheduledEnd": at(12).Format("2006-01-02T15:04"),
		"status": "completed",
	})
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("trip update smuggling status: status %d, want 400", r.StatusCode)
	}
	if got, _ := e.trips.Get(ctx, actor, trip.ID); got.Status != domain.TripPlanned {
		t.Errorf("trip status changed to %s via an unknown field", got.Status)
	}

	anon := &apiClient{newClient(t, base)}
	r = anon.postJSON("/api/auth/login", anon.me().CSRFToken, map[string]string{
		"email": "dispatcher@example.lv", "password": testPassword, "role": "admin",
	})
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest {
		t.Errorf("login carrying a role: status %d, want 400", r.StatusCode)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading body: %v", err)
	}
	return string(b)
}

// TestNoCredentialsInResponses: password hashes, session tokens and CSRF secrets never
// appear in API output.
func TestNoCredentialsInResponses(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)
	base := e.serve(t)

	c := &apiClient{newClient(t, base)}
	anon := c.me()
	login := readBody(t, c.postJSON("/api/auth/login", anon.CSRFToken, map[string]string{
		"email": "admin@example.lv", "password": testPassword,
	}))
	token := sessionCookie(t, c.http.Jar, base)
	if token == "" {
		t.Fatal("no session cookie after login")
	}

	bodies := map[string]string{
		"login":     login,
		"me":        readBody(t, c.get("/api/auth/me")),
		"users":     readBody(t, c.get("/api/users")),
		"drivers":   readBody(t, c.get("/api/drivers")),
		"user page": readBody(t, c.get("/users")),
	}
	for name, body := range bodies {
		for _, secret := range []string{"$argon2", "password_hash", "passwordHash", testPassword, token, "csrf_secret", "csrfSecret"} {
			if strings.Contains(body, secret) {
				t.Errorf("%s response contains %q", name, secret)
			}
		}
	}
}

// TestLogsCarryNoSecretsOrPersonalData: CLAUDE.md requires structured logs with no
// PII. Failed and successful logins, and personal-data edits, must not leak into them.
func TestLogsCarryNoSecretsOrPersonalData(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin-logcheck@example.lv", testPassword)
	base := e.serve(t)

	const wrongPassword = "wrong-password-that-must-not-be-logged"
	c := &apiClient{newClient(t, base)}
	r := c.postJSON("/api/auth/login", c.me().CSRFToken, map[string]string{
		"email": "nobody-logcheck@example.lv", "password": wrongPassword,
	})
	r.Body.Close()
	r = c.postJSON("/api/auth/login", c.me().CSRFToken, map[string]string{
		"email": "admin-logcheck@example.lv", "password": wrongPassword,
	})
	r.Body.Close()
	csrf := c.apiLogin("admin-logcheck@example.lv")

	r = c.postJSON("/api/drivers", csrf, map[string]any{
		"fullName": "Logcheck Driverson", "phone": "+371 29998877", "licenseNumber": "LIC-LOGCHECK", "licenseExpiry": "", "isActive": true,
	})
	r.Body.Close()
	r = c.postJSON("/api/clients", csrf, map[string]any{
		"name": "Logcheck Client", "email": "client-logcheck@example.lv", "address": "Logcheck iela 1",
	})
	r.Body.Close()
	// A request that fails validation is the likeliest to echo its input into a log.
	r = c.postJSON("/api/clients", csrf, map[string]any{"name": "Logcheck Client", "email": "bad-logcheck-email"})
	r.Body.Close()

	logs := e.logs.String()
	if logs == "" {
		t.Fatal("captured no logs at all; the capture is not wired up")
	}
	for _, leak := range []string{
		testPassword, wrongPassword, "logcheck@example.lv", "Logcheck Driverson", "29998877",
		"LIC-LOGCHECK", "Logcheck Client", "Logcheck iela", "bad-logcheck-email",
	} {
		if strings.Contains(logs, leak) {
			t.Errorf("logs contain %q", leak)
		}
	}
}

// TestUserContentIsEscaped: text a user typed is output-encoded, so it can never run as
// markup in the server-rendered pages.
func TestUserContentIsEscaped(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	const payload = `<script>alert("xss")</script><img src=x onerror=alert(1)>`
	e.newDriver(payload)
	base := e.serve(t)

	c := newClient(t, base)
	c.login("admin@example.lv")
	page := c.body("/drivers")
	if strings.Contains(page, "<script>alert") || strings.Contains(page, "<img src=x") {
		t.Fatal("driver name rendered as raw HTML on /drivers")
	}
	if !strings.Contains(page, "&lt;script&gt;") {
		t.Error("escaped driver name not found on /drivers; the test is not seeing the row")
	}

	api := &apiClient{newClient(t, base)}
	api.apiLogin("admin@example.lv")
	if body := readBody(t, api.get("/api/drivers")); strings.Contains(body, "<script>") {
		t.Error("JSON response carries a raw <script> tag")
	}
}

// TestInjectionLikeInputIsInert: SQL metacharacters are stored and returned as plain
// text, and a malformed filter is rejected rather than reaching a query.
func TestInjectionLikeInputIsInert(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	base := e.serve(t)
	c := &apiClient{newClient(t, base)}
	csrf := c.apiLogin("admin@example.lv")

	const name = `Robert'); DROP TABLE clients;--`
	created := decodeJSON[apiClientDTO](t, c.postJSON("/api/clients", csrf, map[string]any{"name": name}))
	if created.Name != name {
		t.Fatalf("stored name = %q, want it unchanged", created.Name)
	}
	if got := decodeJSON[apiClientDTO](t, c.get("/api/clients/"+strconv.FormatInt(created.ID, 10))); got.Name != name {
		t.Errorf("read back %q", got.Name)
	}

	// Date filters are parsed into time values before any query runs; junk is ignored
	// in favour of the default range rather than reaching SQL as text.
	junk := readBody(t, c.get("/api/trips?from="+url.QueryEscape("2026-01-01' OR '1'='1")+"&to="+url.QueryEscape("x'--")))
	if def := readBody(t, c.get("/api/trips")); junk != def {
		t.Errorf("junk date filter changed the result:\n got %s\nwant %s", junk, def)
	}
	r := c.get("/api/trips/" + url.PathEscape("1 OR 1=1"))
	r.Body.Close()
	if r.StatusCode != http.StatusBadRequest && r.StatusCode != http.StatusNotFound {
		t.Errorf("non-numeric id: status %d, want 400 or 404", r.StatusCode)
	}
}

// TestOversizedBodyIsRejected: request bodies are capped, so one huge payload cannot
// exhaust memory or be stored.
func TestOversizedBodyIsRejected(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	base := e.serve(t)
	c := &apiClient{newClient(t, base)}
	csrf := c.apiLogin("admin@example.lv")

	r := c.postJSON("/api/clients", csrf, map[string]any{"name": "Big", "notes": strings.Repeat("x", 2<<20)})
	r.Body.Close()
	if r.StatusCode < 400 {
		t.Fatalf("2 MiB body: status %d, want a 4xx", r.StatusCode)
	}
	var n int
	if err := e.pool.QueryRow(context.Background(), `SELECT count(*) FROM clients`).Scan(&n); err != nil {
		t.Fatalf("counting clients: %v", err)
	}
	if n != 0 {
		t.Errorf("oversized request still created %d client(s)", n)
	}
}
