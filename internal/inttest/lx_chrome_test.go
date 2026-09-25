package inttest

import (
	"io"
	"net/url"
	"strings"
	"testing"
)

// TestChromeIslandsProgressiveEnhancement covers the nav toolbar and the primary form
// save button: the two page-chrome islands added on top of the field islands. Same
// contract, same limitation as every other island test here — it verifies the HTML the
// server sends and that assets are served, not that Vue actually renders anything; see
// docs/lx-ui-integration.md.
func TestChromeIslandsProgressiveEnhancement(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	c := newClient(t, base)
	c.login("admin@example.lv")

	t.Run("nav toolbar", func(t *testing.T) {
		body := c.body("/timeline")

		if !strings.Contains(body, `<nav id="site-nav"`) {
			t.Fatal("the real, working <nav> is missing")
		}
		if !strings.Contains(body, `id="logout-form"`) {
			t.Fatal("the logout form has no id for the toolbar island to submit")
		}
		idx := strings.Index(body, `data-kind="toolbar"`)
		if idx == -1 {
			t.Fatal("no toolbar island present")
		}
		if !strings.Contains(body[max(0, idx-40):idx], "hidden") {
			t.Error("the toolbar island does not start hidden")
		}
		if !strings.Contains(body, `data-logout-form-id="logout-form"`) {
			t.Error("the toolbar island is not wired to the real logout form")
		}
		// The items list must be real JSON, not empty, and must not blindly trust
		// anything client-controllable — it is built entirely server-side from the
		// signed-in role, so this also doubles as an RBAC-in-the-nav sanity check.
		if !strings.Contains(body, `id&#34;:&#34;logout`) || !strings.Contains(body, `id&#34;:&#34;timeline`) {
			t.Error("the toolbar's item list is missing expected entries")
		}
	})

	t.Run("driver does not get the planner nav items", func(t *testing.T) {
		e.newDriverWithLogin("Driver A", "driver@example.lv", testPassword)
		dc := newClient(t, base)
		dc.login("driver@example.lv")
		body := dc.body("/my/trips")

		if strings.Contains(body, `id&#34;:&#34;users`) {
			t.Error("a driver's nav items JSON includes the admin-only users link")
		}
		if !strings.Contains(body, `id&#34;:&#34;my_trips`) {
			t.Error("a driver's nav items JSON is missing their own trips link")
		}
	})

	t.Run("form save button", func(t *testing.T) {
		body := c.body("/buses/new")

		if !strings.Contains(body, `id="save-button" type="submit"`) {
			t.Fatal("the real save button is missing")
		}
		idx := strings.Index(body, `data-kind="button"`)
		if idx == -1 {
			t.Fatal("no button island present")
		}
		if !strings.Contains(body[max(0, idx-40):idx], "hidden") {
			t.Error("the button island does not start hidden")
		}
		if !strings.Contains(body, `data-input-id="save-button"`) {
			t.Error("the button island is not wired to the real save button")
		}
	})

	t.Run("password forms are left alone", func(t *testing.T) {
		body := c.body("/users/1/password")
		if strings.Contains(body, `data-kind="button"`) {
			t.Error("the password form's submit button was enhanced; it should stay plain, same as the password field itself")
		}
	})
}

// TestFlashNotificationIsland triggers a real validation failure to check the error
// banner's island wiring, and confirms the conflict list (when present) survives
// outside the hideable banner — see the comment on Flash() in components.templ.
func TestFlashNotificationIsland(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	c := newClient(t, base)
	c.login("admin@example.lv")

	// An empty plate fails validation server-side and re-renders the form with an
	// error flash.
	resp := c.post("/buses/new", "/buses", url.Values{
		"plate":  {""},
		"model":  {"Setra S515"},
		"seats":  {"50"},
		"status": {"active"},
	})
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("reading response body: %v", err)
	}
	respBody := string(raw)

	if !strings.Contains(respBody, `id="flash-banner"`) {
		t.Fatal("the real, working error banner is missing")
	}
	idx := strings.Index(respBody, `data-kind="notification"`)
	if idx == -1 {
		t.Fatal("no notification island present")
	}
	if !strings.Contains(respBody[max(0, idx-40):idx], "hidden") {
		t.Error("the notification island does not start hidden")
	}
	if !strings.Contains(respBody, `data-variant="error"`) {
		t.Error("the notification island is not marked as an error")
	}
}
