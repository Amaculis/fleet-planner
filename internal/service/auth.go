package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/buscompany/bus_fleet/internal/auth"
	"github.com/buscompany/bus_fleet/internal/domain"
)

// UserStore is the slice of the repository the auth service needs.
type UserStore interface {
	GetUserByEmail(ctx context.Context, email string) (domain.User, error)
}

// AuthService performs login and logout. Every failure path returns
// domain.ErrUnauthorized with the same shape, so nothing distinguishes "no such user"
// from "wrong password" from "account deactivated" to the caller.
type AuthService struct {
	users     UserStore
	sessions  *auth.Manager
	auditor   *Auditor
	log       *slog.Logger
	params    auth.Params
	dummyHash string // verified when no user matches, to equalise response time
}

func NewAuthService(users UserStore, sessions *auth.Manager, auditor *Auditor, log *slog.Logger, params auth.Params, dummyHash string) *AuthService {
	return &AuthService{users: users, sessions: sessions, auditor: auditor, log: log, params: params, dummyHash: dummyHash}
}

// Sessions exposes the session manager to the HTTP layer, which needs it to resolve the
// cookie on every request. Session *lifecycle* still happens here, in the service.
func (s *AuthService) Sessions() *auth.Manager { return s.sessions }

// NormalizeEmail matches the DB CHECK (users.email = lower(email)).
func NormalizeEmail(email string) string { return strings.ToLower(strings.TrimSpace(email)) }

// Login verifies credentials and issues a brand-new session token. Because the token is
// created only here and only after a successful verification, there is no pre-login
// identifier to fixate.
func (s *AuthService) Login(ctx context.Context, email, password string, meta Meta) (token string, expiresAt time.Time, identity domain.Identity, err error) {
	email = NormalizeEmail(email)

	// Cheap shape checks first; they cost no hashing time and are not user-distinguishable.
	if email == "" || len(email) > 254 || password == "" || len(password) > 1024 {
		return "", time.Time{}, domain.Identity{}, domain.ErrUnauthorized
	}

	user, lookupErr := s.users.GetUserByEmail(ctx, email)
	switch {
	case lookupErr == nil:
		// fall through to verification below
	case errors.Is(lookupErr, domain.ErrNotFound):
		// Spend the same CPU as a real verification, then fail.
		_, _ = auth.VerifyPassword(password, s.dummyHash)
		s.recordFailedLogin(ctx, nil, meta)
		return "", time.Time{}, domain.Identity{}, domain.ErrUnauthorized
	default:
		return "", time.Time{}, domain.Identity{}, fmt.Errorf("looking up user: %w", lookupErr)
	}

	ok, err := auth.VerifyPassword(password, user.PasswordHash)
	if err != nil {
		// A stored hash we cannot parse is an operational problem, not a client one.
		s.log.Error("password hash unusable", "user_id", user.ID, "error", err)
		return "", time.Time{}, domain.Identity{}, domain.ErrUnauthorized
	}
	if !ok || !user.IsActive {
		s.recordFailedLogin(ctx, &user.ID, meta)
		return "", time.Time{}, domain.Identity{}, domain.ErrUnauthorized
	}

	token, expiresAt, err = s.sessions.Create(ctx, user.ID)
	if err != nil {
		return "", time.Time{}, domain.Identity{}, fmt.Errorf("creating session: %w", err)
	}

	// Audited without any credential material; the actor is the user themselves.
	actor := user.ID
	if err := s.auditor.Record(ctx, &actor, "login", "user", &actor, nil,
		map[string]any{"role": string(user.Role)}, meta); err != nil {
		// The session exists; failing the login now would be worse than a missing
		// audit row, but this must be visible in the logs and alerted on.
		s.log.Error("audit write failed for login", "user_id", user.ID, "error", err)
	}

	identity = domain.Identity{
		UserID:    user.ID,
		Email:     user.Email,
		Role:      user.Role,
		DriverID:  user.DriverID,
		Locale:    user.Locale,
		ExpiresAt: expiresAt,
	}
	return token, expiresAt, identity, nil
}

// recordFailedLogin puts failed attempts in the audit log — the one place where an IP
// may be stored — instead of in the application log, which must stay free of personal
// data. userID is nil when no account matched; the attempted email is deliberately not
// recorded (data minimisation: it is often a real person's address, and the IP plus
// timestamp is what brute-force analysis actually needs).
func (s *AuthService) recordFailedLogin(ctx context.Context, userID *int64, meta Meta) {
	if err := s.auditor.Record(ctx, nil, "login_failed", "user", userID, nil,
		map[string]any{"reason": "invalid_credentials"}, meta); err != nil {
		s.log.Error("audit write failed for failed login", "error", err)
	}
}

// Logout invalidates the session server-side. It is idempotent: an unknown or already
// expired token is not an error.
func (s *AuthService) Logout(ctx context.Context, token string, identity domain.Identity, meta Meta) error {
	if err := s.sessions.Destroy(ctx, token); err != nil {
		return fmt.Errorf("destroying session: %w", err)
	}
	if identity.UserID != 0 {
		actor := identity.UserID
		if err := s.auditor.Record(ctx, &actor, "logout", "user", &actor, nil, nil, meta); err != nil {
			s.log.Error("audit write failed for logout", "user_id", identity.UserID, "error", err)
		}
	}
	return nil
}
