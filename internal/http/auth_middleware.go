package http

import (
	"errors"
	"net/http"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/domain"
)

// LoadSession resolves the session cookie into an Identity and prepares the request's
// translator and CSRF token. It never rejects a request on its own — RequireAuth and
// RequireRole do that — so public pages (login, static, health) share one pipeline.
func (s *Server) LoadSession(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		var (
			identity domain.Identity
			haveID   bool
		)

		if token := cookieValue(r, s.sessionCookieName()); token != "" {
			id, err := s.auth.Sessions().Resolve(ctx, token)
			switch {
			case err == nil:
				identity, haveID = id, true
				r = withValue(r, ctxKeySessionToken, token)
			case errors.Is(err, domain.ErrUnauthorized):
				// Expired, unknown or belonging to a deactivated user: drop the cookie
				// so the browser stops sending it.
				s.clearSessionCookie(w)
			default:
				s.log.Error("resolving session", "request_id", RequestIDFrom(ctx), "error", err)
				s.renderError(w, r, http.StatusInternalServerError, "error.server")
				return
			}
		}

		// Locale: user preference (server-side) -> Accept-Language -> en.
		var userPref *string
		if haveID {
			userPref = identity.Locale
		}
		printer := s.i18n.Printer(s.i18n.Match(userPref, r.Header.Get("Accept-Language")))
		r = withValue(r, ctxKeyPrinter, printer)

		if haveID {
			r = withValue(r, ctxKeyIdentity, identity)
			r = withValue(r, ctxKeyCSRFToken, auth.SessionCSRFToken(identity.CSRFSecret))
		} else {
			// Anonymous visitors still need a CSRF token for the login form.
			nonce := cookieValue(r, s.csrfCookieName())
			if nonce == "" {
				newNonce, err := auth.NewCSRFNonce()
				if err != nil {
					s.log.Error("generating csrf nonce", "request_id", RequestIDFrom(ctx), "error", err)
					s.renderError(w, r, http.StatusInternalServerError, "error.server")
					return
				}
				nonce = newNonce
				s.setCSRFNonceCookie(w, nonce)
			}
			r = withValue(r, ctxKeyCSRFToken, auth.AnonCSRFToken(s.cfg.AppSecret, []byte(nonce)))
		}

		next.ServeHTTP(w, r)
	})
}

// CSRF verifies the synchronizer token on every state-changing request. GET, HEAD and
// OPTIONS are exempt because they must not change state; any handler that would break
// that rule is the bug.
func (s *Server) CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			next.ServeHTTP(w, r)
			return
		}

		// Accept the token from the form field or the htmx-friendly header.
		submitted := r.Header.Get("X-CSRF-Token")
		if submitted == "" {
			// ParseForm is bounded by MaxBytesReader in SecurityHeaders.
			if err := r.ParseForm(); err != nil {
				s.renderError(w, r, http.StatusBadRequest, "error.csrf")
				return
			}
			submitted = r.PostFormValue("csrf_token")
		}

		if !auth.ValidCSRFToken(CSRFTokenFrom(r.Context()), submitted) {
			s.log.Warn("csrf rejected",
				"request_id", RequestIDFrom(r.Context()),
				"method", r.Method,
				"path", r.URL.Path)
			s.renderError(w, r, http.StatusForbidden, "error.csrf")
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireAuth rejects anonymous callers. Mount it in front of every non-public route.
func (s *Server) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := IdentityFrom(r.Context()); !ok {
			if isHTMX(r) {
				// Tell htmx to reload the page so the login form replaces the fragment.
				w.Header().Set("HX-Redirect", "/login")
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// RequireRole authorizes server-side, before the handler runs. Hiding a link in a
// template is presentation, never access control: every non-public route is wrapped
// here, and ownership checks (a driver's own data) happen additionally in the handler
// or service using Identity.DriverID.
func (s *Server) RequireRole(allowed ...domain.Role) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			identity, ok := IdentityFrom(r.Context())
			if !ok {
				// Defensive: RequireAuth should already have run.
				http.Redirect(w, r, "/login", http.StatusSeeOther)
				return
			}
			for _, role := range allowed {
				if identity.Role == role {
					next.ServeHTTP(w, r)
					return
				}
			}
			s.log.Warn("authorization denied",
				"request_id", RequestIDFrom(r.Context()),
				"user_id", identity.UserID,
				"role", string(identity.Role),
				"method", r.Method,
				"path", r.URL.Path)
			s.renderError(w, r, http.StatusForbidden, "error.forbidden")
		})
	}
}

func isHTMX(r *http.Request) bool { return r.Header.Get("HX-Request") == "true" }
