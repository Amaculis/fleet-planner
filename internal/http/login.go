package http

import (
	"context"
	"errors"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/service"
)

// errAccountThrottled means this email has had too many failed logins recently. The
// handlers answer it with 429, exactly like the per-IP limiter.
var errAccountThrottled = errors.New("account throttled")

// attemptLogin is the one login path both the HTML form and the JSON API use. On top of
// the per-IP limiter (route middleware) it applies a per-account throttle that only
// failed attempts spend, and it retires any session the browser was still holding, so a
// re-login never leaves an old token alive in the database.
func (s *Server) attemptLogin(ctx context.Context, email, password string, meta service.Meta) (string, domain.Identity, error) {
	key := service.NormalizeEmail(email)
	if key != "" && !s.accountLimiter.HasToken(key) {
		return "", domain.Identity{}, errAccountThrottled
	}

	token, _, identity, err := s.auth.Login(ctx, email, password, meta)
	if err != nil {
		if errors.Is(err, domain.ErrUnauthorized) && key != "" {
			s.accountLimiter.Allow(key)
		}
		return "", domain.Identity{}, err
	}

	if old := sessionTokenFrom(ctx); old != "" {
		if err := s.auth.Sessions().Destroy(ctx, old); err != nil {
			// Not fatal: the old session still expires on its own.
			s.log.Warn("retiring previous session", "request_id", RequestIDFrom(ctx), "error", err)
		}
	}
	return token, identity, nil
}
