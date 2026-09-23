package http

import (
	"net/http"

	"github.com/buscompany/bus_fleet/internal/domain"
)

type apiUserListItem struct {
	ID         int64   `json:"id"`
	Email      string  `json:"email"`
	Role       string  `json:"role"`
	DriverID   *int64  `json:"driverId,omitempty"`
	DriverName *string `json:"driverName,omitempty"`
	IsActive   bool    `json:"isActive"`
}

func apiUserListItemFrom(u domain.UserListItem) apiUserListItem {
	return apiUserListItem{
		ID: u.ID, Email: u.Email, Role: string(u.Role),
		DriverID: u.DriverID, DriverName: u.DriverName, IsActive: u.IsActive,
	}
}

func (s *Server) handleAPIUserList(w http.ResponseWriter, r *http.Request) {
	users, err := s.users.List(r.Context(), MustIdentity(r.Context()))
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	out := make([]apiUserListItem, len(users))
	for i, u := range users {
		out[i] = apiUserListItemFrom(u)
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

type apiUserCreateRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
	Role     string `json:"role"`
	DriverID *int64 `json:"driverId"`
}

func (s *Server) handleAPIUserCreate(w http.ResponseWriter, r *http.Request) {
	var req apiUserCreateRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	user, err := s.users.Create(r.Context(), MustIdentity(r.Context()), req.Email, req.Password,
		domain.Role(req.Role), req.DriverID, s.meta(r))
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, apiUserFrom(domain.Identity{
		UserID: user.ID, Email: user.Email, Role: user.Role, DriverID: user.DriverID, Locale: user.Locale,
	}))
}

func (s *Server) handleAPIUserActivate(w http.ResponseWriter, r *http.Request) {
	s.setAPIUserActive(w, r, true)
}

func (s *Server) handleAPIUserDeactivate(w http.ResponseWriter, r *http.Request) {
	s.setAPIUserActive(w, r, false)
}

func (s *Server) setAPIUserActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	if _, err := s.users.SetActive(r.Context(), MustIdentity(r.Context()), id, active, s.meta(r)); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

type apiSetPasswordRequest struct {
	Password string `json:"password"`
}

// handleAPIUserSetPassword logs the account out everywhere — same as the HTML flow
// (see UserService.SetPassword) — the SPA has no separate "log out other sessions"
// step to skip.
func (s *Server) handleAPIUserSetPassword(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	var req apiSetPasswordRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	if err := s.users.SetPassword(r.Context(), MustIdentity(r.Context()), id, req.Password, s.meta(r)); err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
