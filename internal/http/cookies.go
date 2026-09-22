package http

import (
	"net/http"
	"time"
)

// Cookie names. The __Host- prefix is the strongest binding a browser offers: the
// cookie must be Secure, Path=/ and carry no Domain, so a sibling or parent host cannot
// overwrite it. It requires HTTPS, so plain-HTTP development falls back to a plain name.
const (
	sessionCookieProd = "__Host-fleet_session"
	sessionCookieDev  = "fleet_session"
	csrfCookieProd    = "__Host-fleet_csrf"
	csrfCookieDev     = "fleet_csrf"
)

func (s *Server) sessionCookieName() string {
	if s.cfg.IsProduction() {
		return sessionCookieProd
	}
	return sessionCookieDev
}

func (s *Server) csrfCookieName() string {
	if s.cfg.IsProduction() {
		return csrfCookieProd
	}
	return csrfCookieDev
}

// setSessionCookie stores the opaque token. HttpOnly keeps it away from JavaScript
// (so an XSS bug cannot steal it), SameSite=Lax blocks cross-site form posts while
// keeping normal navigation working, and no Expires/Max-Age is set: the cookie is a
// session cookie and the server-side row is the real expiry.
func (s *Server) setSessionCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
	})
}

func (s *Server) clearSessionCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.sessionCookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

// setCSRFNonceCookie holds the nonce behind the anonymous (pre-login) CSRF token. The
// token itself is an HMAC of this nonce under the app secret, so possession of the
// cookie alone does not let an attacker forge a form value.
func (s *Server) setCSRFNonceCookie(w http.ResponseWriter, nonce string) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.csrfCookieName(),
		Value:    nonce,
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   int((30 * time.Minute).Seconds()),
	})
}

func (s *Server) clearCSRFNonceCookie(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{
		Name:     s.csrfCookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   s.cfg.IsProduction(),
		SameSite: http.SameSiteLaxMode,
		MaxAge:   -1,
		Expires:  time.Unix(0, 0),
	})
}

func cookieValue(r *http.Request, name string) string {
	c, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return c.Value
}
