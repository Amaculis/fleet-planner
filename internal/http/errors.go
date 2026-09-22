package http

import (
	"errors"
	"net/http"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/i18n"
	"github.com/buscompany/bus_fleet/internal/service"
)

// UserMessage is a failure rendered back into a page: a translated headline plus, for a
// booking clash, the trips that are in the way.
type UserMessage struct {
	Text      string
	Conflicts []domain.Conflict
}

// describeError maps an error to (status, message). Internal detail never reaches the
// client: anything unrecognised becomes a generic message and is logged instead.
func describeError(p *i18n.Printer, err error) (int, UserMessage) {
	// A booking clash carries the overlapping trips, so the user can see what blocks it.
	var conflict *service.ConflictError
	if errors.As(err, &conflict) {
		return http.StatusConflict, UserMessage{
			Text:      p.T(conflict.MessageKey(), conflict.Name),
			Conflicts: conflict.Conflicts,
		}
	}

	// Services report failures as message ids so they render in the user's language.
	if msg, ok := service.AsMessage(err); ok {
		args := msg.Args
		if msg.FieldKey != "" {
			args = append([]any{p.T(msg.FieldKey)}, msg.Args...)
		}
		return statusFor(msg.Sentinel), UserMessage{Text: p.T(msg.Key, args...)}
	}

	switch {
	case errors.Is(err, domain.ErrNotFound):
		return http.StatusNotFound, UserMessage{Text: p.T("error.not_found")}
	case errors.Is(err, domain.ErrForbidden):
		return http.StatusForbidden, UserMessage{Text: p.T("error.forbidden")}
	case errors.Is(err, domain.ErrUnauthorized):
		return http.StatusUnauthorized, UserMessage{Text: p.T("error.forbidden")}
	case errors.Is(err, domain.ErrTimeConflict):
		// The EXCLUDE constraint fired without a pre-check catching it first — a race.
		return http.StatusConflict, UserMessage{Text: p.T("error.time_conflict")}
	case errors.Is(err, domain.ErrConflict):
		return http.StatusConflict, UserMessage{Text: p.T("error.conflict")}
	case errors.Is(err, domain.ErrValidation):
		return http.StatusBadRequest, UserMessage{Text: p.T("error.validation")}
	default:
		return http.StatusInternalServerError, UserMessage{Text: p.T("error.server")}
	}
}

func statusFor(sentinel error) int {
	switch {
	case errors.Is(sentinel, domain.ErrNotFound):
		return http.StatusNotFound
	case errors.Is(sentinel, domain.ErrForbidden):
		return http.StatusForbidden
	case errors.Is(sentinel, domain.ErrTimeConflict), errors.Is(sentinel, domain.ErrConflict):
		return http.StatusConflict
	case errors.Is(sentinel, domain.ErrValidation):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

// failed logs the internal detail and returns what the page should show. Server errors
// are logged at error level with the request id; expected outcomes (validation, a
// booking clash) are the user's business, not an incident.
func (s *Server) failed(r *http.Request, err error) (int, UserMessage) {
	printer := PrinterFrom(r.Context())
	if printer == nil {
		// The request failed before its locale was resolved.
		printer = s.i18n.Printer(i18n.Fallback)
	}
	status, msg := describeError(printer, err)
	if status >= http.StatusInternalServerError {
		s.log.Error("request failed",
			"request_id", RequestIDFrom(r.Context()),
			"method", r.Method,
			"path", r.URL.Path,
			"error", err)
	} else {
		s.log.Debug("request rejected",
			"request_id", RequestIDFrom(r.Context()),
			"status", status,
			"path", r.URL.Path)
	}
	return status, msg
}

// abort renders the standalone error page. Handlers that can re-render their own form
// with the message inline should do that instead.
func (s *Server) abort(w http.ResponseWriter, r *http.Request, err error) {
	status, msg := s.failed(r, err)
	s.renderErrorText(w, r, status, msg.Text)
}
