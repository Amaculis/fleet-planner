package http

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/buscompany/bus_fleet/internal/config"
)

// TestProductionCookiesAreLockedDown: in production the session and CSRF cookies are
// __Host- prefixed, Secure, HttpOnly, SameSite=Lax and host-only.
func TestProductionCookiesAreLockedDown(t *testing.T) {
	s := &Server{cfg: config.Config{Env: "production"}}
	rec := httptest.NewRecorder()
	s.setSessionCookie(rec, "token")
	s.setCSRFNonceCookie(rec, "nonce")

	cookies := map[string]*http.Cookie{}
	for _, c := range rec.Result().Cookies() {
		cookies[c.Name] = c
	}
	for _, name := range []string{"__Host-fleet_session", "__Host-fleet_csrf"} {
		c, ok := cookies[name]
		if !ok {
			t.Errorf("cookie %s not set; got %v", name, rec.Result().Cookies())
			continue
		}
		if !c.Secure || !c.HttpOnly || c.SameSite != http.SameSiteLaxMode || c.Path != "/" || c.Domain != "" {
			t.Errorf("%s = %+v, want Secure, HttpOnly, SameSite=Lax, Path=/, no Domain", name, c)
		}
	}
	if s := cookies["__Host-fleet_session"]; s != nil && (s.MaxAge != 0 || !s.Expires.IsZero()) {
		t.Errorf("session cookie persists on disk (MaxAge %d, Expires %v); the server-side row is the expiry", s.MaxAge, s.Expires)
	}
}
