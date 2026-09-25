package http

import (
	"net/http"

	"github.com/a-h/templ"

	"github.com/buscompany/bus_fleet/internal/service"
	"github.com/buscompany/bus_fleet/web/templates"
)

// view builds the per-request bag every page template takes: translator, CSP nonce,
// CSRF token, identity and display time zone.
func (s *Server) view(r *http.Request) templates.View {
	identity, _ := IdentityFrom(r.Context())
	return templates.View{
		P:        s.printerFor(r),
		Nonce:    cspNonceFrom(r.Context()),
		CSRF:     CSRFTokenFrom(r.Context()),
		Identity: identity,
		Loc:      s.cfg.Location,
		// Used only to highlight the active nav link; never trusted for anything else.
		Path: r.URL.Path,
	}
}

// viewWithError is the same view carrying a failure: a translated headline and, for a
// booking clash, the overlapping trips.
func (s *Server) viewWithError(r *http.Request, msg UserMessage) templates.View {
	v := s.view(r)
	v.Message = msg.Text
	v.IsError = true
	v.Conflicts = msg.Conflicts
	return v
}

// render writes a templ component with the given status.
func (s *Server) render(w http.ResponseWriter, r *http.Request, status int, page templ.Component) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(status)
	if err := page.Render(r.Context(), w); err != nil {
		// The status line is already sent, so this can only be logged.
		s.log.Error("rendering page",
			"request_id", RequestIDFrom(r.Context()),
			"path", r.URL.Path,
			"error", err)
	}
}

// meta carries the request facts an audit entry needs into the service layer.
func (s *Server) meta(r *http.Request) service.Meta {
	return service.Meta{IP: s.clientIP(r), RequestID: RequestIDFrom(r.Context())}
}
