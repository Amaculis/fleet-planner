package http

import (
	"errors"
	"net/http"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
	"github.com/buscompany/bus_fleet/web/templates"
)

func (s *Server) handleLoginForm(w http.ResponseWriter, r *http.Request) {
	if _, ok := IdentityFrom(r.Context()); ok {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	// Only a fixed set of notices can be shown; the query value is never echoed.
	notice := ""
	switch r.URL.Query().Get("notice") {
	case "logged_out":
		notice = "login.logged_out"
	case "expired":
		notice = "login.error.expired"
	}
	s.renderLogin(w, r, http.StatusOK, "", notice)
}

func (s *Server) handleLoginSubmit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// The CSRF middleware has already parsed and validated the form.
	email := r.PostFormValue("email")
	password := r.PostFormValue("password")

	meta := service.Meta{IP: s.clientIP(r), RequestID: RequestIDFrom(ctx)}

	token, _, identity, err := s.auth.Login(ctx, email, password, meta)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			// One message for every failure mode: unknown email, wrong password and
			// deactivated account are indistinguishable to the client.
			// Neither the email nor the IP goes into the application log — personal data
			// belongs in the audit log, where the service already recorded this attempt.
			s.log.Warn("failed login", "request_id", RequestIDFrom(ctx))
			s.renderLogin(w, r, http.StatusUnauthorized, "login.error.invalid", "")
			return
		}
		s.log.Error("login failed", "request_id", RequestIDFrom(ctx), "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "error.server")
		return
	}

	// Fresh token + the pre-login CSRF nonce discarded: nothing from the anonymous
	// request survives into the authenticated session.
	s.setSessionCookie(w, token)
	s.clearCSRFNonceCookie(w)

	http.Redirect(w, r, landingPath(identity), http.StatusSeeOther)
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	identity, _ := IdentityFrom(ctx)
	meta := service.Meta{IP: s.clientIP(r), RequestID: RequestIDFrom(ctx)}

	if err := s.auth.Logout(ctx, sessionTokenFrom(ctx), identity, meta); err != nil {
		s.log.Error("logout failed", "request_id", RequestIDFrom(ctx), "error", err)
		s.renderError(w, r, http.StatusInternalServerError, "error.server")
		return
	}
	s.clearSessionCookie(w)

	if isHTMX(r) {
		w.Header().Set("HX-Redirect", "/login?notice=logged_out")
		w.WriteHeader(http.StatusOK)
		return
	}
	http.Redirect(w, r, "/login?notice=logged_out", http.StatusSeeOther)
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	identity := MustIdentity(r.Context())
	page := templates.HomePage(
		PrinterFrom(r.Context()),
		cspNonceFrom(r.Context()),
		CSRFTokenFrom(r.Context()),
		identity,
	)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := page.Render(r.Context(), w); err != nil {
		s.log.Error("rendering home", "request_id", RequestIDFrom(r.Context()), "error", err)
	}
}

func (s *Server) renderLogin(w http.ResponseWriter, r *http.Request, status int, errorKey, noticeKey string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	page := templates.LoginPage(
		PrinterFrom(r.Context()),
		cspNonceFrom(r.Context()),
		CSRFTokenFrom(r.Context()),
		errorKey,
		noticeKey,
	)
	if err := page.Render(r.Context(), w); err != nil {
		s.log.Error("rendering login", "request_id", RequestIDFrom(r.Context()), "error", err)
	}
}

// landingPath sends each role to the view built for it. Convenience only — the
// authorization that matters is RequireRole on the route itself.
func landingPath(identity domain.Identity) string {
	if identity.Role == domain.RoleDriver {
		return "/my/trips"
	}
	return "/timeline"
}
