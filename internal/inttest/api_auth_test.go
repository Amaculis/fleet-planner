package inttest

import (
	"bytes"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// apiClient is a browser-like client for the JSON API: it keeps cookies (the session
// is still an httpOnly cookie, same as the server-rendered app) but never follows
// redirects, so a wrongly-redirecting handler fails loudly instead of silently
// returning whatever the redirect target happens to render.
type apiClient struct{ *client }

type meResponse struct {
	Authenticated bool `json:"authenticated"`
	User          *struct {
		ID    int64  `json:"id"`
		Email string `json:"email"`
		Role  string `json:"role"`
	} `json:"user"`
	CSRFToken string `json:"csrfToken"`
}

func (c *apiClient) me() meResponse {
	c.t.Helper()
	resp := c.get("/api/auth/me")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		c.t.Fatalf("GET /api/auth/me: status %d, want 200", resp.StatusCode)
	}
	var out meResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		c.t.Fatalf("decoding /api/auth/me: %v", err)
	}
	return out
}

func (c *apiClient) doJSON(method, path, csrfToken string, body any) *http.Response {
	c.t.Helper()
	buf, err := json.Marshal(body)
	if err != nil {
		c.t.Fatalf("marshaling request body: %v", err)
	}
	req, err := http.NewRequest(method, c.base+path, bytes.NewReader(buf))
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
	return resp
}

func (c *apiClient) postJSON(path, csrfToken string, body any) *http.Response {
	return c.doJSON(http.MethodPost, path, csrfToken, body)
}

func (c *apiClient) putJSON(path, csrfToken string, body any) *http.Response {
	return c.doJSON(http.MethodPut, path, csrfToken, body)
}

func (c *apiClient) deleteJSON(path, csrfToken string) *http.Response {
	return c.doJSON(http.MethodDelete, path, csrfToken, struct{}{})
}

// TestAPIMeBootstraps: the SPA's very first call must work whether or not anyone is
// signed in, and always carries a CSRF token — the anonymous one pre-login, matching
// what the server-rendered login form already embeds via the same LoadSession path.
func TestAPIMeBootstraps(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	c := &apiClient{newClient(t, base)}

	me := c.me()
	if me.Authenticated {
		t.Error("an anonymous caller was reported as authenticated")
	}
	if me.User != nil {
		t.Error("an anonymous caller got a user back")
	}
	if me.CSRFToken == "" {
		t.Error("an anonymous caller got no CSRF token — the login call below cannot be authorized without one")
	}
}

// TestAPILoginRoundTrip: a correct login sets the session cookie, hands back the
// user and a fresh (session-bound) CSRF token, and /api/auth/me then reports the same
// identity on a follow-up call — the exact bootstrap sequence the SPA's auth store
// performs (portal/src/stores/auth.js).
func TestAPILoginRoundTrip(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	c := &apiClient{newClient(t, base)}

	anon := c.me()
	resp := c.postJSON("/api/auth/login", anon.CSRFToken, map[string]string{
		"email": "admin@example.lv", "password": testPassword,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("login: status %d, want 200", resp.StatusCode)
	}
	var loginResp meResponse
	if err := json.NewDecoder(resp.Body).Decode(&loginResp); err != nil {
		t.Fatalf("decoding login response: %v", err)
	}
	if !loginResp.Authenticated || loginResp.User == nil || loginResp.User.Email != "admin@example.lv" {
		t.Fatalf("login response = %+v, want an authenticated admin", loginResp)
	}
	if loginResp.CSRFToken == "" || loginResp.CSRFToken == anon.CSRFToken {
		t.Error("login did not hand back a fresh, session-bound CSRF token")
	}

	// The session cookie is what makes this work — no bearer token anywhere.
	me := c.me()
	if !me.Authenticated || me.User == nil || me.User.Role != "admin" {
		t.Fatalf("/api/auth/me after login = %+v, want the same admin", me)
	}

	logoutResp := c.postJSON("/api/auth/logout", loginResp.CSRFToken, map[string]string{})
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("logout: status %d, want 200", logoutResp.StatusCode)
	}
	after := c.me()
	if after.Authenticated {
		t.Error("still authenticated after logout")
	}
}

// TestAPILoginRejectsWrongPassword: one message for every failure mode, same as the
// HTML login form — this API must not distinguish "unknown email" from "wrong
// password" either.
func TestAPILoginRejectsWrongPassword(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	c := &apiClient{newClient(t, base)}
	anon := c.me()

	resp := c.postJSON("/api/auth/login", anon.CSRFToken, map[string]string{
		"email": "admin@example.lv", "password": "wrong",
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", resp.StatusCode)
	}

	me := c.me()
	if me.Authenticated {
		t.Error("a failed login left the caller authenticated")
	}
}

// TestAPILoginRejectsMissingCSRF: the JSON API is bound by the same CSRF middleware
// as every other state-changing route — a login attempt with no token must not
// succeed just because it arrived as JSON instead of a form post.
func TestAPILoginRejectsMissingCSRF(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	c := &apiClient{newClient(t, base)}

	resp := c.postJSON("/api/auth/login", "", map[string]string{
		"email": "admin@example.lv", "password": testPassword,
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("status %d, want 403", resp.StatusCode)
	}
}

// TestAPILogoutRequiresAuth: an anonymous call gets a clean 401 JSON error, never an
// HTML redirect — an SPA calling this expects JSON back, not a login page to parse.
func TestAPILogoutRequiresAuth(t *testing.T) {
	e := newEnv(t)
	base := e.serve(t)
	c := &apiClient{newClient(t, base)}

	// A real, valid (anonymous) CSRF token — this test is about RequireAuth's
	// response shape, not CSRF, so the request must clear that check first.
	anon := c.me()
	resp := c.postJSON("/api/auth/logout", anon.CSRFToken, map[string]string{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("status %d, want 401", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}
