package http

import (
	"errors"
	"net/http"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/domain"
)

// apiUser is the identity shape the SPA gets back. PasswordHash never enters this
// package at all (domain.User keeps it out of Identity), so there is nothing to
// accidentally leak here.
type apiUser struct {
	ID       int64  `json:"id"`
	Email    string `json:"email"`
	Role     string `json:"role"`
	DriverID *int64 `json:"driverId,omitempty"`
	Locale   string `json:"locale,omitempty"`
}

type apiMeResponse struct {
	Authenticated bool     `json:"authenticated"`
	User          *apiUser `json:"user,omitempty"`
	CSRFToken     string   `json:"csrfToken"`
}

func apiUserFrom(identity domain.Identity) *apiUser {
	u := &apiUser{
		ID:    identity.UserID,
		Email: identity.Email,
		Role:  string(identity.Role),
	}
	if identity.DriverID != nil {
		u.DriverID = identity.DriverID
	}
	if identity.Locale != nil {
		u.Locale = *identity.Locale
	}
	return u
}

// handleAPIMe is how the SPA bootstraps: on every fresh load it asks who (if anyone) is
// signed in and gets back the CSRF token it must echo on every state-changing request
// from here on — the anonymous token from LoadSession pre-login, the session-bound one
// after. No cookie access from JavaScript is needed either way.
func (s *Server) handleAPIMe(w http.ResponseWriter, r *http.Request) {
	identity, ok := IdentityFrom(r.Context())
	if !ok {
		s.writeJSON(w, r, http.StatusOK, apiMeResponse{
			Authenticated: false,
			CSRFToken:     CSRFTokenFrom(r.Context()),
		})
		return
	}
	s.writeJSON(w, r, http.StatusOK, apiMeResponse{
		Authenticated: true,
		User:          apiUserFrom(identity),
		CSRFToken:     CSRFTokenFrom(r.Context()),
	})
}

type apiLoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// handleAPILogin mirrors handleLoginSubmit exactly — same service call, same cookie,
// same "one message for every failure mode" rule — just JSON in and out instead of a
// form post and a redirect.
func (s *Server) handleAPILogin(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	var req apiLoginRequest
	if err := readJSON(r, &req); err != nil {
		s.writeJSON(w, r, http.StatusBadRequest, apiError{Error: s.printerFor(r).T("error.validation")})
		return
	}

	token, _, identity, err := s.auth.Login(ctx, req.Email, req.Password, s.meta(r))
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) {
			s.log.Warn("failed login", "request_id", RequestIDFrom(ctx))
			s.writeJSON(w, r, http.StatusUnauthorized, apiError{Error: s.printerFor(r).T("login.error.invalid")})
			return
		}
		s.log.Error("login failed", "request_id", RequestIDFrom(ctx), "error", err)
		s.writeJSON(w, r, http.StatusInternalServerError, apiError{Error: s.printerFor(r).T("error.server")})
		return
	}

	s.setSessionCookie(w, token)
	s.clearCSRFNonceCookie(w)

	// The identity Login() returns carries no CSRFSecret — that field only exists on
	// the session row, which LoadSession normally resolves on the *next* request. An
	// HTML login has no need of it (it only redirects); this one must hand the SPA a
	// working token immediately, so it resolves the session it just created rather
	// than computing a token over a zero-value secret.
	resolved, err := s.auth.Sessions().Resolve(ctx, token)
	if err != nil {
		s.log.Error("resolving freshly created session", "request_id", RequestIDFrom(ctx), "error", err)
		s.writeJSON(w, r, http.StatusInternalServerError, apiError{Error: s.printerFor(r).T("error.server")})
		return
	}

	s.writeJSON(w, r, http.StatusOK, apiMeResponse{
		Authenticated: true,
		User:          apiUserFrom(identity),
		CSRFToken:     auth.SessionCSRFToken(resolved.CSRFSecret),
	})
}

func (s *Server) handleAPILogout(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	identity, _ := IdentityFrom(ctx)

	if err := s.auth.Logout(ctx, sessionTokenFrom(ctx), identity, s.meta(r)); err != nil {
		s.log.Error("logout failed", "request_id", RequestIDFrom(ctx), "error", err)
		s.writeJSON(w, r, http.StatusInternalServerError, apiError{Error: s.printerFor(r).T("error.server")})
		return
	}
	s.clearSessionCookie(w)
	s.writeJSON(w, r, http.StatusOK, map[string]bool{"ok": true})
}
