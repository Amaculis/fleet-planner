package http

import (
	"net/http"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/web/templates"
)

// User management. Admin-only: the route group is wrapped in RequireRole(admin) and
// every service method re-checks.

func (s *Server) handleUserList(w http.ResponseWriter, r *http.Request) {
	users, err := s.users.List(r.Context(), MustIdentity(r.Context()))
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.UsersPage(s.view(r), users))
}

func (s *Server) handleUserNew(w http.ResponseWriter, r *http.Request) {
	form, err := s.userForm(r)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.UserFormPage(s.view(r), form))
}

func (s *Server) userForm(r *http.Request) (templates.UserForm, error) {
	// Only active drivers can be linked to a new driver login.
	drivers, err := s.fleet.ListAssignableDrivers(r.Context(), MustIdentity(r.Context()))
	if err != nil {
		return templates.UserForm{}, err
	}
	return templates.UserForm{Drivers: drivers}, nil
}

func (s *Server) handleUserCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		s.abort(w, r, domain.ErrValidation)
		return
	}
	email := formString(r, "email")
	password := r.PostFormValue("password") // never trimmed, never logged
	role := domain.Role(formString(r, "role"))

	var driverID *int64
	if raw := formString(r, "driver_id"); raw != "" {
		id, err := formInt(r, "driver_id", "field.driver")
		if err != nil {
			s.renderUserFormError(w, r, err)
			return
		}
		driverID = &id
	}

	if _, err := s.users.Create(r.Context(), MustIdentity(r.Context()), email, password, role, driverID, s.meta(r)); err != nil {
		s.renderUserFormError(w, r, err)
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (s *Server) handleUserActivate(w http.ResponseWriter, r *http.Request) {
	s.setUserActive(w, r, true)
}
func (s *Server) handleUserDeactivate(w http.ResponseWriter, r *http.Request) {
	s.setUserActive(w, r, false)
}

func (s *Server) setUserActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	if _, err := s.users.SetActive(r.Context(), MustIdentity(r.Context()), id, active, s.meta(r)); err != nil {
		s.abort(w, r, err)
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (s *Server) handleUserPasswordForm(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	user, err := s.users.Get(r.Context(), MustIdentity(r.Context()), id)
	if err != nil {
		s.abort(w, r, err)
		return
	}
	s.render(w, r, http.StatusOK, templates.UserPasswordPage(s.view(r), user))
}

func (s *Server) handleUserPasswordSet(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.abort(w, r, err)
		return
	}
	// Setting a password logs that account out everywhere (see UserService).
	if err := s.users.SetPassword(r.Context(), MustIdentity(r.Context()), id, r.PostFormValue("password"), s.meta(r)); err != nil {
		status, msg := s.failed(r, err)
		user, getErr := s.users.Get(r.Context(), MustIdentity(r.Context()), id)
		if getErr != nil {
			s.abort(w, r, getErr)
			return
		}
		s.render(w, r, status, templates.UserPasswordPage(s.viewWithError(r, msg), user))
		return
	}
	http.Redirect(w, r, "/users", http.StatusSeeOther)
}

func (s *Server) renderUserFormError(w http.ResponseWriter, r *http.Request, cause error) {
	status, msg := s.failed(r, cause)
	form, err := s.userForm(r)
	if err != nil {
		s.abort(w, r, cause)
		return
	}
	s.render(w, r, status, templates.UserFormPage(s.viewWithError(r, msg), form))
}
