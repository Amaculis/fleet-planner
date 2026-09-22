package repo

import (
	"context"
	"fmt"
	"time"

	"github.com/buscompany/bus_fleet/internal/domain"
	"github.com/buscompany/bus_fleet/internal/repo/sqlcgen"
)

func (r *Repo) CreateSession(ctx context.Context, s domain.Session) error {
	_, err := r.q.CreateSession(ctx, sqlcgen.CreateSessionParams{
		TokenHash:  s.TokenHash,
		UserID:     s.UserID,
		CsrfSecret: s.CSRFSecret,
		ExpiresAt:  s.ExpiresAt,
	})
	if err != nil {
		return fmt.Errorf("creating session: %w", translate(err))
	}
	return nil
}

// GetAndTouchSession resolves a live session and refreshes its idle clock in one round
// trip. Absolute expiry, idle expiry and users.is_active are all enforced in SQL, so an
// expired or deactivated account cannot slip through a Go-side mistake.
func (r *Repo) GetAndTouchSession(ctx context.Context, tokenHash []byte, idleCutoff time.Time) (domain.Identity, error) {
	row, err := r.q.GetAndTouchSession(ctx, sqlcgen.GetAndTouchSessionParams{
		TokenHash:  tokenHash,
		LastSeenAt: idleCutoff,
	})
	if err != nil {
		return domain.Identity{}, fmt.Errorf("getting session: %w", translate(err))
	}
	role, ok := domain.ParseRole(string(row.Role))
	if !ok {
		return domain.Identity{}, fmt.Errorf("unknown role on session for user %d", row.UserID)
	}
	return domain.Identity{
		UserID:     row.UserID,
		Email:      row.Email,
		Role:       role,
		DriverID:   row.DriverID,
		Locale:     row.Locale,
		CSRFSecret: row.CsrfSecret,
		ExpiresAt:  row.ExpiresAt,
	}, nil
}

func (r *Repo) DeleteSession(ctx context.Context, tokenHash []byte) error {
	if err := r.q.DeleteSession(ctx, tokenHash); err != nil {
		return fmt.Errorf("deleting session: %w", translate(err))
	}
	return nil
}

func (r *Repo) DeleteSessionsForUser(ctx context.Context, userID int64) error {
	if err := r.q.DeleteSessionsForUser(ctx, userID); err != nil {
		return fmt.Errorf("deleting sessions for user: %w", translate(err))
	}
	return nil
}

func (r *Repo) DeleteExpiredSessions(ctx context.Context, idleCutoff time.Time) (int64, error) {
	n, err := r.q.DeleteExpiredSessions(ctx, idleCutoff)
	if err != nil {
		return 0, fmt.Errorf("deleting expired sessions: %w", translate(err))
	}
	return n, nil
}
