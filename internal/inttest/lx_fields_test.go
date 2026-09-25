package inttest

import (
	"net/url"
	"strconv"
	"strings"
	"testing"
)

// TestFormFieldIslandsProgressiveEnhancement checks the same contract as
// TestCalendarIslandProgressiveEnhancement (timeline_test.go), extended to the text,
// toggle and select field islands added to the buses/drivers/trips/users forms: every
// enhanced field still has a real, working, non-hidden native control underneath, and
// the island mount point starts hidden so a page where the script never runs is fully
// usable. It cannot drive an actual browser — see docs/lx-ui-integration.md.
func TestFormFieldIslandsProgressiveEnhancement(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	c := newClient(t, base)
	c.login("admin@example.lv")

	tests := []struct {
		page   string
		fields []string // native input/select ids expected, each with a sibling island
		kinds  []string // data-kind values expected somewhere on the page
	}{
		{page: "/buses/new", fields: []string{"plate", "model", "status"}, kinds: []string{"text", "select"}},
		{page: "/drivers/new", fields: []string{"full_name", "phone", "license_number"}, kinds: []string{"text"}},
		{page: "/users/new", fields: []string{"email", "role", "driver_id"}, kinds: []string{"text", "select"}},
	}

	for _, tt := range tests {
		t.Run(tt.page, func(t *testing.T) {
			body := c.body(tt.page)

			for _, id := range tt.fields {
				if !strings.Contains(body, `id="`+id+`"`) {
					t.Errorf("%s: native field %q is missing", tt.page, id)
				}
			}
			for _, kind := range tt.kinds {
				marker := `data-kind="` + kind + `"`
				idx := strings.Index(body, marker)
				if idx == -1 {
					t.Errorf("%s: no %s field island present", tt.page, marker)
					continue
				}
				if !strings.Contains(body[max(0, idx-60):idx], "hidden") {
					t.Errorf("%s: %s field island does not start hidden", tt.page, marker)
				}
			}
			// Every enhanced <select>'s options must still be real, working options —
			// the lx-ui dropdown is a visual replacement, never the only way to choose.
			if strings.Contains(body, `id="status"`) && !strings.Contains(body, "<option") {
				t.Errorf("%s: the status <select> has no <option> elements", tt.page)
			}
		})
	}
}

// TestDriverToggleFieldIsland: the is_active checkbox (edit form only — new drivers
// default active) gets the same treatment, with its own labelling wrinkle: the label
// wraps the checkbox rather than using a for= id, since that is the pattern the rest of
// the app already used for inline checkbox+label pairs.
func TestDriverToggleFieldIsland(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)
	driver := e.newDriver("Driver A")

	base := e.serve(t)
	c := newClient(t, base)
	c.login("admin@example.lv")

	body := c.body("/drivers/" + strconv.FormatInt(driver.ID, 10) + "/edit")

	if !strings.Contains(body, `id="is_active"`) {
		t.Fatal("the native is_active checkbox is missing")
	}
	if !strings.Contains(body, `data-kind="toggle"`) {
		t.Fatal("no toggle field island present on the driver edit form")
	}
	if !strings.Contains(body, `id="is_active-label"`) {
		t.Error("the toggle's label has no id for the island to reference via aria-labelledby")
	}
}

// TestFormSubmissionUnaffectedByFieldIslands is the test that actually matters most:
// the presence of a hidden mount point next to every enhanced field must not change
// what a plain form POST does. This drives the real create-bus flow exactly as a
// browser with JavaScript disabled would — through the native fields only — and checks
// it still works end to end.
func TestFormSubmissionUnaffectedByFieldIslands(t *testing.T) {
	e := newEnv(t)
	e.newAdmin("admin@example.lv", testPassword)

	base := e.serve(t)
	c := newClient(t, base)
	c.login("admin@example.lv")

	resp := c.post("/buses/new", "/buses", url.Values{
		"plate":  {"CC-3333"},
		"model":  {"MAN Lion's Coach"},
		"seats":  {"55"},
		"status": {"active"},
	})
	defer resp.Body.Close()
	if resp.StatusCode != 303 {
		body := c.body("/buses")
		t.Fatalf("creating a bus via the plain form fields: status %d (buses page: %.200s)", resp.StatusCode, body)
	}

	if !strings.Contains(c.body("/buses"), "CC-3333") {
		t.Error("the bus created through the plain form fields does not appear in the list")
	}
}
