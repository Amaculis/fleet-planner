package http

import (
	"net/http"

	"github.com/buscompany/bus_fleet/internal/domain"
)

// Clients are planner-only: every route below sits in the admin/dispatcher group (the
// erasure route in the admin-only one), and no driver-facing shape carries any of this.

type apiClient struct {
	ID            int64   `json:"id"`
	Name          string  `json:"name"`
	ContactPerson *string `json:"contactPerson,omitempty"`
	Phone         *string `json:"phone,omitempty"`
	Email         *string `json:"email,omitempty"`
	Address       *string `json:"address,omitempty"`
	Notes         *string `json:"notes,omitempty"`
	IsActive      bool    `json:"isActive"`
	Anonymized    bool    `json:"anonymized"`
}

func apiClientFrom(c domain.Client) apiClient {
	return apiClient{
		ID:            c.ID,
		Name:          c.Name,
		ContactPerson: c.ContactPerson,
		Phone:         c.Phone,
		Email:         c.Email,
		Address:       c.Address,
		Notes:         c.Notes,
		IsActive:      c.IsActive,
		Anonymized:    c.IsAnonymized(),
	}
}

type apiClientRequest struct {
	Name          string  `json:"name"`
	ContactPerson *string `json:"contactPerson"`
	Phone         *string `json:"phone"`
	Email         *string `json:"email"`
	Address       *string `json:"address"`
	Notes         *string `json:"notes"`
	IsActive      bool    `json:"isActive"`
}

func (req apiClientRequest) toDomain(id int64, isNew bool) domain.Client {
	client := domain.Client{
		ID:            id,
		Name:          req.Name,
		ContactPerson: req.ContactPerson,
		Phone:         req.Phone,
		Email:         req.Email,
		Address:       req.Address,
		Notes:         req.Notes,
		IsActive:      true,
	}
	if !isNew {
		client.IsActive = req.IsActive
	}
	return client
}

func (s *Server) handleAPIClientList(w http.ResponseWriter, r *http.Request) {
	identity := MustIdentity(r.Context())
	var (
		clients []domain.Client
		err     error
	)
	if isAssignable(r) {
		clients, err = s.clients.ListSelectable(r.Context(), identity)
	} else {
		clients, err = s.clients.List(r.Context(), identity)
	}
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	out := make([]apiClient, len(clients))
	for i, c := range clients {
		out[i] = apiClientFrom(c)
	}
	s.writeJSON(w, r, http.StatusOK, out)
}

func (s *Server) handleAPIClientGet(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	client, err := s.clients.Get(r.Context(), MustIdentity(r.Context()), id)
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, apiClientFrom(client))
}

func (s *Server) handleAPIClientCreate(w http.ResponseWriter, r *http.Request) {
	var req apiClientRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	client, err := s.clients.Create(r.Context(), MustIdentity(r.Context()), req.toDomain(0, true), s.meta(r))
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusCreated, apiClientFrom(client))
}

func (s *Server) handleAPIClientUpdate(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	var req apiClientRequest
	if err := readJSON(r, &req); err != nil {
		s.writeAPIError(w, r, domain.ErrValidation)
		return
	}
	client, err := s.clients.Update(r.Context(), MustIdentity(r.Context()), req.toDomain(id, false), s.meta(r))
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, apiClientFrom(client))
}

// handleAPIClientAnonymize is the GDPR erasure action — irreversible, hence its own
// endpoint rather than a field on the update request (same as drivers).
func (s *Server) handleAPIClientAnonymize(w http.ResponseWriter, r *http.Request) {
	id, err := idParam(r, "id")
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	client, err := s.clients.Anonymize(r.Context(), MustIdentity(r.Context()), id, s.meta(r))
	if err != nil {
		s.writeAPIError(w, r, err)
		return
	}
	s.writeJSON(w, r, http.StatusOK, apiClientFrom(client))
}
